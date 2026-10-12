package review

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

func TestEnqueueIsIdempotent(t *testing.T) {
	url := os.Getenv("KRILL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KRILL_TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID uuid.UUID
	var frameID int64
	err = tx.QueryRow(ctx, `
		WITH u AS (
			INSERT INTO users (name, username, email) VALUES ('Ada', 'review-test', 'review-test@example.com') RETURNING id
		), v AS (
			INSERT INTO videos (name, filename) VALUES ('v', 'v.mp4') RETURNING id
		), c AS (
			INSERT INTO clips (video_id, idx, start_frame, frame_count) SELECT id, 0, 0, 1 FROM v RETURNING id, video_id
		), f AS (
			INSERT INTO frames (video_id, clip_id, idx, phash) SELECT video_id, id, 0, 0 FROM c RETURNING id
		)
		SELECT u.id, f.id FROM u, f`).Scan(&userID, &frameID)
	if err != nil {
		t.Fatal(err)
	}
	q := db.New(tx)

	first, err := Enqueue(ctx, q, frameID, ReasonFlagged, userID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Enqueue(ctx, q, frameID, ReasonFlagged, userID)
	if err != nil {
		t.Fatal(err)
	}
	other, err := Enqueue(ctx, q, frameID, ReasonDrift, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if again.GetId() != first.GetId() || other.GetId() != first.GetId() {
		t.Fatalf("ids %d, %d, %d: want one open review", first.GetId(), again.GetId(), other.GetId())
	}
	if other.GetReason() != krillv1.ReviewReason_REVIEW_REASON_FLAGGED {
		t.Errorf("reason = %v, want the first reason kept", other.GetReason())
	}
	if n, err := q.CountOpenReviews(ctx); err != nil || n != 1 {
		t.Fatalf("open reviews = %d, %v; want 1", n, err)
	}

	if _, err := q.ResolveReview(ctx, db.ResolveReviewParams{ID: first.GetId(), ResolvedBy: userID, Resolution: "accepted"}); err != nil {
		t.Fatal(err)
	}
	next, err := Enqueue(ctx, q, frameID, ReasonDrift, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if next.GetId() == first.GetId() {
		t.Error("a resolved review blocked a new one")
	}
	if next.GetCreatedBy() != nil {
		t.Errorf("created_by = %v, want unset for an automatic review", next.GetCreatedBy())
	}
}
