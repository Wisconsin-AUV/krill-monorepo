package annotation

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/review"
)

func TestEditsClaimClips(t *testing.T) {
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
	q := db.New(pool)
	s := NewService(pool, nil, nil, func() review.GoldRouting { return review.GoldRouting{} })

	user := func(name, role string) db.User {
		var u db.User
		err := pool.QueryRow(ctx, `
			INSERT INTO users (name, username, email, role) VALUES ($1, $2, $2 || '@example.com', $3)
			RETURNING id, name, username, role`, name, "claim-test-"+uuid.NewString()[:8], role,
		).Scan(&u.ID, &u.Name, &u.Username, &u.Role)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u.ID) })
		return u
	}
	labeler := user("Ada", "labeler")
	other := user("Grace", "labeler")
	reviewer := user("Lin", "developer")

	var videoID, labelTypeID int64
	err = pool.QueryRow(ctx, `INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', 'ready') RETURNING id`).Scan(&videoID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	err = pool.QueryRow(ctx, `
		INSERT INTO label_types (name, color, position) VALUES ($1, '#fff', 0) RETURNING id`,
		"claim-test-"+uuid.NewString()[:8],
	).Scan(&labelTypeID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, labelTypeID) })

	// edit sets a box on a fresh clip, after setup has arranged its claim.
	idx := 0
	edit := func(u db.User, setup func(clipID int64)) (clipID int64) {
		var trackID, frameID int64
		err := pool.QueryRow(ctx, `
			WITH c AS (
				INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, $2, $2, 1) RETURNING id
			), f AS (
				INSERT INTO frames (video_id, clip_id, idx, phash) SELECT $1, id, $2, 0 FROM c RETURNING id
			), tr AS (
				INSERT INTO tracks (clip_id, label_type_id) SELECT id, $3 FROM c RETURNING id
			)
			SELECT c.id, tr.id, f.id FROM c, tr, f`, videoID, idx, labelTypeID,
		).Scan(&clipID, &trackID, &frameID)
		if err != nil {
			t.Fatal(err)
		}
		idx++
		setup(clipID)
		_, err = s.SetBox(auth.WithUser(ctx, u), &krillv1.SetBoxRequest{
			TrackId: trackID, FrameId: frameID, Box: &krillv1.Box{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2},
		})
		if err != nil {
			t.Fatal(err)
		}
		return clipID
	}
	hold := func(u db.User, stale bool) func(int64) {
		return func(clipID int64) {
			if err := q.TakeClip(ctx, db.TakeClipParams{ClipID: clipID, UserID: u.ID}); err != nil {
				t.Fatal(err)
			}
			if stale {
				_, err := pool.Exec(ctx, `UPDATE clip_claims SET active_at = now() - interval '1 day' WHERE clip_id = $1`, clipID)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	free := func(int64) {}

	tests := []struct {
		name   string
		editor db.User
		setup  func(int64)
		holder *db.User
	}{
		{"labeler does not take an active claim", labeler, hold(other, false), &other},
		{"labeler takes a stale claim", labeler, hold(other, true), &labeler},
		{"labeler claims a free clip", labeler, free, &labeler},
		{"reviewer does not claim a free clip", reviewer, free, nil},
		{"reviewer does not take a stale claim", reviewer, hold(other, true), &other},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clipID := edit(tt.editor, tt.setup)
			rows, err := q.ListClipClaims(ctx, []int64{clipID})
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tt.holder == nil && len(rows) != 0:
				t.Fatalf("claimed by %s, want no claim", rows[0].User.Name)
			case tt.holder != nil && (len(rows) != 1 || rows[0].User.ID != tt.holder.ID):
				t.Fatalf("claims %+v, want held by %s", rows, tt.holder.Name)
			}
		})
	}

	t.Run("reviewer keeps their own claim alive", func(t *testing.T) {
		clipID := edit(reviewer, hold(reviewer, true))
		var active bool
		err := pool.QueryRow(ctx, `SELECT active_at > now() - interval '1 minute' FROM clip_claims WHERE clip_id = $1 AND user_id = $2`,
			clipID, reviewer.ID).Scan(&active)
		if err != nil {
			t.Fatal(err)
		}
		if !active {
			t.Fatal("claim was not refreshed")
		}
	})
}
