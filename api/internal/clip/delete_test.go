package clip

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

func TestDeleteClipClosesTheGap(t *testing.T) {
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
	s := NewService(pool, nil, nil)

	var videoID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO videos (name, filename, status, frame_count) VALUES ('v', 'v.mp4', 'ready', 40) RETURNING id`,
	).Scan(&videoID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })

	clips := make([]int64, 4)
	for i := range clips {
		err := pool.QueryRow(ctx, `
			WITH c AS (
				INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, $2, $2 * 10, 10) RETURNING id
			), f AS (
				INSERT INTO frames (video_id, clip_id, idx, phash) SELECT $1, id, $2 * 10, 0 FROM c
			)
			SELECT id FROM c`, videoID, i,
		).Scan(&clips[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.DeleteClip(ctx, &krillv1.DeleteClipRequest{Id: clips[1]}); err != nil {
		t.Fatal(err)
	}

	rows, err := pool.Query(ctx, `SELECT id FROM clips WHERE video_id = $1 AND deleted_at IS NULL ORDER BY idx`, videoID)
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		got = append(got, id)
	}
	want := []int64{clips[0], clips[2], clips[3]}
	if len(got) != len(want) {
		t.Fatalf("clips = %v, want %v", got, want)
	}
	for i := range want {
		var idx int32
		if err := pool.QueryRow(ctx, `SELECT idx FROM clips WHERE id = $1`, want[i]).Scan(&idx); err != nil {
			t.Fatal(err)
		}
		if got[i] != want[i] || idx != int32(i) {
			t.Errorf("clip %d: id %d at idx %d, want id %d at idx %d", i, got[i], idx, want[i], i)
		}
	}

	neighbours, err := db.New(pool).GetNeighbourClips(ctx, clips[2])
	if err != nil {
		t.Fatal(err)
	}
	if neighbours.PreviousID != clips[0] || neighbours.NextID != clips[3] {
		t.Errorf("neighbours of clip 2 = %+v, want %d and %d", neighbours, clips[0], clips[3])
	}

	var frameCount int32
	if err := pool.QueryRow(ctx, `SELECT frame_count FROM videos WHERE id = $1`, videoID).Scan(&frameCount); err != nil {
		t.Fatal(err)
	}
	if frameCount != 30 {
		t.Errorf("video frame_count = %d, want 30", frameCount)
	}

	_, err = s.DeleteClip(ctx, &krillv1.DeleteClipRequest{Id: clips[1]})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("deleting it again: err = %v, want not found", err)
	}
}
