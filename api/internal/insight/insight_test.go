package insight

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/review"
)

type fixture struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	video int64
	label int64
	idx   int
}

func setup(t *testing.T) *fixture {
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
	f := &fixture{t: t, ctx: ctx, pool: pool}
	err = pool.QueryRow(ctx, `INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', 'ready') RETURNING id`).Scan(&f.video)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, f.video) })
	err = pool.QueryRow(ctx, `INSERT INTO label_types (name, color, position) VALUES ($1, '#fff', 0) RETURNING id`,
		"insight-test-"+uuid.NewString()[:8]).Scan(&f.label)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, f.label) })
	return f
}

func (f *fixture) user(name, role string) db.User {
	var u db.User
	err := f.pool.QueryRow(f.ctx, `
		INSERT INTO users (name, username, email, role) VALUES ($1, $2, $2 || '@example.com', $3)
		RETURNING id, name, username, role`, name, "insight-test-"+uuid.NewString()[:8], role,
	).Scan(&u.ID, &u.Name, &u.Username, &u.Role)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM audit_log WHERE actor = $1`, u.ID)
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u.ID)
	})
	return u
}

// clip makes a clip with frames frames and returns its id and frame ids.
func (f *fixture) clip(frames int) (int64, []int64) {
	var clipID int64
	err := f.pool.QueryRow(f.ctx, `INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, $2, $2, $3) RETURNING id`,
		f.video, f.idx, frames).Scan(&clipID)
	if err != nil {
		f.t.Fatal(err)
	}
	f.idx++
	ids := make([]int64, frames)
	for i := range ids {
		err := f.pool.QueryRow(f.ctx, `INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, $3, 0) RETURNING id`,
			f.video, clipID, 1000*f.idx+i).Scan(&ids[i])
		if err != nil {
			f.t.Fatal(err)
		}
	}
	return clipID, ids
}

func box(x float64) *krillv1.Box {
	return &krillv1.Box{X: x, Y: 0.1, Width: 0.2, Height: 0.2}
}

func find(t *testing.T, s *Service, ctx context.Context, u db.User) *krillv1.LabelerStats {
	res, err := s.GetLabelerStats(ctx, &krillv1.GetLabelerStatsRequest{Period: krillv1.Period_PERIOD_ALL_TIME})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range res.GetLabelers() {
		if l.GetUser().GetId() == u.ID.String() {
			return l
		}
	}
	return &krillv1.LabelerStats{}
}

// Corrections must count against whoever marked the frame, and only when
// someone else changes it afterwards. Fixing your own frame is labeling.
func TestCorrectionsCountAgainstTheLabeler(t *testing.T) {
	f := setup(t)
	labeler := f.user("Ada", "labeler")
	reviewer := f.user("Lin", "developer")
	ann := annotation.NewService(f.pool, nil, nil, func() review.GoldRouting { return review.GoldRouting{} })
	s := NewService(f.pool, nil)
	as := func(u db.User) context.Context { return auth.WithUser(f.ctx, u) }

	clipID, frames := f.clip(3)
	created, err := ann.CreateTrack(as(labeler), &krillv1.CreateTrackRequest{
		ClipId: clipID, FrameId: frames[0], LabelTypeId: f.label, Box: box(0.1),
	})
	if err != nil {
		t.Fatal(err)
	}
	track := created.GetTrack().GetId()
	for _, fr := range frames[1:] {
		if _, err := ann.SetBox(as(labeler), &krillv1.SetBoxRequest{TrackId: track, FrameId: fr, Box: box(0.1)}); err != nil {
			t.Fatal(err)
		}
	}
	// The labeler moves their own box after marking the frame.
	mark := func(u db.User, frame int64) {
		_, err := ann.SetFrameStatus(as(u), &krillv1.SetFrameStatusRequest{FrameId: frame, Status: krillv1.FrameStatus_FRAME_STATUS_LABELED})
		if err != nil {
			t.Fatal(err)
		}
	}
	mark(labeler, frames[0])
	mark(labeler, frames[1])
	if _, err := ann.SetBox(as(labeler), &krillv1.SetBoxRequest{TrackId: track, FrameId: frames[0], Box: box(0.3)}); err != nil {
		t.Fatal(err)
	}
	if got := find(t, s, f.ctx, labeler).GetFramesCorrected(); got != 0 {
		t.Fatalf("own fix counted as %d corrected frames", got)
	}

	// The reviewer moves a box on frame 0 twice, deletes the one on frame 1,
	// and fixes frame 2, which nobody marked yet.
	for _, x := range []float64{0.5, 0.6} {
		if _, err := ann.SetBox(as(reviewer), &krillv1.SetBoxRequest{TrackId: track, FrameId: frames[0], Box: box(x)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ann.DeleteBox(as(reviewer), &krillv1.DeleteBoxRequest{TrackId: track, FrameId: frames[1]}); err != nil {
		t.Fatal(err)
	}
	if _, err := ann.SetBox(as(reviewer), &krillv1.SetBoxRequest{TrackId: track, FrameId: frames[2], Box: box(0.5)}); err != nil {
		t.Fatal(err)
	}

	l := find(t, s, f.ctx, labeler)
	if l.GetFramesMarked() != 2 || l.GetFramesCorrected() != 2 || l.GetBoxesCorrected() != 2 {
		t.Errorf("marked %d, corrected %d frames and %d boxes, want 2, 2, 2",
			l.GetFramesMarked(), l.GetFramesCorrected(), l.GetBoxesCorrected())
	}
	if r := find(t, s, f.ctx, reviewer); r.GetFramesCorrected() != 0 {
		t.Errorf("reviewer has %d corrected frames", r.GetFramesCorrected())
	}

	trend, err := s.GetLabelerTrend(f.ctx, &krillv1.GetLabelerTrendRequest{
		UserId: labeler.ID.String(), Period: krillv1.Period_PERIOD_WEEK, TimeZone: "America/Chicago",
	})
	if err != nil {
		t.Fatal(err)
	}
	var marked, corrected int64
	for _, p := range trend.GetPoints() {
		marked += p.GetFrames()
		corrected += p.GetFramesCorrected()
	}
	if marked != 2 || corrected != 2 {
		t.Errorf("trend has %d marked and %d corrected frames, want 2 and 2", marked, corrected)
	}
}

// Held-out test footage must not make a class look covered.
func TestCoverageLeavesOutHeldOutVideos(t *testing.T) {
	f := setup(t)
	labeler := f.user("Ada", "labeler")
	ann := annotation.NewService(f.pool, nil, nil, func() review.GoldRouting { return review.GoldRouting{} })
	s := NewService(f.pool, nil)

	clipID, frames := f.clip(1)
	_, err := ann.CreateTrack(auth.WithUser(f.ctx, labeler), &krillv1.CreateTrackRequest{
		ClipId: clipID, FrameId: frames[0], LabelTypeId: f.label, Box: box(0.1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE videos SET split = 'test' WHERE id = $1`, f.video); err != nil {
		t.Fatal(err)
	}
	res, err := s.GetCoverage(f.ctx, &krillv1.GetCoverageRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, lt := range res.GetLabelTypes() {
		if lt.GetId() != f.label {
			continue
		}
		if lt.GetVerified() != 0 || lt.GetVideos() != 0 || lt.GetHeldOutVerified() != 1 || !lt.GetFewVideos() {
			t.Errorf("coverage = %v, want the box counted only as held out", lt)
		}
		return
	}
	t.Fatal("label type missing from coverage")
}

