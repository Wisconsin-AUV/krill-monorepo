// Package content backs the admin panel's content pages: the trash and bulk
// actions on videos and clips.
package content

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/compute"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/trash"
	"github.com/wauv/krill/api/internal/video"
)

const (
	// maxSelection bounds one bulk request.
	maxSelection = 1000
	// jobsWindow is how far back ListContentJobs looks. River forgets
	// finished jobs after a day, so older progress could not be read.
	jobsWindow = 24 * time.Hour
	// riverListMax is the most jobs River lists at once.
	riverListMax = 10_000
)

type Service struct {
	krillv1connect.UnimplementedContentServiceHandler
	pool    *pgxpool.Pool
	q       *db.Queries
	store   *storage.Store
	present *video.Presenter
	jobs    *river.Client[pgx.Tx]
	nodes   *compute.Registry
	// retention is the live trash retention in days. 0 keeps items.
	retention func() int
}

func NewService(pool *pgxpool.Pool, store *storage.Store, jobs *river.Client[pgx.Tx], nodes *compute.Registry, retention func() int) *Service {
	return &Service{
		pool:      pool,
		q:         db.New(pool),
		store:     store,
		present:   video.NewPresenter(store),
		jobs:      jobs,
		nodes:     nodes,
		retention: retention,
	}
}

// currentModel is the SAM model of the most recently seen GPU worker.
// Embeddings from any other model are stale.
func (s *Service) currentModel() string {
	for _, w := range s.nodes.Workers() {
		if w.Model != "" {
			return w.Model
		}
	}
	return ""
}

var kindToProto = map[string]krillv1.ContentJobKind{
	kindReextract: krillv1.ContentJobKind_CONTENT_JOB_KIND_REEXTRACT,
	kindEmbed:     krillv1.ContentJobKind_CONTENT_JOB_KIND_EMBED,
	trash.JobKind: krillv1.ContentJobKind_CONTENT_JOB_KIND_PURGE,
}

const (
	kindReextract = "reextract"
	kindEmbed     = "embed"
)

func (s *Service) createJob(ctx context.Context, q *db.Queries, kind, summary string, ids []int64) (*krillv1.ContentJob, error) {
	sess, signedIn := auth.SessionFrom(ctx)
	j, err := q.CreateContentJob(ctx, db.CreateContentJobParams{
		Kind:        kind,
		Summary:     summary,
		RiverJobIds: ids,
		CreatedBy:   pgtype.UUID{Bytes: sess.User.ID, Valid: signedIn},
	})
	if err != nil {
		return nil, rpc.Internal(err, "record job")
	}
	out := &krillv1.ContentJob{
		Id:        j.ID,
		Kind:      kindToProto[kind],
		Summary:   summary,
		CreatedAt: timestamppb.New(j.CreatedAt.Time),
		Total:     int32(len(ids)), //nolint:gosec // bounded by maxSelection
	}
	if signedIn {
		u := sess.User
		out.CreatedBy = auth.PublicProfile(u.ID, u.Name, u.Username, u.Role, u.CreatedAt)
	}
	return out, nil
}

func (s *Service) ListContentJobs(ctx context.Context, _ *krillv1.ListContentJobsRequest) (*krillv1.ListContentJobsResponse, error) {
	rows, err := s.q.ListContentJobs(ctx, pgtype.Timestamptz{Time: time.Now().Add(-jobsWindow), Valid: true})
	if err != nil {
		return nil, rpc.Internal(err, "list jobs")
	}
	var ids []int64
	for _, r := range rows {
		ids = append(ids, r.RiverJobIds...)
	}
	states, err := s.jobStates(ctx, ids)
	if err != nil {
		return nil, rpc.Internal(err, "load job states")
	}
	out := &krillv1.ListContentJobsResponse{Jobs: make([]*krillv1.ContentJob, len(rows))}
	for i, r := range rows {
		j := &krillv1.ContentJob{
			Id:        r.ID,
			Kind:      kindToProto[r.Kind],
			Summary:   r.Summary,
			CreatedAt: timestamppb.New(r.CreatedAt.Time),
			Total:     int32(len(r.RiverJobIds)), //nolint:gosec // bounded by maxSelection
		}
		if r.CreatorID.Valid {
			j.CreatedBy = auth.PublicProfile(r.CreatorID.Bytes, r.CreatorName.String, r.CreatorUsername.String, r.CreatorRole.String, r.CreatorCreatedAt)
		}
		for _, id := range r.RiverJobIds {
			state, ok := states[id]
			switch {
			// River deletes finished jobs after a while.
			case !ok, state == rivertype.JobStateCompleted:
				j.Done++
			case state == rivertype.JobStateDiscarded, state == rivertype.JobStateCancelled:
				j.Failed++
			case state == rivertype.JobStateRunning:
				j.Running++
			}
		}
		out.Jobs[i] = j
	}
	return out, nil
}

func (s *Service) jobStates(ctx context.Context, ids []int64) (map[int64]rivertype.JobState, error) {
	out := make(map[int64]rivertype.JobState, len(ids))
	for start := 0; start < len(ids); start += riverListMax {
		chunk := ids[start:min(start+riverListMax, len(ids))]
		res, err := s.jobs.JobList(ctx, river.NewJobListParams().IDs(chunk...).First(len(chunk)))
		if err != nil {
			return nil, err
		}
		for _, j := range res.Jobs {
			out[j.ID] = j.State
		}
	}
	return out, nil
}

// checkSelection rejects empty and oversized selections.
func checkSelection(n int) error {
	if n == 0 {
		return rpc.Invalid("select at least one item")
	}
	if n > maxSelection {
		return rpc.Invalid("select at most %d items", maxSelection)
	}
	return nil
}

// names reads like "a.mp4 and b.mp4" or "5 videos".
func names(items []string, noun string) string {
	switch len(items) {
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	}
	return fmt.Sprintf("%d %s", len(items), noun)
}

func unique(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func ts(t pgtype.Timestamptz) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}
