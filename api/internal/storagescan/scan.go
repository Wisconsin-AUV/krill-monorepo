package storagescan

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

// Queue runs scans and cleanups one at a time.
const Queue = "storage"

// maxMissingSamples is how many missing keys of each kind a scan keeps.
const maxMissingSamples = 5

type objectStore interface {
	List(ctx context.Context, prefix string) iter.Seq2[storage.Object, error]
	ListUploads(ctx context.Context, prefix string) iter.Seq2[storage.Upload, error]
	Stat(ctx context.Context, key string) (storage.Object, bool, error)
	RemoveKeys(ctx context.Context, keys []string) error
	AbortUpload(ctx context.Context, key, uploadID string) error
}

type category struct {
	Kind           Kind     `json:"kind"`
	Bytes          int64    `json:"bytes"`
	Objects        int64    `json:"objects"`
	OrphanBytes    int64    `json:"orphan_bytes"`
	OrphanObjects  int64    `json:"orphan_objects"`
	Missing        int64    `json:"missing"`
	MissingSamples []string `json:"missing_samples,omitempty"`
}

func (c *category) miss(key string) {
	c.Missing++
	if len(c.MissingSamples) < maxMissingSamples {
		c.MissingSamples = append(c.MissingSamples, key)
	}
}

type report struct {
	categories map[Kind]*category
	videos     map[int64]*db.InsertStorageScanVideosParams
	orphans    []db.InsertStorageOrphansParams
}

func (r *report) video(id int64) *db.InsertStorageScanVideosParams {
	v, ok := r.videos[id]
	if !ok {
		v = &db.InsertStorageScanVideosParams{VideoID: id}
		r.videos[id] = v
	}
	return v
}

func (r *report) orphan(key, uploadID string, kind Kind, size int64, modified time.Time) {
	c := r.categories[kind]
	c.OrphanBytes += size
	c.OrphanObjects++
	r.orphans = append(r.orphans, db.InsertStorageOrphansParams{
		Key:        key,
		UploadID:   uploadID,
		Kind:       string(kind),
		SizeBytes:  size,
		ModifiedAt: pgtype.Timestamptz{Time: modified, Valid: true},
	})
}

// scan lists the whole bucket against s.
func scan(ctx context.Context, store objectStore, s *state) (*report, error) {
	r := &report{categories: map[Kind]*category{}, videos: map[int64]*db.InsertStorageScanVideosParams{}}
	for _, k := range kinds {
		r.categories[k] = &category{Kind: k}
	}
	for o, err := range store.List(ctx, "") {
		if err != nil {
			return nil, err
		}
		p := parseKey(o.Key)
		c := r.categories[p.kind]
		c.Bytes += o.Size
		c.Objects++
		orphan, err := s.orphan(ctx, p, o.Key, o.Modified)
		if err != nil {
			return nil, err
		}
		if orphan {
			r.orphan(o.Key, "", p.kind, o.Size, o.Modified)
		}
		if err := s.markSeen(ctx, p, o.Key); err != nil {
			return nil, err
		}
		if p.video == 0 {
			continue
		}
		v := r.video(p.video)
		switch p.kind {
		case KindSource:
			v.SourceBytes += o.Size
		case KindFrame:
			v.FrameBytes += o.Size
			v.Frames++
		case KindEmbedding:
			v.EmbeddingBytes += o.Size
			v.Embeddings++
		}
		if orphan {
			v.OrphanBytes += o.Size
		}
	}
	for u, err := range store.ListUploads(ctx, "") {
		if err != nil {
			return nil, err
		}
		c := r.categories[KindUpload]
		c.Bytes += u.Size
		c.Objects++
		if s.orphanUpload(u) {
			r.orphan(u.Key, u.ID, KindUpload, u.Size, u.Initiated)
		}
	}
	if err := s.missing(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *state) markSeen(ctx context.Context, p parsedKey, key string) error {
	if p.video != 0 {
		v, ok := s.videos[p.video]
		if !ok {
			return nil
		}
		switch p.kind {
		case KindSource:
			v.sourceSeen = true
		case KindFrame, KindEmbedding:
			// missing only checks ready videos.
			if v.status != "ready" {
				return nil
			}
			if err := s.loadFrames(ctx, p.video, v); err != nil {
				return err
			}
			if int(p.idx) < len(v.frames) {
				if p.kind == KindFrame {
					v.frames[p.idx] |= frameSeen
				} else {
					v.frames[p.idx] |= embeddingSeen
				}
			}
		}
		return nil
	}
	var r *row
	switch p.kind {
	case KindExport:
		r = s.datasets[p.dataset]
	case KindLabelExample:
		r = s.examples[key]
	case KindBackup:
		r = s.backups[key]
	case KindModel:
		if key == storage.TrainingWeightsKey(p.run) {
			r = s.runs[p.run]
		}
	}
	if r != nil {
		r.seen = true
	}
	return nil
}

// missing counts rows whose object the listing didn't include.
func (s *state) missing(ctx context.Context, r *report) error {
	ids := make([]int64, 0, len(s.videos))
	for id := range s.videos {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		v := s.videos[id]
		if v.status != "uploading" && !v.sourceSeen {
			r.categories[KindSource].miss(storage.VideoSourceKey(id))
		}
		if v.status != "ready" {
			continue
		}
		// Loads the frames of a video with none listed, whose rows are all
		// missing.
		if err := s.loadFrames(ctx, id, v); err != nil {
			return err
		}
		for idx, f := range v.frames {
			if f&hasRow == 0 {
				continue
			}
			if f&frameSeen == 0 {
				r.categories[KindFrame].miss(storage.FrameKey(id, int32(idx))) //nolint:gosec // frame indexes fit in int32
			}
			if f&embedded != 0 && f&embeddingSeen == 0 {
				r.categories[KindEmbedding].miss(storage.EmbeddingKey(id, int32(idx))) //nolint:gosec // frame indexes fit in int32
			}
		}
		// Frames of other videos aren't needed again.
		v.frames = nil
	}
	for id, d := range s.datasets {
		if d.status == "ready" && !d.seen {
			r.categories[KindExport].miss(storage.DatasetKey(id))
		}
	}
	for key, e := range s.examples {
		if !e.seen {
			r.categories[KindLabelExample].miss(key)
		}
	}
	for id, run := range s.runs {
		if run.status == "succeeded" && !run.seen {
			r.categories[KindModel].miss(storage.TrainingWeightsKey(id))
		}
	}
	for key, b := range s.backups {
		if b.status == "done" && !b.seen {
			r.categories[KindBackup].miss(key)
		}
	}
	return nil
}

type ScanArgs struct {
	ScanID int64 `json:"scan_id"`
}

func (ScanArgs) Kind() string { return "storage_scan" }

func (ScanArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: Queue, MaxAttempts: 1}
}

