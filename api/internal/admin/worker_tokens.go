package admin

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

const maxTokenNameLen = 64

// tokenWorkers counts online workers by the name of their token.
func (s *ComputeService) tokenWorkers() map[string]int32 {
	out := map[string]int32{}
	for _, w := range s.nodes.Workers() {
		if w.Online {
			out[w.TokenName]++
		}
	}
	return out
}

func (s *ComputeService) ListWorkerTokens(ctx context.Context, _ *krillv1.ListWorkerTokensRequest) (*krillv1.ListWorkerTokensResponse, error) {
	rows, err := s.q.ListWorkerTokens(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list worker tokens")
	}
	counts := s.tokenWorkers()
	out := &krillv1.ListWorkerTokensResponse{
		Tokens:          make([]*krillv1.WorkerToken, len(rows)),
		EnvToken:        s.settings.Config().WorkerToken != "",
		EnvTokenWorkers: counts[auth.EnvWorkerToken],
	}
	for i, r := range rows {
		t := &krillv1.WorkerToken{
			Id:         r.ID,
			Name:       r.Name,
			CreatedAt:  ts(r.CreatedAt.Time),
			LastUsedAt: tsValid(r.LastUsedAt),
			RevokedAt:  tsValid(r.RevokedAt),
		}
		if r.CreatorID.Valid {
			t.CreatedBy = auth.PublicProfile(r.CreatorID.Bytes, r.CreatorName.String, r.CreatorUsername.String, r.CreatorRole.String, r.CreatorCreatedAt)
		}
		// Names are only unique among active tokens.
		if !r.RevokedAt.Valid {
			t.Workers = counts[r.Name]
		}
		out.Tokens[i] = t
	}
	return out, nil
}

func tsValid(t pgtype.Timestamptz) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return ts(t.Time)
}

func tokenProto(t db.WorkerToken) *krillv1.WorkerToken {
	return &krillv1.WorkerToken{
		Id:         t.ID,
		Name:       t.Name,
		CreatedAt:  ts(t.CreatedAt.Time),
		LastUsedAt: tsValid(t.LastUsedAt),
		RevokedAt:  tsValid(t.RevokedAt),
	}
}

func (s *ComputeService) CreateWorkerToken(ctx context.Context, req *krillv1.CreateWorkerTokenRequest) (*krillv1.CreateWorkerTokenResponse, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" || len(name) > maxTokenNameLen {
		return nil, rpc.Invalid("name must be 1 to %d characters", maxTokenNameLen)
	}
	if strings.EqualFold(name, auth.EnvWorkerToken) {
		return nil, rpc.Invalid("%s names the token in the environment", auth.EnvWorkerToken)
	}
	secret, hash, err := auth.NewWorkerToken()
	if err != nil {
		return nil, rpc.Internal(err, "create worker token")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	caller := auth.CallerID(ctx)
	t, err := q.CreateWorkerToken(ctx, db.CreateWorkerTokenParams{
		Name:      name,
		TokenHash: hash,
		CreatedBy: pgtype.UUID{Bytes: caller, Valid: true},
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("a token with that name exists"))
	}
	if err != nil {
		return nil, rpc.Internal(err, "save worker token")
	}
	if err := recordToken(ctx, q, audit.WorkerTokenCreate, t); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	out := tokenProto(t)
	if sess, ok := auth.SessionFrom(ctx); ok {
		u := sess.User
		out.CreatedBy = auth.PublicProfile(u.ID, u.Name, u.Username, u.Role, u.CreatedAt)
	}
	return &krillv1.CreateWorkerTokenResponse{Token: out, Secret: secret}, nil
}

func (s *ComputeService) RevokeWorkerToken(ctx context.Context, req *krillv1.RevokeWorkerTokenRequest) (*krillv1.RevokeWorkerTokenResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	t, err := q.RevokeWorkerToken(ctx, req.GetId())
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := q.GetWorkerToken(ctx, req.GetId()); err != nil {
			return nil, rpc.DBError(err, "worker token")
		}
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the token is already revoked"))
	}
	if err != nil {
		return nil, rpc.Internal(err, "revoke worker token")
	}
	if err := recordToken(ctx, q, audit.WorkerTokenRevoke, t); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.RevokeWorkerTokenResponse{Token: tokenProto(t)}, nil
}

func recordToken(ctx context.Context, q *db.Queries, action string, t db.WorkerToken) error {
	err := audit.Record(ctx, q, audit.Entry{
		Action:     action,
		TargetType: audit.TargetWorkerToken,
		TargetID:   strconv.FormatInt(t.ID, 10),
		TargetName: t.Name,
	})
	if err != nil {
		return rpc.Internal(err, "record audit entry")
	}
	return nil
}
