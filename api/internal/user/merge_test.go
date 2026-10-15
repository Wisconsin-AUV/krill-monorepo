package user

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
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

func uniq() string {
	return "t" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
}

// testUser runs set against the new user, for columns CreateUser doesn't
// take.
func testUser(t *testing.T, pool *pgxpool.Pool, role, set string, args ...any) db.User {
	t.Helper()
	name := uniq()
	q := db.New(pool)
	u, err := q.CreateUser(t.Context(), db.CreateUserParams{Name: name, Username: name, Email: name + "@example.com", Role: role})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = q.DeleteUser(context.Background(), u.ID) })
	if set != "" {
		if _, err := pool.Exec(t.Context(), "UPDATE users SET "+set+" WHERE id = $1", append([]any{u.ID}, args...)...); err != nil {
			t.Fatal(err)
		}
	}
	if u, err = q.GetUser(t.Context(), u.ID); err != nil {
		t.Fatal(err)
	}
	return u
}

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMergeUsers(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	s := NewService(pool, nil)
	admin := testUser(t, pool, "admin", "")
	// A Slack sign-up with a verified email, and a password sign-up that never
	// verified its email.
	source := testUser(t, pool, "admin", "slack_user_id = $2, email_verified_at = now(), created_at = now() - interval '1 year'", "U"+uniq())
	target := testUser(t, pool, "labeler", "password_hash = 'hash'")
	as := auth.WithUser(ctx, admin)

	var videoID, frameA, frameB, clipID, trackID int64
	err := pool.QueryRow(ctx, `INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', 'ready') RETURNING id`).Scan(&videoID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	err = pool.QueryRow(ctx, `INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, 0, 0, 2) RETURNING id`, videoID).Scan(&clipID)
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range []*int64{&frameA, &frameB} {
		err := pool.QueryRow(ctx, `INSERT INTO frames (video_id, clip_id, idx, phash, status, status_by) VALUES ($1, $2, $3, 0, 'labeled', $4) RETURNING id`,
			videoID, clipID, i, source.ID).Scan(f)
		if err != nil {
			t.Fatal(err)
		}
	}
	var labelType int64
	if err := pool.QueryRow(ctx, `INSERT INTO label_types (name, color, position) VALUES ($1, '#fff', 0) RETURNING id`, uniq()).Scan(&labelType); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, labelType) })
	if err := pool.QueryRow(ctx, `INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, clipID, labelType).Scan(&trackID); err != nil {
		t.Fatal(err)
	}
	ids := pgx.NamedArgs{"track": trackID, "a": frameA, "b": frameB, "clip": clipID, "source": source.ID, "target": target.ID}
	for _, stmt := range []string{
		`INSERT INTO annotations (track_id, frame_id, x, y, width, height, created_by, updated_by)
			VALUES (@track, @a, 0.1, 0.1, 0.2, 0.2, @source, @target), (@track, @b, 0.1, 0.1, 0.2, 0.2, @source, @source)`,
		`INSERT INTO gold_frames (frame_id, boxes, created_by) VALUES (@a, '[]', @source), (@b, '[]', @source)`,
		// Both checked frame A, so the target's attempt there is kept.
		`INSERT INTO gold_attempts (frame_id, user_id, boxes, reference_count, answer_count, matched, correct, iou_sum, score)
			VALUES (@a, @target, '[]', 1, 1, 1, 1, 0.9, 0.9), (@a, @source, '[]', 1, 1, 0, 0, 0, 0.1), (@b, @source, '[]', 1, 1, 1, 1, 0.5, 0.5)`,
		`INSERT INTO clip_claims (clip_id, user_id) VALUES (@clip, @source)`,
		`INSERT INTO reviews (frame_id, reason, created_by) VALUES (@a, 'flagged', @source)`,
	} {
		if _, err := pool.Exec(ctx, stmt, ids); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	for _, u := range []db.User{source, target} {
		if _, err := pool.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, now() + interval '1 day')`,
			[]byte(uniq()), u.ID); err != nil {
			t.Fatal(err)
		}
	}

	preview, err := s.PreviewMergeUsers(as, &krillv1.PreviewMergeUsersRequest{SourceId: source.ID.String(), TargetId: target.ID.String()})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int64{
		"Boxes drawn":        {2, 0},
		"Boxes last edited":  {1, 0},
		"Frames marked":      {2, 0},
		"Gold frames picked": {2, 0},
		"Gold checks":        {1, 1},
		"Clip claims":        {1, 0},
		"Reviews flagged":    {1, 0},
	}
	got := map[string][2]int64{}
	for _, c := range preview.GetCounts() {
		got[c.GetLabel()] = [2]int64{c.GetMoved(), c.GetDropped()}
	}
	for label, w := range want {
		if got[label] != w {
			t.Errorf("preview %s = %v, want %v", label, got[label], w)
		}
	}
	if r := preview.GetResult(); r.GetRole() != krillv1.Role_ROLE_ADMIN || !r.GetSlackLinked() || !r.GetHasPassword() || r.GetEmail() != source.Email {
		t.Errorf("preview result = %v", r)
	}
	if count(t, pool, `SELECT count(*) FROM users WHERE id = $1`, source.ID) != 1 {
		t.Fatal("preview deleted the source")
	}

	res, err := s.MergeUsers(as, &krillv1.MergeUsersRequest{SourceId: source.ID.String(), TargetId: target.ID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if count(t, pool, `SELECT count(*) FROM users WHERE id = $1`, source.ID) != 0 {
		t.Error("source still exists")
	}
	merged, err := db.New(pool).GetUser(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	// An admin merged into a labeler stays an admin.
	if merged.Role != "admin" {
		t.Errorf("role = %s, want admin", merged.Role)
	}
	if merged.SlackUserID != source.SlackUserID || merged.PasswordHash.String != "hash" {
		t.Errorf("sign-in methods = %v, %v", merged.SlackUserID, merged.PasswordHash)
	}
	if merged.Email != source.Email || !merged.EmailVerifiedAt.Valid {
		t.Errorf("email = %s verified %v, want the source's verified email", merged.Email, merged.EmailVerifiedAt.Valid)
	}
	if !merged.CreatedAt.Time.Equal(source.CreatedAt.Time) {
		t.Errorf("created_at = %v, want the older account's", merged.CreatedAt.Time)
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM annotations WHERE track_id = @track AND created_by = @target`:            2,
		`SELECT count(*) FROM annotations WHERE track_id = @track AND updated_by = @target`:            2,
		`SELECT count(*) FROM frames WHERE clip_id = @clip AND status_by = @target`:                    2,
		`SELECT count(*) FROM gold_frames WHERE frame_id IN (@a, @b) AND created_by = @target`:         2,
		`SELECT count(*) FROM gold_attempts WHERE frame_id = @a AND user_id = @target AND score = 0.9`: 1,
		`SELECT count(*) FROM gold_attempts WHERE frame_id = @b AND user_id = @target`:                 1,
		`SELECT count(*) FROM clip_claims WHERE clip_id = @clip AND user_id = @target`:                 1,
		`SELECT count(*) FROM reviews WHERE frame_id = @a AND created_by = @target`:                    1,
		// The source's session ended rather than moving; the target's stays.
		`SELECT count(*) FROM sessions WHERE user_id = @target`: 1,
	} {
		if got := count(t, pool, query, ids); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	if count(t, pool, `SELECT count(*) FROM audit_log WHERE action = 'user.merge' AND target_id = $1`, target.ID.String()) != 1 {
		t.Error("merge not audited")
	}
	if len(res.GetCounts()) != len(preview.GetCounts()) {
		t.Errorf("merge counts = %v, preview = %v", res.GetCounts(), preview.GetCounts())
	}
}

func TestMergeRefusals(t *testing.T) {
	pool := testPool(t)
	s := NewService(pool, nil)
	admin := testUser(t, pool, "admin", "")
	other := testUser(t, pool, "labeler", "")
	as := auth.WithUser(t.Context(), admin)

	for name, tt := range map[string]struct {
		source, target uuid.UUID
		want           connect.Code
	}{
		"merging yourself away": {admin.ID, other.ID, connect.CodeFailedPrecondition},
		"into itself":           {other.ID, other.ID, connect.CodeInvalidArgument},
		"missing target":        {other.ID, uuid.New(), connect.CodeNotFound},
	} {
		_, err := s.MergeUsers(as, &krillv1.MergeUsersRequest{SourceId: tt.source.String(), TargetId: tt.target.String()})
		if connect.CodeOf(err) != tt.want {
			t.Errorf("%s: err = %v, want %v", name, err, tt.want)
		}
	}
	if count(t, pool, `SELECT count(*) FROM users WHERE id IN ($1, $2)`, admin.ID, other.ID) != 2 {
		t.Error("a refused merge deleted an account")
	}
}

// A migration that adds a column referencing users has to say what a merge
// does with it, or the merge would delete those rows or leave them on a
// deleted account.
func TestMergeHandlesEveryUserReference(t *testing.T) {
	pool := testPool(t)
	rows, err := pool.Query(t.Context(), `
		SELECT kcu.table_name, kcu.column_name
		FROM information_schema.referential_constraints rc
		JOIN information_schema.key_column_usage kcu
			ON kcu.constraint_schema = rc.constraint_schema AND kcu.constraint_name = rc.constraint_name
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_schema = rc.unique_constraint_schema AND ccu.constraint_name = rc.unique_constraint_name
		WHERE ccu.table_schema = current_schema() AND ccu.table_name = 'users'`)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			t.Fatal(err)
		}
		found = append(found, table+"."+column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	var handled []string
	for _, r := range userRefs {
		handled = append(handled, r.table+"."+r.column)
	}
	for _, f := range found {
		if !slices.Contains(handled, f) {
			t.Errorf("%s references users but userRefs doesn't list it", f)
		}
	}
	for _, h := range handled {
		if !slices.Contains(found, h) {
			t.Errorf("userRefs lists %s, which doesn't reference users", h)
		}
	}
	if len(found) == 0 {
		t.Fatal("found no references to users")
	}
}

// Moving a row onto the target fails on a unique key the target already
// fills, unless userRefs names the key so the merge keeps the target's row.
func TestMergeKnowsEveryUniqueKey(t *testing.T) {
	pool := testPool(t)
	for _, r := range userRefs {
		if r.drop {
			continue
		}
		rows, err := pool.Query(t.Context(), `
			SELECT array_agg(a.attname::text ORDER BY a.attname)
			FROM pg_index i
			JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
			WHERE i.indrelid = $1::regclass AND i.indisunique
			GROUP BY i.indexrelid
			HAVING $2 = ANY(array_agg(a.attname::text))`, r.table, r.column)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var cols []string
			if err := rows.Scan(&cols); err != nil {
				t.Fatal(err)
			}
			want := slices.DeleteFunc(cols, func(c string) bool { return c == r.column })
			key := slices.Sorted(slices.Values(r.key))
			if !slices.Equal(want, key) {
				t.Errorf("%s.%s is unique with %v, but userRefs has key %v", r.table, r.column, want, r.key)
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}
}
