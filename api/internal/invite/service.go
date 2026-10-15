// Package invite manages invite links and pre-approved emails. Signing up
// with them is in package auth.
package invite

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/email"
	"github.com/wauv/krill/api/internal/rpc"
)

const (
	maxUses         = 100
	maxExpiry       = 90 * 24 * time.Hour
	maxNoteLen      = 200
	maxEmailsAtOnce = 200
)

type Options struct {
	// Mailer is nil when the API can't send email.
	Mailer    email.Sender
	PublicURL string
	TeamName  string
}

type Service struct {
	krillv1connect.UnimplementedInviteServiceHandler
	pool *pgxpool.Pool
	q    *db.Queries
	opts func() Options
}

func NewService(pool *pgxpool.Pool, opts func() Options) *Service {
	return &Service{pool: pool, q: db.New(pool), opts: opts}
}

func state(i db.Invite, now time.Time) krillv1.InviteState {
	switch {
	case i.RevokedAt.Valid:
		return krillv1.InviteState_INVITE_STATE_REVOKED
	case i.Uses >= i.MaxUses:
		return krillv1.InviteState_INVITE_STATE_USED_UP
	case !i.ExpiresAt.Time.After(now):
		return krillv1.InviteState_INVITE_STATE_EXPIRED
	}
	return krillv1.InviteState_INVITE_STATE_ACTIVE
}

// profile is a user joined with LEFT JOIN, unset when the user is gone.
func profile(id pgtype.UUID, name, username, role pgtype.Text, createdAt pgtype.Timestamptz) *krillv1.Profile {
	if !id.Valid || !name.Valid {
		return nil
	}
	return auth.PublicProfile(id.Bytes, name.String, username.String, role.String, createdAt)
}

func timestamp(t pgtype.Timestamptz) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}

func toProto(i db.Invite, creator *krillv1.Profile, uses []*krillv1.InviteUse) *krillv1.Invite {
	return &krillv1.Invite{
		Id:        i.ID,
		Role:      auth.ParseRole(i.Role),
		MaxUses:   i.MaxUses,
		Uses:      i.Uses,
		Note:      i.Note,
		Email:     i.Email,
		ExpiresAt: timestamp(i.ExpiresAt),
		RevokedAt: timestamp(i.RevokedAt),
		CreatedAt: timestamp(i.CreatedAt),
		CreatedBy: creator,
		UsedBy:    uses,
		State:     state(i, time.Now()),
	}
}

