package admin

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/compute"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/ingest"
	"github.com/wauv/krill/api/internal/jobmeta"
	"github.com/wauv/krill/api/internal/settings"
	"github.com/wauv/krill/api/internal/tracker"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("KRILL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KRILL_TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func testAdmin(t *testing.T, pool *pgxpool.Pool) db.User {
	t.Helper()
	name := "admin-" + uuid.NewString()[:8]
	var u db.User
	err := pool.QueryRow(t.Context(), `
		INSERT INTO users (name, username, email, role) VALUES ($1, $1, $1 || '@example.com', 'admin')
		RETURNING id, name, username, role, created_at`, name,
	).Scan(&u.ID, &u.Name, &u.Username, &u.Role, &u.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u.ID) })
	return u
}

type workerStub struct {
	krillv1connect.UnimplementedWorkerServiceHandler
}

func (workerStub) NextTask(context.Context, *krillv1.NextTaskRequest) (*krillv1.NextTaskResponse, error) {
	return &krillv1.NextTaskResponse{}, nil
}

func header(key, value string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(key, value)
			return next(ctx, req)
		}
	}))
}

// Rotating a token only helps if the old one stops working the moment it is
// revoked, and a worker token must never open an admin procedure.
func TestRevokedWorkerTokenIsRejected(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	admin := testAdmin(t, pool)
	session := "session-" + uuid.NewString()
	hash := sha256.Sum256([]byte(session))
	err := db.New(pool).CreateSession(ctx, db.CreateSessionParams{
		TokenHash: hash[:], UserID: admin.ID, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := settings.Open(ctx, pool, func(key string) (string, bool) {
		if key == "KRILL_DATABASE_URL" {
			return "postgres://test", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	q := db.New(pool)
	opts := connect.WithInterceptors(auth.NewInterceptor(auth.LookupSessions(q), func() string { return "" }).AcceptWorkerTokens(q.UseWorkerToken))
	mux := http.NewServeMux()
	mux.Handle(krillv1connect.NewWorkerServiceHandler(workerStub{}, opts))
	mux.Handle(krillv1connect.NewComputeServiceHandler(NewComputeService(ComputeDeps{
		Pool: pool, Settings: store, Nodes: compute.NewRegistry(), Dispatcher: tracker.NewDispatcher(),
	}), opts))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	admins := krillv1connect.NewComputeServiceClient(srv.Client(), srv.URL, header("Cookie", auth.CookieName+"="+session))
	created, err := admins.CreateWorkerToken(ctx, &krillv1.CreateWorkerTokenRequest{Name: "rig " + uuid.NewString()[:8]})
	if err != nil {
		t.Fatal(err)
	}
	id := created.GetToken().GetId()
	secret := created.GetSecret()
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM worker_tokens WHERE id = $1`, id) })

	var stored []byte
	if err := pool.QueryRow(ctx, `SELECT token_hash FROM worker_tokens WHERE id = $1`, id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), secret) {
		t.Error("the token is stored in plain text")
	}

	worker := krillv1connect.NewWorkerServiceClient(srv.Client(), srv.URL, header("Authorization", "Bearer "+secret))
	if _, err := worker.NextTask(ctx, &krillv1.NextTaskRequest{}); err != nil {
		t.Fatalf("NextTask with a new token: %v", err)
	}
	asWorker := krillv1connect.NewComputeServiceClient(srv.Client(), srv.URL, header("Authorization", "Bearer "+secret))
	if _, err := asWorker.ListWorkerTokens(ctx, &krillv1.ListWorkerTokensRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("ListWorkerTokens with a worker token: err = %v, want unauthenticated", err)
	}
	asSession := krillv1connect.NewWorkerServiceClient(srv.Client(), srv.URL, header("Authorization", "Bearer "+session))
	if _, err := asSession.NextTask(ctx, &krillv1.NextTaskRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("NextTask with a session token: err = %v, want unauthenticated", err)
	}

	if _, err := admins.RevokeWorkerToken(ctx, &krillv1.RevokeWorkerTokenRequest{Id: id}); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.NextTask(ctx, &krillv1.NextTaskRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("NextTask with a revoked token: err = %v, want unauthenticated", err)
	}
	if _, err := admins.RevokeWorkerToken(ctx, &krillv1.RevokeWorkerTokenRequest{Id: id}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("revoking twice: err = %v, want failed precondition", err)
	}

	var entries int
	var leaked bool
	err = pool.QueryRow(ctx, `
		SELECT count(*), bool_or(details::text LIKE '%' || $3 || '%' OR target_name LIKE '%' || $3 || '%')
		FROM audit_log WHERE actor = $1 AND target_id = $2::bigint::text`, admin.ID, created.GetToken().GetId(), secret,
	).Scan(&entries, &leaked)
	if err != nil {
		t.Fatal(err)
	}
	if entries != 2 || leaked {
		t.Errorf("%d audit entries, token in them: %v; want 2 without it", entries, leaked)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM audit_log WHERE actor = $1`, admin.ID) })
}

// Running an ingest deletes the video's clips and every label on them, so a
// retry must never run one that finished or was started again since.
func TestJobRetryAndCancelKeepIngestsConsistent(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatal(err)
	}
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Middleware: []rivertype.Middleware{&jobmeta.StartedBy{}}})
	if err != nil {
		t.Fatal(err)
	}
	admin := testAdmin(t, pool)
	ctx = auth.WithUser(ctx, admin)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM audit_log WHERE actor = $1`, admin.ID) })
	s := NewComputeService(ComputeDeps{Pool: pool, Jobs: jobs, Nodes: compute.NewRegistry(), Dispatcher: tracker.NewDispatcher()})

	var videoID int64
	if err := pool.QueryRow(ctx, `INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', 'queued') RETURNING id`).Scan(&videoID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	inserted, err := jobs.Insert(ctx, ingest.Args{VideoID: videoID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobID := inserted.Job.ID
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM river_job WHERE id = $1`, jobID) })
	status := func() string {
		t.Helper()
		var s string
		if err := pool.QueryRow(ctx, `SELECT status FROM videos WHERE id = $1`, videoID).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	list, err := s.ListJobs(ctx, &krillv1.ListJobsRequest{
		Kinds:     []string{"ingest"},
		States:    []krillv1.JobState{krillv1.JobState_JOB_STATE_QUEUED},
		PageSize:  1,
		PageToken: strconv.FormatInt(jobID+1, 10),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.GetJobs()) != 1 || list.GetJobs()[0].GetId() != jobID || list.GetJobs()[0].GetTarget() != "v" {
		t.Errorf("listed %v, want job %d on video v", list.GetJobs(), jobID)
	}

	if _, err := s.RetryJob(ctx, &krillv1.RetryJobRequest{Id: jobID}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("retry a queued job: err = %v, want failed precondition", err)
	}
	res, err := s.CancelJob(ctx, &krillv1.CancelJobRequest{Id: jobID})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetJob().GetState() != krillv1.JobState_JOB_STATE_CANCELLED || status() != "failed" {
		t.Errorf("after cancel: job %v, video %s; want cancelled and failed", res.GetJob().GetState(), status())
	}
	if got := res.GetJob().GetStartedBy().GetId(); got != admin.ID.String() {
		t.Errorf("started by %q, want %s", got, admin.ID)
	}

	// The video page's own retry queued a new ingest in the meantime.
	if _, err := pool.Exec(ctx, `UPDATE videos SET status = 'queued' WHERE id = $1`, videoID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RetryJob(ctx, &krillv1.RetryJobRequest{Id: jobID}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("retry after the video was queued again: err = %v, want failed precondition", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE videos SET status = 'failed' WHERE id = $1`, videoID); err != nil {
		t.Fatal(err)
	}
	retried, err := s.RetryJob(ctx, &krillv1.RetryJobRequest{Id: jobID})
	if err != nil {
		t.Fatal(err)
	}
	if retried.GetJob().GetState() != krillv1.JobState_JOB_STATE_QUEUED || status() != "queued" {
		t.Errorf("after retry: job %v, video %s; want queued", retried.GetJob().GetState(), status())
	}

	if _, err := pool.Exec(ctx, `UPDATE river_job SET state = 'completed', finalized_at = now() WHERE id = $1`, jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE videos SET status = 'ready' WHERE id = $1`, videoID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RetryJob(ctx, &krillv1.RetryJobRequest{Id: jobID}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("retry a completed ingest: err = %v, want failed precondition", err)
	}
	if _, err := s.SetJobPriority(ctx, &krillv1.SetJobPriorityRequest{Id: jobID, Priority: 2}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("reprioritize a completed job: err = %v, want failed precondition", err)
	}
	if got := status(); got != "ready" {
		t.Errorf("video status = %s, want ready", got)
	}
}
