package trash_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/export"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/trash"
)

const framesPerClip = 10

type fixture struct {
	t       *testing.T
	ctx     context.Context
	pool    *pgxpool.Pool
	q       *db.Queries
	videoID int64
	clips   []int64
	// frames[i] are clip i's frame IDs.
	frames [][]int64
	user   uuid.UUID
}

func setup(t *testing.T, clipCount int) *fixture {
	t.Helper()
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
	f := &fixture{t: t, ctx: ctx, pool: pool, q: db.New(pool)}

	err = pool.QueryRow(ctx, `
		INSERT INTO users (name, username, email) VALUES ('Ada', $1, $1 || '@example.com') RETURNING id`,
		"trash-test-"+uuid.NewString()[:8],
	).Scan(&f.user)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.user) })

	err = pool.QueryRow(ctx, `
		INSERT INTO videos (name, filename, status, frame_count) VALUES ('harbour', 'harbour.mp4', 'ready', $1) RETURNING id`,
		clipCount*framesPerClip,
	).Scan(&f.videoID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, f.videoID) })

	for i := range clipCount {
		var clipID int64
		err := pool.QueryRow(ctx, `
			INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, $2::int, $2::int * $3::int, $3::int) RETURNING id`,
			f.videoID, i, framesPerClip,
		).Scan(&clipID)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := pool.Query(ctx, `
			INSERT INTO frames (video_id, clip_id, idx, phash)
			SELECT $1, $2, $3::int * $4::int + n, 0 FROM generate_series(0, $4::int - 1) n
			RETURNING id`, f.videoID, clipID, i, framesPerClip)
		if err != nil {
			t.Fatal(err)
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(ids)
		f.clips = append(f.clips, clipID)
		f.frames = append(f.frames, ids)
	}
	return f
}

// label gives the clip a done frame with a verified box.
func (f *fixture) label(clip int) {
	f.t.Helper()
	var typeID int64
	err := f.pool.QueryRow(f.ctx, `
		INSERT INTO label_types (name, color, position) VALUES ($1, '#ffffff', 0) RETURNING id`,
		"trash_test_"+uuid.NewString()[:8],
	).Scan(&typeID)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM tracks WHERE label_type_id = $1`, typeID)
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, typeID)
	})
	frame := f.frames[clip][0]
	_, err = f.pool.Exec(f.ctx, `
		WITH t AS (INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id)
		INSERT INTO annotations (track_id, frame_id, x, y, width, height) SELECT id, $3, 0.1, 0.1, 0.2, 0.2 FROM t`,
		f.clips[clip], typeID, frame)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE frames SET status = 'labeled' WHERE id = $1`, frame); err != nil {
		f.t.Fatal(err)
	}
}