func listUses(ctx context.Context, q *db.Queries, ids []int64) (map[int64][]*krillv1.InviteUse, error) {
	rows, err := q.ListInviteUses(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := map[int64][]*krillv1.InviteUse{}
	for _, r := range rows {
		out[r.InviteID] = append(out[r.InviteID], &krillv1.InviteUse{
			User:   auth.PublicProfile(r.ID, r.Name, r.Username, r.Role, r.CreatedAt),
			UsedAt: timestamp(r.UsedAt),
		})
	}
	return out, nil
}

func (s *Service) ListInvites(ctx context.Context, _ *krillv1.ListInvitesRequest) (*krillv1.ListInvitesResponse, error) {
	rows, err := s.q.ListInvites(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list invites")
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.Invite.ID
	}
	uses, err := listUses(ctx, s.q, ids)
	if err != nil {
		return nil, rpc.Internal(err, "list invite uses")
	}
	out := make([]*krillv1.Invite, len(rows))
	for i, r := range rows {
		creator := profile(r.Invite.CreatedBy, r.CreatorName, r.CreatorUsername, r.CreatorRole, r.CreatorCreatedAt)
		out[i] = toProto(r.Invite, creator, uses[r.Invite.ID])
	}
	return &krillv1.ListInvitesResponse{Invites: out}, nil
}

func roleName(r krillv1.Role) (string, error) {
	if r == krillv1.Role_ROLE_UNSPECIFIED {
		return "labeler", nil
	}
	name, ok := auth.RoleName(r)
	if !ok {
		return "", rpc.Invalid("unknown role")
	}
	return name, nil
}

func roleLabel(name string) string {
	for _, r := range auth.Roles {
		if r.Name == name {
			return strings.ToLower(r.Label)
		}
	}
	return name
}

func (s *Service) CreateInvite(ctx context.Context, req *krillv1.CreateInviteRequest) (*krillv1.CreateInviteResponse, error) {
	role, err := roleName(req.GetRole())
	if err != nil {
		return nil, err
	}
	if req.GetMaxUses() < 1 || req.GetMaxUses() > maxUses {
		return nil, rpc.Invalid("uses must be from 1 to %d", maxUses)
	}
	expiry := time.Duration(req.GetExpiresInHours()) * time.Hour
	if expiry < time.Hour || expiry > maxExpiry {
		return nil, rpc.Invalid("expiry must be from 1 hour to 90 days")
	}
	note := strings.TrimSpace(req.GetNote())
	if utf8.RuneCountInString(note) > maxNoteLen {
		return nil, rpc.Invalid("note must be at most %d characters", maxNoteLen)
	}
	opts := s.opts()
	var to string
	if req.GetEmail() != "" {
		if to, err = auth.NormalizeEmail(req.GetEmail()); err != nil {
			return nil, rpc.Invalid("%s", err)
		}
		if opts.Mailer == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("this server can't send email"))
		}
	}
	token, hash, err := auth.NewInviteToken()
	if err != nil {
		return nil, rpc.Internal(err, "create token")
	}
	url := strings.TrimRight(opts.PublicURL, "/") + "/register?invite=" + token
	expires := time.Now().Add(expiry)

	sess, _ := auth.SessionFrom(ctx)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	inv, err := q.CreateInvite(ctx, db.CreateInviteParams{
		TokenHash: hash,
		Role:      role,
		MaxUses:   req.GetMaxUses(),
		Note:      note,
		Email:     to,
		ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true},
		CreatedBy: pgtype.UUID{Bytes: sess.User.ID, Valid: sess.User.ID != uuid.Nil},
	})
	if err != nil {
		return nil, rpc.Internal(err, "create invite")
	}
	details := map[string]any{"role": role, "max_uses": inv.MaxUses, "expires_at": expires.UTC().Format(time.RFC3339)}
	if to != "" {
		details["email"] = to
	}
	if err := record(ctx, q, audit.InviteCreate, inv, details); err != nil {
		return nil, err
	}
	// Sent before commit so a failed email leaves no invite behind.
	if to != "" {
		msg, err := email.InviteEmail(to, email.Invite{
			Inviter: sess.User.Name,
			Role:    roleLabel(role),
			URL:     url,
			Expires: "on " + expires.UTC().Format("2 January 2006"),
			Team:    opts.TeamName,
			Year:    time.Now().Year(),
		})
		if err != nil {
			return nil, rpc.Internal(err, "render email")
		}
		if err := opts.Mailer.Send(ctx, msg); err != nil {
			slog.Error("send invite", "err", err)
			return nil, connect.NewError(connect.CodeAborted, errors.New("could not send the email; try again"))
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	creator := auth.PublicProfile(sess.User.ID, sess.User.Name, sess.User.Username, sess.User.Role, sess.User.CreatedAt)
	return &krillv1.CreateInviteResponse{Invite: toProto(inv, creator, nil), Url: url}, nil
}

func inviteName(i db.Invite) string {
	if i.Note != "" {
		return i.Note
	}
	if i.Email != "" {
		return i.Email
	}
	return "Invite " + strconv.FormatInt(i.ID, 10)
}

func record(ctx context.Context, q *db.Queries, action string, i db.Invite, details any) error {
	err := audit.Record(ctx, q, audit.Entry{
		Action:     action,
		TargetType: audit.TargetInvite,
		TargetID:   strconv.FormatInt(i.ID, 10),
		TargetName: inviteName(i),
		Details:    details,
	})
	if err != nil {
		return rpc.Internal(err, "record audit entry")
	}
	return nil
}

func (s *Service) RevokeInvite(ctx context.Context, req *krillv1.RevokeInviteRequest) (*krillv1.RevokeInviteResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	before, err := q.GetInvite(ctx, req.GetId())
	if err != nil {
		return nil, rpc.DBError(err, "invite")
	}
	inv, err := q.RevokeInvite(ctx, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "revoke invite")
	}
	if !before.Invite.RevokedAt.Valid {
		if err := record(ctx, q, audit.InviteRevoke, inv, map[string]any{"uses": inv.Uses}); err != nil {
			return nil, err
		}
	}
	uses, err := listUses(ctx, q, []int64{inv.ID})
	if err != nil {
		return nil, rpc.Internal(err, "list invite uses")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	creator := profile(before.Invite.CreatedBy, before.CreatorName, before.CreatorUsername, before.CreatorRole, before.CreatorCreatedAt)
	return &krillv1.RevokeInviteResponse{Invite: toProto(inv, creator, uses[inv.ID])}, nil
}

func (s *Service) CheckInvite(ctx context.Context, req *krillv1.CheckInviteRequest) (*krillv1.CheckInviteResponse, error) {
	token := strings.TrimSpace(req.GetToken())
	if token == "" {
		return &krillv1.CheckInviteResponse{}, nil
	}
	inv, err := s.q.GetInviteByToken(ctx, auth.HashInviteToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return &krillv1.CheckInviteResponse{}, nil
	}
	if err != nil {
		return nil, rpc.Internal(err, "load invite")
	}
	if state(inv, time.Now()) != krillv1.InviteState_INVITE_STATE_ACTIVE {
		return &krillv1.CheckInviteResponse{}, nil
	}
	return &krillv1.CheckInviteResponse{Valid: true, Role: auth.ParseRole(inv.Role), Email: inv.Email}, nil
}

func approvedProto(r db.ListApprovedEmailsRow) *krillv1.ApprovedEmail {
	a := r.ApprovedEmail
	return &krillv1.ApprovedEmail{
		Email:     a.Email,
		Role:      auth.ParseRole(a.Role),
		CreatedBy: profile(a.CreatedBy, r.CreatorName, r.CreatorUsername, r.CreatorRole, r.CreatorCreatedAt),
		CreatedAt: timestamp(a.CreatedAt),
		UsedBy:    profile(a.UsedBy, r.UserName, r.UserUsername, r.UserRole, r.UserCreatedAt),
		UsedAt:    timestamp(a.UsedAt),
	}
}

func (s *Service) ListApprovedEmails(ctx context.Context, _ *krillv1.ListApprovedEmailsRequest) (*krillv1.ListApprovedEmailsResponse, error) {
	rows, err := s.q.ListApprovedEmails(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list approved emails")
	}
	out := make([]*krillv1.ApprovedEmail, len(rows))
	for i, r := range rows {
		out[i] = approvedProto(r)
	}
	return &krillv1.ListApprovedEmailsResponse{ApprovedEmails: out}, nil
}

func (s *Service) AddApprovedEmails(ctx context.Context, req *krillv1.AddApprovedEmailsRequest) (*krillv1.AddApprovedEmailsResponse, error) {
	role, err := roleName(req.GetRole())
	if err != nil {
		return nil, err
	}
	if len(req.GetEmails()) == 0 {
		return nil, rpc.Invalid("add at least one email")
	}
	if len(req.GetEmails()) > maxEmailsAtOnce {
		return nil, rpc.Invalid("add at most %d emails at once", maxEmailsAtOnce)
	}
	seen := map[string]bool{}
	var emails []string
	for _, raw := range req.GetEmails() {
		e, err := auth.NormalizeEmail(raw)
		if err != nil {
			return nil, rpc.Invalid("%s: %s", strings.TrimSpace(raw), err)
		}
		if !seen[e] {
			seen[e] = true
			emails = append(emails, e)
		}
	}

	sess, _ := auth.SessionFrom(ctx)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	for _, e := range emails {
		_, err := q.AddApprovedEmail(ctx, db.AddApprovedEmailParams{
			Email:     e,
			Role:      role,
			CreatedBy: pgtype.UUID{Bytes: sess.User.ID, Valid: sess.User.ID != uuid.Nil},
		})
		if err != nil {
			return nil, rpc.Internal(err, "approve email")
		}
		err = audit.Record(ctx, q, audit.Entry{
			Action:     audit.ApprovedEmailAdd,
			TargetType: audit.TargetApprovedEmail,
			TargetID:   e,
			TargetName: e,
			Details:    map[string]any{"role": role},
		})
		if err != nil {
			return nil, rpc.Internal(err, "record audit entry")
		}
	}
	rows, err := q.ListApprovedEmails(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list approved emails")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	var out []*krillv1.ApprovedEmail
	for _, r := range rows {
		if seen[r.ApprovedEmail.Email] {
			out = append(out, approvedProto(r))
		}
	}
	return &krillv1.AddApprovedEmailsResponse{ApprovedEmails: out}, nil
}

func (s *Service) RemoveApprovedEmail(ctx context.Context, req *krillv1.RemoveApprovedEmailRequest) (*krillv1.RemoveApprovedEmailResponse, error) {
	e := strings.ToLower(strings.TrimSpace(req.GetEmail()))
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	removed, err := q.RemoveApprovedEmail(ctx, e)
	if err != nil {
		return nil, rpc.DBError(err, "approved email")
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.ApprovedEmailRemove,
		TargetType: audit.TargetApprovedEmail,
		TargetID:   removed.Email,
		TargetName: removed.Email,
		Details:    map[string]any{"used": removed.UsedAt.Valid},
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.RemoveApprovedEmailResponse{}, nil
}
