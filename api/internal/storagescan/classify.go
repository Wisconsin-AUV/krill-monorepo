// Package storagescan breaks down what the bucket holds by kind and video,
// finds objects no database row refers to, and deletes them on request.
package storagescan

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

type Kind string

const (
	KindSource       Kind = "source"
	KindFrame        Kind = "frame"
	KindEmbedding    Kind = "embedding"
	KindExport       Kind = "export"
	KindLabelExample Kind = "label_example"
	KindBackup       Kind = "backup"
	KindUpload       Kind = "upload"
	KindOther        Kind = "other"
	KindModel        Kind = "model"
)

var kinds = []Kind{KindSource, KindFrame, KindEmbedding, KindExport, KindLabelExample, KindBackup, KindModel, KindUpload, KindOther}

const (
	// grace leaves recent objects alone, since a label example is uploaded
	// before the row that refers to it.
	grace = time.Hour
	// staleUpload is past the day video upload URLs last, so no browser can
	// still be sending parts.
	staleUpload = 25 * time.Hour
)

// parsedKey is what a key says about its owner. Keys under a video's prefix
// that Krill doesn't write are KindOther with the video's ID.
type parsedKey struct {
	kind  Kind
	video int64
	idx   int32
	// The dataset of an export.
	dataset int64
	// The training run of a model.
	run int64
}

func parseKey(key string) parsedKey {
	parts := strings.Split(key, "/")
	switch {
	case len(parts) >= 3 && parts[0] == "videos":
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || id <= 0 || storage.VideoPrefix(id) != parts[0]+"/"+parts[1]+"/" {
			return parsedKey{kind: KindOther}
		}
		p := parsedKey{kind: KindOther, video: id}
		switch {
		case len(parts) == 3 && key == storage.VideoSourceKey(id):
			p.kind = KindSource
		case len(parts) == 4 && parts[2] == "frames":
			if idx, ok := frameIndex(parts[3], ".jpg"); ok && key == storage.FrameKey(id, idx) {
				p.kind, p.idx = KindFrame, idx
			}
		case len(parts) == 4 && parts[2] == "embeddings":
			if idx, ok := frameIndex(parts[3], ".pt"); ok && key == storage.EmbeddingKey(id, idx) {
				p.kind, p.idx = KindEmbedding, idx
			}
		}
		return p
	case len(parts) >= 3 && parts[0] == "datasets":
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || storage.DatasetPrefix(id) != parts[0]+"/"+parts[1]+"/" {
			return parsedKey{kind: KindOther}
		}
		return parsedKey{kind: KindExport, dataset: id}
	case len(parts) >= 3 && parts[0] == "training-runs":
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || storage.TrainingRunPrefix(id) != parts[0]+"/"+parts[1]+"/" {
			return parsedKey{kind: KindOther}
		}
		return parsedKey{kind: KindModel, run: id}
	case len(parts) == 4 && parts[0] == "label-types" && parts[2] == "examples":
		return parsedKey{kind: KindLabelExample}
	case strings.HasPrefix(key, storage.BackupsPrefix):
		return parsedKey{kind: KindBackup}
	}
	return parsedKey{kind: KindOther}
}

func frameIndex(name, ext string) (int32, bool) {
	n, ok := strings.CutSuffix(name, ext)
	if !ok {
		return 0, false
	}
	idx, err := strconv.ParseInt(n, 10, 32)
	if err != nil || idx < 0 {
		return 0, false
	}
	return int32(idx), true
}

// Flags on each of a video's frame indexes.
const (
	hasRow uint8 = 1 << iota
	embedded
	frameSeen
	embeddingSeen
)

type video struct {
	status     string
	frameCount int32
	sourceSeen bool
	// frames is indexed by frame index, and nil until loaded.
	frames []uint8
}

type row struct {
	status string
	seen   bool
}

// state is the database rows objects can belong to, as of load.
type state struct {
	q        *db.Queries
	now      time.Time
	videos   map[int64]*video
	datasets map[int64]*row
	// runs is seen once a run's weights are listed.
	runs     map[int64]*row
	examples map[string]*row
	backups  map[string]*row
}

