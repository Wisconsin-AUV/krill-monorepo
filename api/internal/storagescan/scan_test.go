package storagescan

import (
	"context"
	"iter"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

type fakeStore struct {
	objects map[string]storage.Object
	uploads []storage.Upload
	removed []string
	aborted []string
}

func (f *fakeStore) List(_ context.Context, prefix string) iter.Seq2[storage.Object, error] {
	return func(yield func(storage.Object, error) bool) {
		for _, k := range slices.Sorted(maps.Keys(f.objects)) {
			if strings.HasPrefix(k, prefix) && !yield(f.objects[k], nil) {
				return
			}
		}
	}
}

func (f *fakeStore) ListUploads(_ context.Context, prefix string) iter.Seq2[storage.Upload, error] {
	return func(yield func(storage.Upload, error) bool) {
		for _, u := range f.uploads {
			if strings.HasPrefix(u.Key, prefix) && !yield(u, nil) {
				return
			}
		}
	}
}

func (f *fakeStore) Stat(_ context.Context, key string) (storage.Object, bool, error) {
	o, ok := f.objects[key]
	return o, ok, nil
}

func (f *fakeStore) RemoveKeys(_ context.Context, keys []string) error {
	for _, k := range keys {
		delete(f.objects, k)
	}
	f.removed = append(f.removed, keys...)
	return nil
}

func (f *fakeStore) AbortUpload(_ context.Context, key, uploadID string) error {
	f.aborted = append(f.aborted, key+"#"+uploadID)
	return nil
}

func (f *fakeStore) put(key string, size int64, modified time.Time) {
	f.objects[key] = storage.Object{Key: key, Size: size, Modified: modified}
}

func testTx(t *testing.T) (context.Context, pgx.Tx) {
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
	// Repeatable read keeps rows other packages' tests commit meanwhile out
	// of both scans, so counts can be compared.
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return ctx, tx
}

func exec(t *testing.T, ctx context.Context, tx pgx.Tx, sql string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := tx.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return id
}

func orphanKeys(r *report) []string {
	var out []string
	for _, o := range r.orphans {
		k := o.Key
		if o.UploadID != "" {
			k += "#" + o.UploadID
		}
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// A cleanup deletes what a scan found only if it still has no row and hasn't
// changed, so nothing made or claimed since the scan is lost.
func TestScanAndCleanUp(t *testing.T) {
	ctx, tx := testTx(t)
	q := db.New(tx)
	now := time.Now()
	old, recent := now.Add(-2*time.Hour), now.Add(-time.Minute)

	baseline, err := load(ctx, q, now)
	if err != nil {
		t.Fatal(err)
	}
	before, err := scan(ctx, &fakeStore{objects: map[string]storage.Object{}}, baseline)
	if err != nil {
		t.Fatal(err)
	}

	video := func(status string) int64 {
		return exec(t, ctx, tx, `INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', $1) RETURNING id`, status)
	}
	ready := video("ready")
	clip := exec(t, ctx, tx, `INSERT INTO clips (video_id, idx, start_frame, frame_count, embedded_at) VALUES ($1, 0, 0, 2, now()) RETURNING id`, ready)
	for idx := range 2 {
		exec(t, ctx, tx, `INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, $3, 0) RETURNING id`, ready, clip, idx)
	}
	deleted := video("ready")
	exec(t, ctx, tx, `DELETE FROM videos WHERE id = $1 RETURNING id`, deleted)
	processing := video("processing")
	recentVideo := video("ready")
	uploading := video("uploading")
	dataset := exec(t, ctx, tx, `INSERT INTO datasets (name, status, options) VALUES ('d', 'ready', '{}') RETURNING id`)
	goneDataset := exec(t, ctx, tx, `INSERT INTO datasets (name, options) VALUES ('d', '{}') RETURNING id`)
	exec(t, ctx, tx, `DELETE FROM datasets WHERE id = $1 RETURNING id`, goneDataset)
	labelType := exec(t, ctx, tx, `INSERT INTO label_types (name, color, position) VALUES ('scan-test', '#fff', 999) RETURNING id`)
	example := storage.LabelExamplesPrefix(labelType) + "kept"
	exec(t, ctx, tx, `INSERT INTO label_examples (label_type_id, object_key) VALUES ($1, $2) RETURNING id`, labelType, example)
	abandoned := storage.LabelExamplesPrefix(labelType) + "abandoned"

	store := &fakeStore{objects: map[string]storage.Object{}}
	store.put(storage.VideoSourceKey(ready), 1000, old)
	store.put(storage.FrameKey(ready, 0), 10, old)
	store.put(storage.FrameKey(ready, 1), 10, old)
	store.put(storage.FrameKey(ready, 2), 10, old) // of a deleted clip
	store.put(storage.EmbeddingKey(ready, 0), 100, old)
	store.put(storage.EmbeddingKey(ready, 3), 100, old)
	store.put(storage.VideoSourceKey(deleted), 2000, old)
	store.put(storage.FrameKey(deleted, 0), 20, old)
	store.put(storage.FrameKey(processing, 0), 30, old)
	store.put(storage.FrameKey(recentVideo, 5), 40, recent)
	store.put(storage.DatasetKey(dataset), 500, old)
	store.put(storage.DatasetKey(goneDataset), 600, old)
	store.put(example, 5, old)
	store.put(abandoned, 6, old)
	store.put(storage.BackupKey("stray.dump"), 700, old)
	store.put("misc/readme.txt", 1, old)
	store.uploads = []storage.Upload{
		{Key: storage.VideoSourceKey(ready), ID: "stale", Size: 50, Initiated: now.Add(-30 * time.Hour)},
		{Key: storage.VideoSourceKey(uploading), ID: "live", Size: 60, Initiated: old},
	}

	s, err := load(ctx, q, now)
	if err != nil {
		t.Fatal(err)
	}
	r, err := scan(ctx, store, s)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		storage.BackupKey("stray.dump"),
		storage.DatasetKey(goneDataset),
		abandoned,
		storage.EmbeddingKey(ready, 3),
		storage.FrameKey(ready, 2),
		storage.VideoSourceKey(ready) + "#stale",
		storage.FrameKey(deleted, 0),
		storage.VideoSourceKey(deleted),
	}
	slices.Sort(want)
	if got := orphanKeys(r); !slices.Equal(got, want) {
		t.Errorf("orphans:\n got %q\nwant %q", got, want)
	}
	v := r.videos[ready]
	if v.Frames != 3 || v.FrameBytes != 30 || v.Embeddings != 2 || v.SourceBytes != 1000 || v.OrphanBytes != 110 {
		t.Errorf("ready video usage = %+v", v)
	}
	// Only the processing and recent videos lack a source, since the upload
	// isn't done for the uploading one. The clip's second frame has no
	// embedding.
	for kind, n := range map[Kind]int64{KindSource: 2, KindEmbedding: 1, KindFrame: 0, KindExport: 0, KindLabelExample: 0} {
		if d := r.categories[kind].Missing - before.categories[kind].Missing; d != n {
			t.Errorf("%s missing went up by %d, want %d", kind, d, n)
		}
	}

	scanID, err := q.CreateStorageScan(ctx, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range r.orphans {
		r.orphans[i].ScanID = scanID
	}
	if _, err := q.InsertStorageOrphans(ctx, r.orphans); err != nil {
		t.Fatal(err)
	}

	// Since the scan, the abandoned example was added, and the stray frame
	// was written again.
	exec(t, ctx, tx, `INSERT INTO label_examples (label_type_id, object_key) VALUES ($1, $2) RETURNING id`, labelType, abandoned)
	store.put(storage.FrameKey(ready, 2), 10, now)

	res, err := cleanUp(ctx, store, q, scanID, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if res.skipped != 2 || res.deleted != 6 || res.bytes != 2000+20+600+100+700+50 {
		t.Errorf("cleanup = %+v", res)
	}
	for _, k := range []string{abandoned, storage.FrameKey(ready, 2), storage.FrameKey(processing, 0), example, storage.VideoSourceKey(ready)} {
		if _, ok := store.objects[k]; !ok {
			t.Errorf("%s was deleted", k)
		}
	}
	if !slices.Equal(store.aborted, []string{storage.VideoSourceKey(ready) + "#stale"}) {
		t.Errorf("aborted %q", store.aborted)
	}
}

func TestParseKey(t *testing.T) {
	tests := []struct {
		key  string
		want parsedKey
	}{
		{"videos/12/source", parsedKey{kind: KindSource, video: 12}},
		{"videos/12/frames/000007.jpg", parsedKey{kind: KindFrame, video: 12, idx: 7}},
		{"videos/12/embeddings/000007.pt", parsedKey{kind: KindEmbedding, video: 12, idx: 7}},
		// Not a key Krill writes, so it can't be checked against a frame row.
		{"videos/12/frames/7.jpg", parsedKey{kind: KindOther, video: 12}},
		{"videos/012/source", parsedKey{kind: KindOther}},
		{"datasets/3/dataset.zip", parsedKey{kind: KindExport, dataset: 3}},
		{"label-types/4/examples/ab12", parsedKey{kind: KindLabelExample}},
		{"backups/krill.dump", parsedKey{kind: KindBackup}},
		{"videos", parsedKey{kind: KindOther}},
	}
	for _, tt := range tests {
		if got := parseKey(tt.key); got != tt.want {
			t.Errorf("parseKey(%q) = %+v, want %+v", tt.key, got, tt.want)
		}
	}
}