// A proposal counts once for whoever decided it: kept by marking the frame,
// edited by moving it, rejected by deleting it.
func TestProposalOutcomes(t *testing.T) {
	f := setup(t)
	labeler := f.user("Ada", "labeler")
	ann := annotation.NewService(f.pool, nil, nil, func() review.GoldRouting { return review.GoldRouting{} })
	s := NewService(f.pool, nil)
	ctx := auth.WithUser(f.ctx, labeler)

	clipID, frames := f.clip(3)
	var track int64
	err := f.pool.QueryRow(f.ctx, `INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, clipID, f.label).Scan(&track)
	if err != nil {
		t.Fatal(err)
	}
	for _, fr := range frames {
		_, err := f.pool.Exec(f.ctx, `
			INSERT INTO annotations (track_id, frame_id, x, y, width, height, source, status)
			VALUES ($1, $2, 0.1, 0.1, 0.2, 0.2, 'sam', 'proposed')`, track, fr)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ann.SetFrameStatus(ctx, &krillv1.SetFrameStatusRequest{FrameId: frames[0], Status: krillv1.FrameStatus_FRAME_STATUS_LABELED}); err != nil {
		t.Fatal(err)
	}
	if _, err := ann.SetBox(ctx, &krillv1.SetBoxRequest{TrackId: track, FrameId: frames[1], Box: box(0.4)}); err != nil {
		t.Fatal(err)
	}
	if _, err := ann.DeleteBox(ctx, &krillv1.DeleteBoxRequest{TrackId: track, FrameId: frames[2]}); err != nil {
		t.Fatal(err)
	}

	l := find(t, s, f.ctx, labeler)
	if len(l.GetProposals()) != 1 {
		t.Fatalf("proposals = %v", l.GetProposals())
	}
	p := l.GetProposals()[0]
	if p.GetSource() != "sam" || p.GetAccepted() != 1 || p.GetEdited() != 1 || p.GetRejected() != 1 {
		t.Errorf("proposals = %v, want one sam proposal kept, edited, and rejected", p)
	}
	if l.GetBoxesDrawn() != 0 {
		t.Errorf("boxes drawn = %d, want 0", l.GetBoxesDrawn())
	}
}

// Releasing must leave a claim alone if its labeler came back after the
// list loaded, and record each release it makes.
func TestReleaseKeepsReturnedClaims(t *testing.T) {
	f := setup(t)
	admin := f.user("Root", "admin")
	idle := f.user("Ada", "labeler")
	back := f.user("Grace", "labeler")
	s := NewService(f.pool, nil)

	claim := func(u db.User, activeAt time.Time) int64 {
		clipID, _ := f.clip(2)
		_, err := f.pool.Exec(f.ctx, `INSERT INTO clip_claims (clip_id, user_id, active_at) VALUES ($1, $2, $3)`, clipID, u.ID, activeAt)
		if err != nil {
			t.Fatal(err)
		}
		return clipID
	}
	old := time.Now().AddDate(0, 0, -10)
	stuck := claim(idle, old)
	returned := claim(back, old)
	list, err := s.ListStuckClips(f.ctx, &krillv1.ListStuckClipsRequest{IdleDays: 3})
	if err != nil {
		t.Fatal(err)
	}
	listed := map[int64]bool{}
	for _, c := range list.GetClips() {
		listed[c.GetClipId()] = true
	}
	if !listed[stuck] || !listed[returned] {
		t.Fatalf("listed %v, want clips %d and %d", listed, stuck, returned)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE clip_claims SET active_at = now() WHERE clip_id = $1`, returned); err != nil {
		t.Fatal(err)
	}

	ctx := auth.WithUser(f.ctx, admin)
	res, err := s.ReleaseStuckClips(ctx, &krillv1.ReleaseStuckClipsRequest{ClipIds: []int64{stuck, returned}, IdleDays: 3})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetReleased() != 1 {
		t.Errorf("released %d, want 1", res.GetReleased())
	}
	var claims, entries int
	err = f.pool.QueryRow(f.ctx, `SELECT count(*) FROM clip_claims WHERE clip_id = ANY($1)`, []int64{stuck, returned}).Scan(&claims)
	if err != nil {
		t.Fatal(err)
	}
	err = f.pool.QueryRow(f.ctx, `SELECT count(*) FROM audit_log WHERE actor = $1 AND action = 'clip.release' AND target_id = $2`,
		admin.ID, strconv.FormatInt(stuck, 10)).Scan(&entries)
	if err != nil {
		t.Fatal(err)
	}
	if claims != 1 || entries != 1 {
		t.Errorf("%d claims left and %d audit entries, want 1 and 1", claims, entries)
	}
}
