package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

// userRef is a column that references users.
type userRef struct {
	table, column string
	// label names what the rows are in a merge preview. Rows without one
	// aren't counted.
	label string
	// key is the rest of a unique key that includes column. A source row
	// whose key the target already has is dropped instead of moved.
	key []string
	// drop deletes the source's rows instead of moving them.
	drop bool
}

// userRefs is every column that references users. A test fails when a
// migration adds one that isn't listed, or a unique key this doesn't know.
var userRefs = []userRef{
	{table: "annotations", column: "created_by", label: "Boxes drawn"},
	{table: "annotations", column: "updated_by", label: "Boxes last edited"},
	{table: "frames", column: "status_by", label: "Frames marked"},
	{table: "clip_claims", column: "user_id", label: "Clip claims"},
	{table: "gold_frames", column: "created_by", label: "Gold frames picked"},
	{table: "gold_attempts", column: "user_id", label: "Gold checks", key: []string{"frame_id"}},
	{table: "reviews", column: "created_by", label: "Reviews flagged"},
	{table: "reviews", column: "resolved_by", label: "Reviews resolved"},
	{table: "settings", column: "updated_by", label: "Settings changed"},
	{table: "audit_log", column: "actor", label: "Audit log entries"},
	{table: "invites", column: "created_by", label: "Invites created"},
	{table: "invite_uses", column: "user_id", label: "Invites used", key: []string{"invite_id"}},
	{table: "approved_emails", column: "created_by", label: "Emails approved"},
	{table: "approved_emails", column: "used_by", label: "Pre-approved sign-ups"},
	{table: "box_events", column: "user_id", label: "Box decisions"},
	{table: "box_events", column: "labeler_id", label: "Corrections received"},
	{table: "worker_tokens", column: "created_by", label: "Worker tokens created"},
	// Whoever is signed in as the source signs in again as the target, so a
	// mistaken merge doesn't hand them the target's account.
	{table: "sessions", column: "user_id", drop: true},
	// Codes are hashed with the user's ID, so they can't move.
	{table: "email_codes", column: "user_id", drop: true},
}

func (r userRef) ident() (table, column string) {
	return pgx.Identifier{r.table}.Sanitize(), pgx.Identifier{r.column}.Sanitize()
}

// conflict matches source rows, as s, whose key the target already has.
func (r userRef) conflict() string {
	table, column := r.ident()
	match := make([]string, len(r.key))
	for i, k := range r.key {
		k := pgx.Identifier{k}.Sanitize()
		match[i] = fmt.Sprintf("x.%s = s.%s", k, k)
	}
	return fmt.Sprintf("s.%s = $1 AND EXISTS (SELECT 1 FROM %s x WHERE x.%s = $2 AND %s)",
		column, table, column, strings.Join(match, " AND "))
}

type mergeCount struct {
	label          string
	moved, dropped int64
}

// countRefs says what moving source's rows to target would do.
func countRefs(ctx context.Context, conn db.DBTX, source, target uuid.UUID) ([]mergeCount, error) {
	var out []mergeCount
	for _, r := range userRefs {
		if r.label == "" {
			continue
		}
		table, column := r.ident()
		var total, dropped int64
		if err := conn.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s WHERE %s = $1", table, column), source).Scan(&total); err != nil {
			return nil, fmt.Errorf("count %s.%s: %w", r.table, r.column, err)
		}
		if len(r.key) > 0 && total > 0 {
			q := fmt.Sprintf("SELECT count(*) FROM %s s WHERE %s", table, r.conflict())
			if err := conn.QueryRow(ctx, q, source, target).Scan(&dropped); err != nil {
				return nil, fmt.Errorf("count %s conflicts: %w", r.table, err)
			}
		}
		out = append(out, mergeCount{label: r.label, moved: total - dropped, dropped: dropped})
	}
	return out, nil
}

// moveRefs points every reference to source at target.
func moveRefs(ctx context.Context, tx pgx.Tx, source, target uuid.UUID) error {
	for _, r := range userRefs {
		table, column := r.ident()
		var err error
		switch {
		case r.drop:
			_, err = tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE %s = $1", table, column), source)
		case len(r.key) > 0:
			if _, err = tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s s WHERE %s", table, r.conflict()), source, target); err != nil {
				break
			}
			fallthrough
		default:
			_, err = tx.Exec(ctx, fmt.Sprintf("UPDATE %s SET %s = $2 WHERE %s = $1", table, column, column), source, target)
		}
		if err != nil {
			return fmt.Errorf("move %s.%s: %w", r.table, r.column, err)
		}
	}
	return nil
}

// mergedIdentity is the target after taking what it lacks from the source:
// the higher role, a Slack link, a password, and a verified email.
func mergedIdentity(source, target db.User) db.SetMergedIdentityParams {
	p := db.SetMergedIdentityParams{
		ID:              target.ID,
		Role:            target.Role,
		SlackUserID:     target.SlackUserID,
		PasswordHash:    target.PasswordHash,
		Email:           target.Email,
		EmailVerifiedAt: target.EmailVerifiedAt,
		CreatedAt:       target.CreatedAt,
		LastLoginAt:     target.LastLoginAt,
	}
	if auth.ParseRole(source.Role) > auth.ParseRole(target.Role) {
		p.Role = source.Role
	}
	if !p.SlackUserID.Valid {
		p.SlackUserID = source.SlackUserID
	}
	if !p.PasswordHash.Valid {
		p.PasswordHash = source.PasswordHash
	}
	if !p.EmailVerifiedAt.Valid && source.EmailVerifiedAt.Valid {
		p.Email = source.Email
		p.EmailVerifiedAt = source.EmailVerifiedAt
	}
	if source.CreatedAt.Time.Before(p.CreatedAt.Time) {
		p.CreatedAt = source.CreatedAt
	}
	if source.LastLoginAt.Valid && (!p.LastLoginAt.Valid || source.LastLoginAt.Time.After(p.LastLoginAt.Time)) {
		p.LastLoginAt = source.LastLoginAt
	}
	return p
}

