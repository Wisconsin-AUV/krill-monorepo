package user

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
)

// testSession starts a session for u and returns its ID and token hash.
func testSession(t *testing.T, pool *pgxpool.Pool, u db.User, lastSeen string) (uuid.UUID, []byte) {
	t.Helper()
	hash := []byte(uniq())
	var id uuid.UUID
	err := pool.QueryRow(t.Context(), `
		INSERT INTO sessions (token_hash, user_id, expires_at, ip, last_seen_at)
		VALUES ($1, $2, now() + interval '1 day', '10.0.0.1', now() - $3::interval) RETURNING id`,
		hash, u.ID, lastSeen).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id, hash
}

func withSession(ctx context.Context, u db.User, hash []byte) context.Context {
	return auth.WithSession(ctx, auth.Session{User: u, TokenHash: hash})
}

func TestDeactivateUser(t *testing.T) {
	pool := testPool(t)
	s := NewService(pool, nil)
	admin := testUser(t, pool, "admin", "")
	u := testUser(t, pool, "labeler", "")
	_, hash := testSession(t, pool, u, "0")

	var videoID int64
	err := pool.QueryRow(t.Context(), `
		WITH v AS (INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', 'ready') RETURNING id),
		c AS (INSERT INTO clips (video_id, idx, start_frame, frame_count) SELECT id, 0, 0, 1 FROM v RETURNING id, video_id),
		f AS (INSERT INTO frames (video_id, clip_id, idx, phash, status, status_by) SELECT video_id, id, 0, 0, 'labeled', $1 FROM c),
		cc AS (INSERT INTO clip_claims (clip_id, user_id) SELECT id, $1 FROM c)
		SELECT id FROM v`, u.ID).Scan(&videoID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })

	_, err = s.DeactivateUser(auth.WithUser(t.Context(), admin), &krillv1.DeactivateUserRequest{Id: admin.ID.String()})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("deactivating yourself: err = %v", err)
	}

	res, err := s.DeactivateUser(auth.WithUser(t.Context(), admin), &krillv1.DeactivateUserRequest{Id: u.ID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if !res.GetUser().GetDisabled() || res.GetSessionsEnded() != 1 || res.GetClaimsReleased() != 1 {
		t.Errorf("response = %v", res)
	}
	if _, err := auth.LookupSessions(db.New(pool))(t.Context(), hash); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("session still works: %v", err)
	}
	if count(t, pool, `SELECT count(*) FROM clip_claims WHERE user_id = $1`, u.ID) != 0 {
		t.Error("claims kept")
	}
	if count(t, pool, `SELECT count(*) FROM frames WHERE video_id = $1 AND status_by = $2`, videoID, u.ID) != 1 {
		t.Error("labels lost their author")
	}
}

func TestRevokeMySession(t *testing.T) {
	pool := testPool(t)
	s := NewSessionService(pool)
	me := testUser(t, pool, "labeler", "")
	other := testUser(t, pool, "labeler", "")
	_, current := testSession(t, pool, me, "0")
	laptop, _ := testSession(t, pool, me, "0")
	theirs, _ := testSession(t, pool, other, "0")
	ctx := withSession(t.Context(), me, current)

	list, err := s.ListMySessions(ctx, &krillv1.ListMySessionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var currents int
	for _, sess := range list.GetSessions() {
		if sess.GetUser().GetId() != me.ID.String() {
			t.Errorf("listed someone else's session")
		}
		if sess.GetCurrent() {
			currents++
		}
	}
	if len(list.GetSessions()) != 2 || currents != 1 {
		t.Errorf("sessions = %d, current = %d", len(list.GetSessions()), currents)
	}

	_, err = s.RevokeMySession(ctx, &krillv1.RevokeMySessionRequest{Id: theirs.String()})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("revoking someone else's: err = %v", err)
	}
	if count(t, pool, `SELECT count(*) FROM sessions WHERE id = $1`, theirs) != 1 {
		t.Error("revoked someone else's session")
	}

	for _, sess := range list.GetSessions() {
		if sess.GetCurrent() {
			_, err := s.RevokeMySession(ctx, &krillv1.RevokeMySessionRequest{Id: sess.GetId()})
			if connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Errorf("revoking the current session: err = %v", err)
			}
		}
	}
	if count(t, pool, `SELECT count(*) FROM sessions WHERE token_hash = $1`, current) != 1 {
		t.Error("revoked the current session")
	}

	if _, err := s.RevokeMySession(ctx, &krillv1.RevokeMySessionRequest{Id: laptop.String()}); err != nil {
		t.Fatal(err)
	}
	if count(t, pool, `SELECT count(*) FROM sessions WHERE id = $1`, laptop) != 0 {
		t.Error("session not revoked")
	}
	if count(t, pool, `SELECT count(*) FROM audit_log WHERE action = 'session.revoke' AND actor = $1`, me.ID) != 1 {
		t.Error("revoke not audited")
	}
}

func TestSessionLastSeenIsThrottled(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool, "labeler", "")
	lookup := auth.LookupSessions(db.New(pool))
	for _, tt := range []struct {
		lastSeen string
		wantIP   string
	}{
		{"1 minute", "10.0.0.1"},
		{"10 minutes", "10.0.0.2"},
	} {
		id, hash := testSession(t, pool, u, tt.lastSeen)
		if _, err := lookup(auth.WithClientIP(t.Context(), "10.0.0.2"), hash); err != nil {
			t.Fatal(err)
		}
		var ip string
		if err := pool.QueryRow(t.Context(), `SELECT ip FROM sessions WHERE id = $1`, id).Scan(&ip); err != nil {
			t.Fatal(err)
		}
		if ip != tt.wantIP {
			t.Errorf("seen %s ago: ip = %s, want %s", tt.lastSeen, ip, tt.wantIP)
		}
	}
}
