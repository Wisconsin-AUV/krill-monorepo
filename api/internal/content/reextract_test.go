package content

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/compute"
	"github.com/wauv/krill/api/internal/db"
)

func TestReextractNeedsConsentToDiscardLabels(t *testing.T) {
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
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatal(err)
	}
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(pool, nil, jobs, compute.NewRegistry(), func() int { return 0 })

	video := func(labeled bool) int64 {
		var id int64
		err := pool.QueryRow(ctx, `
			WITH v AS (
				INSERT INTO videos (name, filename, status, extract_fps) VALUES ('v', 'v.mp4', 'ready', 5) RETURNING id
			), c AS (
				INSERT INTO clips (video_id, idx, start_frame, frame_count) SELECT id, 0, 0, 1 FROM v RETURNING id, video_id
			), f AS (
				INSERT INTO frames (video_id, clip_id, idx, phash, status)
				SELECT video_id, id, 0, 0, CASE WHEN $1 THEN 'empty' ELSE 'unlabeled' END FROM c
			)
			SELECT id FROM v`, labeled,
		).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, id) })
		return id
	}
	status := func(id int64) (string, float64) {
		var s string
		var fps float64
		if err := pool.QueryRow(ctx, `SELECT status, extract_fps FROM videos WHERE id = $1`, id).Scan(&s, &fps); err != nil {
			t.Fatal(err)
		}
		return s, fps
	}

	unlabeled, labeled := video(false), video(true)
	_, err = s.ReextractVideos(ctx, &krillv1.ReextractVideosRequest{VideoIds: []int64{unlabeled, labeled}})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("re-extracting a labeled video without consent: err = %v, want failed precondition", err)
	}
	if got, _ := status(unlabeled); got != "ready" {
		t.Errorf("the unlabeled video in the refused request is %s, want ready", got)
	}

	fps := 10.0
	res, err := s.ReextractVideos(ctx, &krillv1.ReextractVideosRequest{
		VideoIds: []int64{unlabeled, labeled}, DiscardLabels: true, ExtractFps: &fps,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetJob().GetTotal() != 2 {
		t.Errorf("job has %d ingests, want 2", res.GetJob().GetTotal())
	}
	for _, id := range []int64{unlabeled, labeled} {
		if got, gotFPS := status(id); got != "queued" || gotFPS != fps {
			t.Errorf("video %d is %s at %v fps, want queued at %v", id, got, gotFPS, fps)
		}
	}

	_, err = s.ReextractVideos(ctx, &krillv1.ReextractVideosRequest{VideoIds: []int64{unlabeled}})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("re-extracting a queued video: err = %v, want failed precondition", err)
	}
}