func applyIdentity(u db.User, p db.SetMergedIdentityParams) db.User {
	u.Role = p.Role
	u.SlackUserID = p.SlackUserID
	u.PasswordHash = p.PasswordHash
	u.Email = p.Email
	u.EmailVerifiedAt = p.EmailVerifiedAt
	if u.PendingEmail.Valid && u.PendingEmail.String == p.Email {
		u.PendingEmail = pgtype.Text{}
	}
	u.CreatedAt = p.CreatedAt
	u.LastLoginAt = p.LastLoginAt
	return u
}

// mergePair parses the two accounts and rejects merging an account into
// itself, or the caller merging their own account away.
func mergePair(ctx context.Context, sourceID, targetID string) (source, target uuid.UUID, err error) {
	if source, err = uuid.Parse(sourceID); err != nil {
		return uuid.Nil, uuid.Nil, rpc.Invalid("invalid source user id")
	}
	if target, err = uuid.Parse(targetID); err != nil {
		return uuid.Nil, uuid.Nil, rpc.Invalid("invalid target user id")
	}
	if source == target {
		return uuid.Nil, uuid.Nil, rpc.Invalid("pick two different accounts")
	}
	if source == auth.CallerID(ctx) {
		return uuid.Nil, uuid.Nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("you can't merge your own account away; merge the other account into yours"))
	}
	return source, target, nil
}

func countsProto(counts []mergeCount) []*krillv1.MergeCount {
	var out []*krillv1.MergeCount
	for _, c := range counts {
		if c.moved == 0 && c.dropped == 0 {
			continue
		}
		out = append(out, &krillv1.MergeCount{Label: c.label, Moved: c.moved, Dropped: c.dropped})
	}
	return out
}

func (s *Service) PreviewMergeUsers(ctx context.Context, req *krillv1.PreviewMergeUsersRequest) (*krillv1.PreviewMergeUsersResponse, error) {
	sourceID, targetID, err := mergePair(ctx, req.GetSourceId(), req.GetTargetId())
	if err != nil {
		return nil, err
	}
	source, err := s.q.GetUser(ctx, sourceID)
	if err != nil {
		return nil, rpc.DBError(err, "source user")
	}
	target, err := s.q.GetUser(ctx, targetID)
	if err != nil {
		return nil, rpc.DBError(err, "target user")
	}
	counts, err := countRefs(ctx, s.pool, sourceID, targetID)
	if err != nil {
		return nil, rpc.Internal(err, "count records")
	}
	return &krillv1.PreviewMergeUsersResponse{
		Source: auth.ToProto(source),
		Target: auth.ToProto(target),
		Result: auth.ToProto(applyIdentity(target, mergedIdentity(source, target))),
		Counts: countsProto(counts),
	}, nil
}

func (s *Service) MergeUsers(ctx context.Context, req *krillv1.MergeUsersRequest) (*krillv1.MergeUsersResponse, error) {
	sourceID, targetID, err := mergePair(ctx, req.GetSourceId(), req.GetTargetId())
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	// Locked so neither account changes, and nothing new references the
	// source, until the merge commits.
	pair, err := q.LockUserPair(ctx, db.LockUserPairParams{SourceID: sourceID, TargetID: targetID})
	if err != nil {
		return nil, rpc.Internal(err, "lock users")
	}
	var source, target db.User
	for _, u := range pair {
		if u.ID == sourceID {
			source = u
		} else {
			target = u
		}
	}
	if source.ID == uuid.Nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("source user not found"))
	}
	if target.ID == uuid.Nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("target user not found"))
	}

	counts, err := countRefs(ctx, tx, sourceID, targetID)
	if err != nil {
		return nil, rpc.Internal(err, "count records")
	}
	if err := moveRefs(ctx, tx, sourceID, targetID); err != nil {
		return nil, rpc.Internal(err, "move records")
	}
	// Deleted before the target takes its Slack link and email, which are
	// unique.
	if _, err := q.DeleteUser(ctx, sourceID); err != nil {
		return nil, rpc.Internal(err, "delete source user")
	}
	merged, err := q.SetMergedIdentity(ctx, mergedIdentity(source, target))
	if err != nil {
		return nil, rpc.Internal(err, "merge identity")
	}

	moved := map[string]int64{}
	dropped := map[string]int64{}
	for _, c := range counts {
		if c.moved > 0 {
			moved[c.label] = c.moved
		}
		if c.dropped > 0 {
			dropped[c.label] = c.dropped
		}
	}
	details := map[string]any{
		"source": map[string]any{
			"id": source.ID.String(), "name": source.Name, "username": source.Username, "email": source.Email,
		},
		"moved": moved,
	}
	if len(dropped) > 0 {
		details["kept_target"] = dropped
	}
	if merged.Role != target.Role {
		details["role"] = audit.Change(target.Role, merged.Role)
	}
	if err := record(ctx, q, audit.UserMerge, merged, details); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.MergeUsersResponse{User: auth.ToProto(merged), Counts: countsProto(counts)}, nil
}