// live returns the video's live clips in index order, checking the indexes
// have no gaps.
func (f *fixture) live() []int64 {
	f.t.Helper()
	rows, err := f.pool.Query(f.ctx, `SELECT id, idx FROM clips WHERE video_id = $1 AND deleted_at IS NULL ORDER BY idx`, f.videoID)
	if err != nil {
		f.t.Fatal(err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		var idx int32
		if err := rows.Scan(&id, &idx); err != nil {
			f.t.Fatal(err)
		}
		if int(idx) != len(ids) {
			f.t.Fatalf("clip %d at index %d, want %d", id, idx, len(ids))
		}
		ids = append(ids, id)
	}
	return ids
}

func (f *fixture) count(query string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, query, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func TestTrashedContentIsLeftOut(t *testing.T) {
	f := setup(t, 3)
	f.label(0)
	f.label(1)
	if err := f.q.TakeClip(f.ctx, db.TakeClipParams{ClipID: f.clips[1], UserID: f.user}); err != nil {
		t.Fatal(err)
	}

	// boxes counts what an export of the video would hold.
	boxes := func() int32 {
		t.Helper()
		p, err := export.Load(f.ctx, f.q, &krillv1.ExportOptions{VideoIds: []int64{f.videoID}, Stride: 1})
		if err != nil {
			t.Fatal(err)
		}
		s := p.Stats
		return s.GetTrainBoxes() + s.GetValBoxes() + s.GetTestBoxes() + s.GetUnlabeledFrames()
	}
	if got := boxes(); got != 2+framesPerClip*3-2 {
		t.Fatalf("exported boxes and unlabeled frames before trashing = %d, want %d", got, framesPerClip*3)
	}

	if err := trash.TrashClip(f.ctx, f.q, f.clips[1]); err != nil {
		t.Fatal(err)
	}
	if got := boxes(); got != 1+framesPerClip*2-1 {
		t.Errorf("exported boxes and unlabeled frames with a clip trashed = %d, want %d", got, framesPerClip*2)
	}
	if _, err := f.q.GetClip(f.ctx, f.clips[1]); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetClip of a trashed clip: err = %v, want no rows", err)
	}
	if _, err := f.q.GetFrame(f.ctx, f.frames[1][0]); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetFrame in a trashed clip: err = %v, want no rows", err)
	}
	open, err := f.q.ListOpenClips(f.ctx, f.user)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 0 {
		t.Errorf("open clips = %d, want the trashed clip's claim released", len(open))
	}
	progress, err := f.q.ListClipProgress(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range progress {
		if p.ID == f.clips[1] {
			t.Error("queue progress lists the trashed clip")
		}
	}

	if _, err := trash.TrashVideo(f.ctx, f.q, f.videoID); err != nil {
		t.Fatal(err)
	}
	if got := boxes(); got != 0 {
		t.Errorf("export of a trashed video counts %d boxes and unlabeled frames, want none", got)
	}
	if _, err := f.q.GetClip(f.ctx, f.clips[0]); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetClip in a trashed video: err = %v, want no rows", err)
	}
	progress, err = f.q.ListClipProgress(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range progress {
		if p.VideoID == f.videoID {
			t.Error("queue progress lists a clip of the trashed video")
		}
	}
}

func TestRestoreRoundTripsClip(t *testing.T) {
	f := setup(t, 4)
	f.label(1)
	labels := func(clip int64) int {
		return f.count(`
			SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id
			JOIN frames fr ON fr.id = a.frame_id
			WHERE t.clip_id = $1 AND fr.status = 'labeled'`, clip)
	}
	frameCount := func() int { return f.count(`SELECT frame_count FROM videos WHERE id = $1`, f.videoID) }
	c := f.clips

	if err := trash.TrashClip(f.ctx, f.q, c[1]); err != nil {
		t.Fatal(err)
	}
	if err := trash.TrashClip(f.ctx, f.q, c[2]); err != nil {
		t.Fatal(err)
	}
	if got := f.live(); !slices.Equal(got, []int64{c[0], c[3]}) {
		t.Fatalf("live clips = %v, want %v", got, []int64{c[0], c[3]})
	}
	if got := frameCount(); got != 2*framesPerClip {
		t.Errorf("frame count = %d, want %d", got, 2*framesPerClip)
	}
	n, err := f.q.GetNeighbourClips(f.ctx, c[0])
	if err != nil {
		t.Fatal(err)
	}
	if n.NextID != c[3] {
		t.Errorf("next clip after trashing = %d, want %d", n.NextID, c[3])
	}

	// Restoring out of order still puts each clip back where it was.
	if err := trash.RestoreClip(f.ctx, f.q, c[2]); err != nil {
		t.Fatal(err)
	}
	if got := f.live(); !slices.Equal(got, []int64{c[0], c[2], c[3]}) {
		t.Fatalf("live clips = %v, want %v", got, []int64{c[0], c[2], c[3]})
	}
	if err := trash.RestoreClip(f.ctx, f.q, c[1]); err != nil {
		t.Fatal(err)
	}
	if got := f.live(); !slices.Equal(got, c) {
		t.Fatalf("live clips = %v, want %v", got, c)
	}
	if got := frameCount(); got != 4*framesPerClip {
		t.Errorf("frame count = %d, want %d", got, 4*framesPerClip)
	}
	if got := labels(c[1]); got != 1 {
		t.Errorf("restored clip has %d labeled boxes, want 1", got)
	}

	err = trash.RestoreClip(f.ctx, f.q, c[1])
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("restoring a live clip: err = %v, want not found", err)
	}

	if err := trash.TrashClip(f.ctx, f.q, c[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := trash.TrashVideo(f.ctx, f.q, f.videoID); err != nil {
		t.Fatal(err)
	}
	err = trash.RestoreClip(f.ctx, f.q, c[1])
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("restoring a clip of a trashed video: err = %v, want not found", err)
	}
	if _, err := trash.RestoreVideo(f.ctx, f.q, f.videoID); err != nil {
		t.Fatal(err)
	}
	if got := f.live(); !slices.Equal(got, []int64{c[0], c[2], c[3]}) {
		t.Errorf("after restoring the video, live clips = %v, want the clip trashed on its own left out", got)
	}
}

type fakeObjects struct {
	mu       sync.Mutex
	prefixes []string
	keys     []string
}

func (o *fakeObjects) AbortMultipartUploads(context.Context, string) error { return nil }

func (o *fakeObjects) RemovePrefix(_ context.Context, prefix string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.prefixes = append(o.prefixes, prefix)
	return nil
}

func (o *fakeObjects) RemoveKeys(_ context.Context, keys []string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.keys = append(o.keys, keys...)
	return nil
}

func TestPurgeRemovesRowsAndObjects(t *testing.T) {
	f := setup(t, 3)
	f.label(1)
	migrator, err := rivermigrate.New(riverpgxv5.New(f.pool), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.Migrate(f.ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatal(err)
	}
	jobs, err := river.NewClient(riverpgxv5.New(f.pool), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// purge runs Purge and then the storage jobs it queued.
	purge := func(videos, clips []int64) (trash.Purged, *fakeObjects) {
		t.Helper()
		tx, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(f.ctx) }()
		p, err := trash.Purge(f.ctx, f.q.WithTx(tx), tx, jobs, videos, clips, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
		objects := &fakeObjects{}
		w := trash.NewPurgeWorker(objects)
		for _, id := range p.Jobs {
			row, err := jobs.JobGet(f.ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			var args trash.PurgeArgs
			if err := json.Unmarshal(row.EncodedArgs, &args); err != nil {
				t.Fatal(err)
			}
			if err := w.Work(f.ctx, &river.Job[trash.PurgeArgs]{JobRow: row, Args: args}); err != nil {
				t.Fatal(err)
			}
			_, _ = jobs.JobDelete(f.ctx, id)
		}
		return p, objects
	}

	// A live clip is not in the trash, so purging it does nothing.
	if p, _ := purge(nil, []int64{f.clips[1]}); p.Clips != 0 {
		t.Fatalf("purged %d live clips", p.Clips)
	}

	if err := trash.TrashClip(f.ctx, f.q, f.clips[1]); err != nil {
		t.Fatal(err)
	}
	p, objects := purge(nil, []int64{f.clips[1]})
	if p.Clips != 1 {
		t.Fatalf("purged %d clips, want 1", p.Clips)
	}
	if n := f.count(`SELECT count(*) FROM frames WHERE clip_id = $1`, f.clips[1]); n != 0 {
		t.Errorf("%d frames of the purged clip are left", n)
	}
	if n := f.count(`SELECT count(*) FROM tracks WHERE clip_id = $1`, f.clips[1]); n != 0 {
		t.Errorf("%d tracks of the purged clip are left", n)
	}
	var want []string
	for idx := int32(framesPerClip); idx < 2*framesPerClip; idx++ {
		want = append(want, storage.FrameKey(f.videoID, idx), storage.EmbeddingKey(f.videoID, idx))
	}
	slices.Sort(want)
	slices.Sort(objects.keys)
	if !slices.Equal(objects.keys, want) {
		t.Errorf("removed keys %v, want the clip's frames and embeddings %v", objects.keys, want)
	}
	if len(objects.prefixes) != 0 {
		t.Errorf("purging a clip removed prefixes %v", objects.prefixes)
	}
	if got := f.live(); !slices.Equal(got, []int64{f.clips[0], f.clips[2]}) {
		t.Errorf("live clips = %v after purging", got)
	}

	if _, err := trash.TrashVideo(f.ctx, f.q, f.videoID); err != nil {
		t.Fatal(err)
	}
	p, objects = purge([]int64{f.videoID}, nil)
	if p.Videos != 1 {
		t.Fatalf("purged %d videos, want 1", p.Videos)
	}
	if n := f.count(`SELECT count(*) FROM clips WHERE video_id = $1`, f.videoID); n != 0 {
		t.Errorf("%d clips of the purged video are left", n)
	}
	if !slices.Equal(objects.prefixes, []string{storage.VideoPrefix(f.videoID)}) {
		t.Errorf("removed prefixes %v, want the video's", objects.prefixes)
	}
}