func load(ctx context.Context, q *db.Queries, now time.Time) (*state, error) {
	s := &state{
		q:        q,
		now:      now,
		videos:   map[int64]*video{},
		datasets: map[int64]*row{},
		runs:     map[int64]*row{},
		examples: map[string]*row{},
		backups:  map[string]*row{},
	}
	videos, err := q.ScanVideos(ctx)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	for _, v := range videos {
		s.videos[v.ID] = &video{status: v.Status, frameCount: v.FrameCount}
	}
	datasets, err := q.ScanDatasets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list exports: %w", err)
	}
	for _, d := range datasets {
		s.datasets[d.ID] = &row{status: d.Status}
	}
	runs, err := q.ScanTrainingRuns(ctx)
	if err != nil {
		return nil, fmt.Errorf("list training runs: %w", err)
	}
	for _, r := range runs {
		s.runs[r.ID] = &row{status: r.Status}
	}
	examples, err := q.ScanLabelExampleKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list label examples: %w", err)
	}
	for _, k := range examples {
		s.examples[k] = &row{}
	}
	backups, err := q.ScanBackups(ctx)
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	for _, b := range backups {
		s.backups[b.ObjectKey] = &row{status: b.Status}
	}
	return s, nil
}

func (s *state) loadFrames(ctx context.Context, id int64, v *video) error {
	if v.frames != nil {
		return nil
	}
	rows, err := s.q.ScanVideoFrames(ctx, id)
	if err != nil {
		return fmt.Errorf("list frames of video %d: %w", id, err)
	}
	n := 0
	for _, r := range rows {
		n = max(n, int(r.Idx)+1)
	}
	v.frames = make([]uint8, n)
	for _, r := range rows {
		v.frames[r.Idx] |= hasRow
		if r.Embedded {
			v.frames[r.Idx] |= embedded
		}
	}
	return nil
}

// settled reports whether a video's frames are done changing, so a frame
// without a row is left over rather than about to get one.
func settled(status string) bool {
	return status == "ready" || status == "failed"
}

// orphan reports whether no row refers to the object at key.
func (s *state) orphan(ctx context.Context, p parsedKey, key string, modified time.Time) (bool, error) {
	if s.now.Sub(modified) < grace {
		return false, nil
	}
	if p.video != 0 {
		v, ok := s.videos[p.video]
		if !ok {
			return true, nil
		}
		if (p.kind != KindFrame && p.kind != KindEmbedding) || !settled(v.status) {
			return false, nil
		}
		if err := s.loadFrames(ctx, p.video, v); err != nil {
			return false, err
		}
		return int(p.idx) >= len(v.frames) || v.frames[p.idx]&hasRow == 0, nil
	}
	switch p.kind {
	case KindExport:
		_, ok := s.datasets[p.dataset]
		return !ok, nil
	case KindModel:
		_, ok := s.runs[p.run]
		return !ok, nil
	case KindLabelExample:
		_, ok := s.examples[key]
		return !ok, nil
	case KindBackup:
		_, ok := s.backups[key]
		return !ok, nil
	}
	return false, nil
}

// orphanUpload reports whether nothing will complete an unfinished upload.
func (s *state) orphanUpload(u storage.Upload) bool {
	age := s.now.Sub(u.Initiated)
	if age < grace {
		return false
	}
	if age > staleUpload {
		return true
	}
	p := parseKey(u.Key)
	switch {
	case p.kind == KindSource:
		v, ok := s.videos[p.video]
		return !ok || v.status != "uploading"
	case p.kind == KindExport:
		d, ok := s.datasets[p.dataset]
		return !ok || !active(d.status)
	case p.kind == KindBackup:
		b, ok := s.backups[u.Key]
		return !ok || !active(b.status)
	case p.kind == KindModel:
		r, ok := s.runs[p.run]
		return !ok || !training(r.status)
	case p.kind == KindOther && p.video == 0:
		return false
	}
	return true
}

func active(status string) bool {
	return status == "queued" || status == "running"
}

func training(status string) bool {
	return status == "queued" || status == "preparing" || status == "training" || status == "evaluating"
}
