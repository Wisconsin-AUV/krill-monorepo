package queue

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/video"
)

func TestTakeClipAuditsOnlyTakingAnActiveClaim(t *testing.T) {
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
	s := NewService(pool, nil)

	user := func(name string) db.User {
		var u db.User
		err := pool.QueryRow(ctx, `
			INSERT INTO users (name, username, email, role) VALUES ($1, $2, $2 || '@example.com', 'labeler')
			RETURNING id, name, username, role`, name, "take-test-"+uuid.NewString()[:8],
		).Scan(&u.ID, &u.Name, &u.Username, &u.Role)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u.ID) })
		return u
	}
	me := user("Ada")
	other := user("Grace")

	var videoID int64
	err = pool.QueryRow(ctx, `INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', 'ready') RETURNING id`).Scan(&videoID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM audit_log WHERE actor = $1`, me.ID) })

	stale := time.Now().Add(-video.ClaimTimeout - time.Minute)
	for i, tt := range []struct {
		name    string
		holder  *db.User
		active  time.Time
		audited bool
	}{
		{"unclaimed", nil, time.Time{}, false},
		{"held by me", &me, time.Now(), false},
		{"held by someone else", &other, time.Now(), true},
		{"abandoned by someone else", &other, stale, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var clipID int64
			err := pool.QueryRow(ctx, `
				INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, $2, $2, 1) RETURNING id`,
				videoID, i,
			).Scan(&clipID)
			if err != nil {
				t.Fatal(err)
			}
			if tt.holder != nil {
				_, err := pool.Exec(ctx, `INSERT INTO clip_claims (clip_id, user_id, active_at) VALUES ($1, $2, $3)`,
					clipID, tt.holder.ID, tt.active)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.TakeClip(auth.WithUser(ctx, me), &krillv1.TakeClipRequest{ClipId: clipID}); err != nil {
				t.Fatal(err)
			}
			var n int
			err = pool.QueryRow(ctx, `
				SELECT count(*) FROM audit_log
				WHERE action = 'clip.take' AND target_id = $1::bigint::text AND actor = $2 AND details->>'from' = $3`,
				clipID, me.ID, other.Name,
			).Scan(&n)
			if err != nil {
				t.Fatal(err)
			}
			if got := n == 1; got != tt.audited {
				t.Errorf("audited = %v, want %v", got, tt.audited)
			}
		})
	}
}
