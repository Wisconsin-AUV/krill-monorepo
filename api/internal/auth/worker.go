package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/wauv/krill/api/internal/db"
)

// EnvWorkerToken names the token from KRILL_WORKER_TOKEN, which has no row.
const EnvWorkerToken = "KRILL_WORKER_TOKEN" //nolint:gosec // a variable name, not its value

// workerTokenPrefix makes a leaked token easy to recognize in a scan.
const workerTokenPrefix = "krill_wt_"

// WorkerTokenLookup returns the active saved token with a hash, or
// pgx.ErrNoRows when there is none or it was revoked.
type WorkerTokenLookup func(ctx context.Context, tokenHash []byte) (db.UseWorkerTokenRow, error)

// Worker is the token a worker request authenticated with.
type Worker struct {
	// TokenID is 0 for KRILL_WORKER_TOKEN.
	TokenID   int64
	TokenName string
}

type workerKey struct{}

// WorkerFrom returns the token of a worker procedure's caller.
func WorkerFrom(ctx context.Context) (Worker, bool) {
	w, ok := ctx.Value(workerKey{}).(Worker)
	return w, ok
}

// WithWorker runs a call as a worker, for tests that call services directly.
func WithWorker(ctx context.Context, w Worker) context.Context {
	return context.WithValue(ctx, workerKey{}, w)
}

// NewWorkerToken returns a random worker token and the hash to store.
func NewWorkerToken() (token string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("read worker token: %w", err)
	}
	token = workerTokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}
