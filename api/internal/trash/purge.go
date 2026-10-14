package trash

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
)

// ErrBusy is returned when a trashed video is still being ingested, since
// the ingest would keep writing frames after its objects were removed.
var ErrBusy = errors.New("video is still processing")

// JobKind is how purges are recorded in content_jobs.
const JobKind = "purge"

// Purged is what Purge deleted. Jobs remove the objects from storage.
type Purged struct {
	Videos int
	Clips  int
	Jobs   []int64
}

// Purge deletes trashed videos and clips in the caller's transaction and
// queues jobs that remove their objects. Items that are no longer in the
// trash are skipped. A video still processing fails with ErrBusy unless
// skipBusy is set.
func Purge(ctx context.Context, q *db.Queries, tx pgx.Tx, jobs *river.Client[pgx.Tx], videoIDs, clipIDs []int64, skipBusy bool, details any) (Purged, error) {
	var out Purged
	var args []river.InsertManyParams
	for _, id := range videoIDs {
		v, err := q.LockTrashedVideo(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return Purged{}, rpc.Internal(err, "lock video")
		}
		if v.Status == "queued" || v.Status == "processing" {
			if skipBusy {
				continue
			}
			return Purged{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s: %w", v.Name, ErrBusy))
		}
		if _, err := q.DeleteVideo(ctx, id); err != nil {
			return Purged{}, rpc.Internal(err, "delete video")
		}
		if err := record(ctx, q, audit.VideoPurge, audit.TargetVideo, id, v.Name, details); err != nil {
			return Purged{}, err
		}
		args = append(args, river.InsertManyParams{Args: PurgeArgs{VideoID: id, Whole: true}})
		out.Videos++
	}
	for _, id := range clipIDs {
		name, err := audit.ClipName(ctx, q, id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return Purged{}, rpc.Internal(err, "name clip")
		}
		c, err := q.DeleteClip(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return Purged{}, rpc.Internal(err, "delete clip")
		}
		if err := record(ctx, q, audit.ClipPurge, audit.TargetClip, id, name, details); err != nil {
			return Purged{}, err
		}
		args = append(args, river.InsertManyParams{Args: PurgeArgs{
			VideoID: c.VideoID, StartFrame: c.StartFrame, FrameCount: c.FrameCount,
		}})
		out.Clips++
	}
	if len(args) == 0 {
		return out, nil
	}
	res, err := jobs.InsertManyTx(ctx, tx, args)
	if err != nil {
		return Purged{}, rpc.Internal(err, "queue storage cleanup")
	}
	for _, r := range res {
		out.Jobs = append(out.Jobs, r.Job.ID)
	}
	return out, nil
}

// Summary reads like "2 videos, 1 clip".
func (p Purged) Summary() string {
	return Count(p.Videos, p.Clips)
}

func Count(videos, clips int) string {
	var parts []string
	if videos > 0 {
		parts = append(parts, plural(videos, "video"))
	}
	if clips > 0 {
		parts = append(parts, plural(clips, "clip"))
	}
	switch len(parts) {
	case 0:
		return "Nothing"
	case 1:
		return parts[0]
	}
	return parts[0] + ", " + parts[1]
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// PurgeArgs removes a purged video's or clip's objects from storage.
type PurgeArgs struct {
	VideoID int64 `json:"video_id"`
	// Whole removes every object of the video, otherwise only the frames and
	// embeddings of FrameCount frames from StartFrame.
	Whole      bool  `json:"whole,omitempty"`
	StartFrame int32 `json:"start_frame,omitempty"`
	FrameCount int32 `json:"frame_count,omitempty"`
}

func (PurgeArgs) Kind() string { return "purge_objects" }

// Objects is the part of storage purging needs.
type Objects interface {
	AbortMultipartUploads(ctx context.Context, key string) error
	RemovePrefix(ctx context.Context, prefix string) error
	RemoveKeys(ctx context.Context, keys []string) error
}

type PurgeWorker struct {
	river.WorkerDefaults[PurgeArgs]
	q     *db.Queries
	store Objects
}

func NewPurgeWorker(q *db.Queries, store Objects) *PurgeWorker {
	return &PurgeWorker{q: q, store: store}
}

func (w *PurgeWorker) Timeout(*river.Job[PurgeArgs]) time.Duration { return time.Hour }

func (w *PurgeWorker) Work(ctx context.Context, job *river.Job[PurgeArgs]) error {
	a := job.Args
	if a.Whole {
		if err := w.store.AbortMultipartUploads(ctx, storage.VideoSourceKey(a.VideoID)); err != nil {
			slog.WarnContext(ctx, "abort video upload", "video_id", a.VideoID, "err", err)
		}
		return w.store.RemovePrefix(ctx, storage.VideoPrefix(a.VideoID))
	}
	// A clip's frames share keys with whatever a re-extract of its video
	// writes at the same indexes, so wait out an ingest and keep any frame
	// that has a row again.
	status, err := w.q.GetVideoStatus(ctx, a.VideoID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return fmt.Errorf("load video %d: %w", a.VideoID, err)
	case status == "queued" || status == "processing":
		return river.JobSnooze(time.Minute)
	}
	live, err := w.q.ListFrameIdxInRange(ctx, db.ListFrameIdxInRangeParams{
		VideoID: a.VideoID, StartIdx: a.StartFrame, EndIdx: a.StartFrame + a.FrameCount,
	})
	if err != nil {
		return fmt.Errorf("list frames of video %d: %w", a.VideoID, err)
	}
	keep := make(map[int32]bool, len(live))
	for _, idx := range live {
		keep[idx] = true
	}
	keys := make([]string, 0, 2*a.FrameCount)
	for idx := a.StartFrame; idx < a.StartFrame+a.FrameCount; idx++ {
		if !keep[idx] {
			keys = append(keys, storage.FrameKey(a.VideoID, idx), storage.EmbeddingKey(a.VideoID, idx))
		}
	}
	return w.store.RemoveKeys(ctx, keys)
}
