package video

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

func TestMovingToTestRemovesModelProposals(t *testing.T) {
	url := os.Getenv("KRILL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KRILL_TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewService(pool, nil, nil, func() bool { return false })

	id := func(sql string, args ...any) int64 {
		t.Helper()
		var v int64
		if err := pool.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return v
	}
	typeID := id(`INSERT INTO label_types (name, color, position) VALUES ('heldout_test_buoy', '#ff0000', 99) RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, typeID) })
	videoID := id(`INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', 'processing') RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	clipID := id(`INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, 0, 0, 2) RETURNING id`, videoID)
	f0 := id(`INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, 0, 0) RETURNING id`, videoID, clipID)
	f1 := id(`INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, 1, 0) RETURNING id`, videoID, clipID)

	track := func(boxes ...[3]any) int64 {
		tr := id(`INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, clipID, typeID)
		for _, b := range boxes {
			_, err := pool.Exec(ctx, `
				INSERT INTO annotations (track_id, frame_id, x, y, width, height, source, status)
				VALUES ($1, $2, 0.1, 0.1, 0.2, 0.2, $3, $4)`, tr, b[0], b[1], b[2])
			if err != nil {
				t.Fatal(err)
			}
		}
		return tr
	}
	yolo := track([3]any{f0, "yolo", "proposed"})
	cv := track([3]any{f0, "cv", "proposed"})
	sam := track([3]any{f0, "sam", "proposed"})
	human := track([3]any{f0, "human", "verified"})
	accepted := track([3]any{f0, "yolo", "verified"}, [3]any{f1, "yolo", "proposed"})

	split := krillv1.SplitAssignment_SPLIT_ASSIGNMENT_TEST
	if _, err := s.UpdateVideo(ctx, &krillv1.UpdateVideoRequest{Id: videoID, Split: &split}); err != nil {
		t.Fatal(err)
	}

	rows, err := pool.Query(ctx, `
		SELECT t.id, coalesce(a.source || '/' || a.status, '')
		FROM tracks t LEFT JOIN annotations a ON a.track_id = t.id
		WHERE t.clip_id = $1 ORDER BY t.id, a.frame_id`, clipID)
	if err != nil {
		t.Fatal(err)
	}
	type box struct {
		track int64
		state string
	}
	var got []box
	for rows.Next() {
		var b box
		if err := rows.Scan(&b.track, &b.state); err != nil {
			t.Fatal(err)
		}
		got = append(got, b)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []box{{sam, "sam/proposed"}, {human, "human/verified"}, {accepted, "yolo/verified"}}
	if !slices.Equal(got, want) {
		t.Errorf("boxes = %+v, want %+v with tracks %d and %d gone", got, want, yolo, cv)
	}
}