type ScanWorker struct {
	river.WorkerDefaults[ScanArgs]
	pool  *pgxpool.Pool
	store objectStore
}

func NewScanWorker(pool *pgxpool.Pool, store *storage.Store) *ScanWorker {
	return &ScanWorker{pool: pool, store: store}
}

func (w *ScanWorker) Timeout(*river.Job[ScanArgs]) time.Duration { return 6 * time.Hour }

func (w *ScanWorker) Work(ctx context.Context, job *river.Job[ScanArgs]) error {
	id := job.Args.ScanID
	q := db.New(w.pool)
	n, err := q.StartStorageScan(ctx, id)
	if err != nil {
		return fmt.Errorf("start scan %d: %w", id, err)
	}
	if n == 0 {
		return nil
	}
	start := time.Now()
	err = w.run(ctx, q, id)
	if err == nil {
		slog.Info("storage scan finished", "scan_id", id, "took", time.Since(start).Round(time.Millisecond))
		return nil
	}
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if ferr := q.FailStorageScan(failCtx, db.FailStorageScanParams{ID: id, Error: err.Error()}); ferr != nil {
		return fmt.Errorf("%w; record failure: %w", err, ferr)
	}
	return err
}

func (w *ScanWorker) run(ctx context.Context, q *db.Queries, id int64) error {
	s, err := load(ctx, q, time.Now())
	if err != nil {
		return err
	}
	r, err := scan(ctx, w.store, s)
	if err != nil {
		return err
	}
	cats := make([]*category, len(kinds))
	for i, k := range kinds {
		cats[i] = r.categories[k]
	}
	catsJSON, err := json.Marshal(cats)
	if err != nil {
		return fmt.Errorf("encode categories: %w", err)
	}
	videos := make([]db.InsertStorageScanVideosParams, 0, len(r.videos))
	for _, v := range r.videos {
		v.ScanID = id
		videos = append(videos, *v)
	}
	for i := range r.orphans {
		r.orphans[i].ScanID = id
	}

	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := q.WithTx(tx)
	if _, err := qtx.InsertStorageScanVideos(ctx, videos); err != nil {
		return fmt.Errorf("save videos: %w", err)
	}
	if _, err := qtx.InsertStorageOrphans(ctx, r.orphans); err != nil {
		return fmt.Errorf("save orphans: %w", err)
	}
	if err := qtx.FinishStorageScan(ctx, db.FinishStorageScanParams{ID: id, Categories: catsJSON}); err != nil {
		return fmt.Errorf("finish scan: %w", err)
	}
	if err := qtx.DeleteOldStorageScans(ctx); err != nil {
		return fmt.Errorf("delete old scans: %w", err)
	}
	return tx.Commit(ctx)
}
