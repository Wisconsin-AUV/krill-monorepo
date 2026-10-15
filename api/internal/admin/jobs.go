package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/export"
	"github.com/wauv/krill/api/internal/ingest"
	"github.com/wauv/krill/api/internal/jobmeta"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/tracker"
)

const (
	defaultJobPage = 50
	maxJobPage     = 200
	minPriority    = 1
	maxPriority    = 4
)

var jobStates = map[rivertype.JobState]krillv1.JobState{
	rivertype.JobStateAvailable: krillv1.JobState_JOB_STATE_QUEUED,
	rivertype.JobStateScheduled: krillv1.JobState_JOB_STATE_QUEUED,
	rivertype.JobStatePending:   krillv1.JobState_JOB_STATE_QUEUED,
	rivertype.JobStateRunning:   krillv1.JobState_JOB_STATE_RUNNING,
	rivertype.JobStateRetryable: krillv1.JobState_JOB_STATE_RETRYABLE,
	rivertype.JobStateDiscarded: krillv1.JobState_JOB_STATE_FAILED,
	rivertype.JobStateCancelled: krillv1.JobState_JOB_STATE_CANCELLED,
	rivertype.JobStateCompleted: krillv1.JobState_JOB_STATE_COMPLETED,
}

// riverStates returns the River states behind each state.
func riverStates(states []krillv1.JobState) ([]rivertype.JobState, error) {
	var out []rivertype.JobState
	for _, want := range states {
		found := false
		for rs, s := range jobStates {
			if s == want {
				out = append(out, rs)
				found = true
			}
		}
		if !found {
			return nil, rpc.Invalid("unknown job state %d", want)
		}
	}
	return out, nil
}

func (s *ComputeService) ListJobs(ctx context.Context, req *krillv1.ListJobsRequest) (*krillv1.ListJobsResponse, error) {
	size := int(req.GetPageSize())
	switch {
	case size < 0:
		return nil, rpc.Invalid("page size must not be negative")
	case size == 0:
		size = defaultJobPage
	case size > maxJobPage:
		size = maxJobPage
	}
	params := river.NewJobListParams().OrderBy(river.JobListOrderByID, river.SortOrderDesc).First(size + 1)
	if len(req.GetKinds()) > 0 {
		params = params.Kinds(req.GetKinds()...)
	}
	if len(req.GetStates()) > 0 {
		states, err := riverStates(req.GetStates())
		if err != nil {
			return nil, err
		}
		params = params.States(states...)
	}
	if t := req.GetPageToken(); t != "" {
		before, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return nil, rpc.Invalid("bad page token")
		}
		params = params.Where("id < @before", river.NamedArgs{"before": before})
	}
	res, err := s.jobs.JobList(ctx, params)
	if err != nil {
		return nil, rpc.Internal(err, "list jobs")
	}
	rows := res.Jobs
	out := &krillv1.ListJobsResponse{}
	if len(rows) > size {
		rows = rows[:size]
		out.NextPageToken = strconv.FormatInt(rows[len(rows)-1].ID, 10)
	}
	if out.Jobs, err = s.describeJobs(ctx, s.q, rows); err != nil {
		return nil, err
	}
	if out.Counts, err = s.countJobs(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ComputeService) countJobs(ctx context.Context) ([]*krillv1.JobCount, error) {
	// River's client has no counts, and its table isn't in the schema sqlc
	// reads.
	rows, err := s.pool.Query(ctx, `SELECT kind, state, count(*) FROM river_job GROUP BY kind, state ORDER BY kind, state`)
	if err != nil {
		return nil, rpc.Internal(err, "count jobs")
	}
	defer rows.Close()
	merged := map[string]map[krillv1.JobState]int32{}
	var kinds []string
	for rows.Next() {
		var kind, state string
		var n int32
		if err := rows.Scan(&kind, &state, &n); err != nil {
			return nil, rpc.Internal(err, "count jobs")
		}
		if merged[kind] == nil {
			merged[kind] = map[krillv1.JobState]int32{}
			kinds = append(kinds, kind)
		}
		merged[kind][jobStates[rivertype.JobState(state)]] += n
	}
	if err := rows.Err(); err != nil {
		return nil, rpc.Internal(err, "count jobs")
	}
	var out []*krillv1.JobCount
	for _, kind := range kinds {
		states := make([]krillv1.JobState, 0, len(merged[kind]))
		for st := range merged[kind] {
			states = append(states, st)
		}
		slices.Sort(states)
		for _, st := range states {
			out = append(out, &krillv1.JobCount{Kind: kind, State: st, Count: merged[kind][st]})
		}
	}
	return out, nil
}

// gpuBacklog counts GPU jobs waiting in River or for a worker to take their
// task, by whether someone is waiting on them.
func gpuBacklog(ctx context.Context, pool db.DBTX, dispatcher *tracker.Dispatcher) (interactive, background int, err error) {
	err = pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE queue = $1), count(*) FILTER (WHERE queue = $2)
		FROM river_job WHERE state IN ('available', 'scheduled', 'pending') AND queue IN ($1, $2)`,
		annotation.TrackQueue, ingest.EmbedQueue,
	).Scan(&interactive, &background)
	if err != nil {
		return 0, 0, fmt.Errorf("count GPU jobs: %w", err)
	}
	waitingInteractive, waitingBackground := dispatcher.Queue()
	return interactive + waitingInteractive, background + waitingBackground, nil
}

// jobArgs are the arguments that name a job's target. Jobs of new kinds get
// a name as long as they use the same keys.
type jobArgs struct {
	VideoID   int64 `json:"video_id"`
	ClipID    int64 `json:"clip_id"`
	FrameID   int64 `json:"frame_id"`
	DatasetID int64 `json:"dataset_id"`
	TrackID   int64 `json:"track_id"`
}

func parseArgs(job *rivertype.JobRow) jobArgs {
	var a jobArgs
	_ = json.Unmarshal(job.EncodedArgs, &a)
	return a
}

func (s *ComputeService) describeJobs(ctx context.Context, q *db.Queries, rows []*rivertype.JobRow) ([]*krillv1.Job, error) {
	var videoIDs, clipIDs, frameIDs, datasetIDs []int64
	var starterIDs []uuid.UUID
	args := make([]jobArgs, len(rows))
	for i, r := range rows {
		a := parseArgs(r)
		args[i] = a
		switch {
		case a.FrameID != 0:
			frameIDs = append(frameIDs, a.FrameID)
		case a.ClipID != 0:
			clipIDs = append(clipIDs, a.ClipID)
		case a.VideoID != 0:
			videoIDs = append(videoIDs, a.VideoID)
		case a.DatasetID != 0:
			datasetIDs = append(datasetIDs, a.DatasetID)
		}
		if id := jobmeta.Starter(r.Metadata); id != uuid.Nil {
			starterIDs = append(starterIDs, id)
		}
	}
	names := map[string]string{}
	key := func(kind string, id int64) string { return kind + ":" + strconv.FormatInt(id, 10) }
	if len(videoIDs) > 0 {
		vs, err := q.ListJobVideos(ctx, videoIDs)
		if err != nil {
			return nil, rpc.Internal(err, "name videos")
		}
		for _, v := range vs {
			names[key("video", v.ID)] = v.Name
		}
	}
	if len(clipIDs) > 0 {
		cs, err := q.ListJobClips(ctx, clipIDs)
		if err != nil {
			return nil, rpc.Internal(err, "name clips")
		}
		for _, c := range cs {
			names[key("clip", c.ID)] = fmt.Sprintf("%s · Clip %d", c.VideoName, c.Idx+1)
		}
	}
	if len(frameIDs) > 0 {
		fs, err := q.ListJobFrames(ctx, frameIDs)
		if err != nil {
			return nil, rpc.Internal(err, "name frames")
		}
		for _, f := range fs {
			names[key("frame", f.ID)] = fmt.Sprintf("%s · Frame %d", f.VideoName, f.Idx+1)
		}
	}
	if len(datasetIDs) > 0 {
		ds, err := q.ListJobDatasets(ctx, datasetIDs)
		if err != nil {
			return nil, rpc.Internal(err, "name exports")
		}
		for _, d := range ds {
			names[key("dataset", d.ID)] = d.Name
		}
	}
	starters := map[uuid.UUID]*krillv1.Profile{}
	if len(starterIDs) > 0 {
		us, err := q.ListJobStarters(ctx, starterIDs)
		if err != nil {
			return nil, rpc.Internal(err, "load starters")
		}
		for _, u := range us {
			starters[u.ID] = auth.PublicProfile(u.ID, u.Name, u.Username, u.Role, u.CreatedAt)
		}
	}

	tasks := s.dispatcher.Jobs()
	workers := map[int64][2]string{}
	for _, w := range s.nodes.Workers() {
		if w.Online && w.TaskID != 0 {
			workers[w.TaskID] = [2]string{w.ID, w.Hostname}
		}
	}

	out := make([]*krillv1.Job, len(rows))
	for i, r := range rows {
		a := args[i]
		j := &krillv1.Job{
			Id:          r.ID,
			Kind:        r.Kind,
			Queue:       r.Queue,
			State:       jobStates[r.State],
			Priority:    int32(r.Priority),    //nolint:gosec // River priorities are 1 to 4
			Attempt:     int32(r.Attempt),     //nolint:gosec // attempts are small
			MaxAttempts: int32(r.MaxAttempts), //nolint:gosec // attempts are small
			Args:        string(r.EncodedArgs),
			CreatedAt:   timestamppb.New(r.CreatedAt),
			ScheduledAt: timestamppb.New(r.ScheduledAt),
			AttemptedAt: tsPtr(r.AttemptedAt),
			FinalizedAt: tsPtr(r.FinalizedAt),
			StartedBy:   starters[jobmeta.Starter(r.Metadata)],
			Cancelling:  r.State == rivertype.JobStateRunning && cancelAttempted(r.Metadata),
		}
		switch {
		case a.FrameID != 0:
			j.Target = names[key("frame", a.FrameID)]
		case a.ClipID != 0:
			j.Target = names[key("clip", a.ClipID)]
		case a.VideoID != 0:
			j.Target = names[key("video", a.VideoID)]
		case a.DatasetID != 0:
			j.Target = names[key("dataset", a.DatasetID)]
		}
		for _, e := range r.Errors {
			j.Errors = append(j.Errors, &krillv1.JobError{
				At:      timestamppb.New(e.At),
				Attempt: int32(e.Attempt), //nolint:gosec // attempts are small
				Message: e.Error,
			})
		}
		if t, ok := tasks[r.ID]; ok && r.State == rivertype.JobStateRunning {
			j.Gpu = &krillv1.GpuTask{Id: t.ID, PickedAt: ts(t.StartedAt)}
			if w, ok := workers[t.ID]; ok {
				j.Gpu.WorkerId, j.Gpu.WorkerHostname = w[0], w[1]
			}
		}
		out[i] = j
	}
	return out, nil
}

func tsPtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// cancelAttempted reports whether River was asked to cancel the running job.
func cancelAttempted(metadata []byte) bool {
	var md struct {
		CancelAttemptedAt *time.Time `json:"cancel_attempted_at"`
	}
	return json.Unmarshal(metadata, &md) == nil && md.CancelAttemptedAt != nil
}

// jobTx loads a job for a change inside tx.
func (s *ComputeService) jobTx(ctx context.Context, tx pgx.Tx, id int64) (*rivertype.JobRow, error) {
	job, err := s.jobs.JobGetTx(ctx, tx, id)
	if errors.Is(err, rivertype.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("job not found"))
	}
	if err != nil {
		return nil, rpc.Internal(err, "load job")
	}
	return job, nil
}

func (s *ComputeService) recordJob(ctx context.Context, q *db.Queries, action string, job *rivertype.JobRow, details any) error {
	described, err := s.describeJobs(ctx, q, []*rivertype.JobRow{job})
	if err != nil {
		return err
	}
	name := job.Kind
	if t := described[0].GetTarget(); t != "" {
		name += " · " + t
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     action,
		TargetType: audit.TargetJob,
		TargetID:   strconv.FormatInt(job.ID, 10),
		TargetName: name,
		Details:    details,
	})
	if err != nil {
		return rpc.Internal(err, "record audit entry")
	}
	return nil
}

// finishJob commits tx and returns the job as it is now.
func (s *ComputeService) finishJob(ctx context.Context, tx pgx.Tx, id int64) (*krillv1.Job, error) {
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	job, err := s.jobs.JobGet(ctx, id)
	if err != nil {
		return nil, rpc.Internal(err, "load job")
	}
	described, err := s.describeJobs(ctx, s.q, []*rivertype.JobRow{job})
	if err != nil {
		return nil, err
	}
	return described[0], nil
}

var errJobDone = connect.NewError(connect.CodeFailedPrecondition, errors.New("the job already finished"))

func (s *ComputeService) RetryJob(ctx context.Context, req *krillv1.RetryJobRequest) (*krillv1.RetryJobResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	job, err := s.jobTx(ctx, tx, req.GetId())
	if err != nil {
		return nil, err
	}
	// A completed job is left alone: running an ingest again would delete
	// the video's clips and every label on them.
	switch job.State {
	case rivertype.JobStateDiscarded, rivertype.JobStateCancelled, rivertype.JobStateRetryable:
	default:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("only failed, cancelled, or retrying jobs can be retried"))
	}
	if err := requeue(ctx, q, job); err != nil {
		return nil, err
	}
	if _, err := s.jobs.JobRetryTx(ctx, tx, job.ID); err != nil {
		return nil, rpc.Internal(err, "retry job")
	}
	if err := s.recordJob(ctx, q, audit.JobRetry, job, nil); err != nil {
		return nil, err
	}
	out, err := s.finishJob(ctx, tx, job.ID)
	if err != nil {
		return nil, err
	}
	return &krillv1.RetryJobResponse{Job: out}, nil
}

// requeue puts what a job works on back in the state the job expects, and
// fails when it was deleted or something else started it again.
func requeue(ctx context.Context, q *db.Queries, job *rivertype.JobRow) error {
	a := parseArgs(job)
	var n int64
	var err error
	var gone string
	switch job.Kind {
	case ingest.Args{}.Kind():
		n, err = q.RequeueIngest(ctx, a.VideoID)
		gone = "the video was deleted or ingested again since"
	case export.Args{}.Kind():
		n, err = q.RequeueDataset(ctx, a.DatasetID)
		gone = "the export was deleted"
	case annotation.TrackArgs{}.Kind(), annotation.ExemplarArgs{}.Kind():
		n = 1
		if _, err = q.GetTrack(ctx, a.TrackID); errors.Is(err, pgx.ErrNoRows) {
			n, err = 0, nil
		}
		gone = "the track was deleted"
	default:
		return nil
	}
	if err != nil {
		return rpc.Internal(err, "requeue")
	}
	if n == 0 {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(gone))
	}
	return nil
}

const cancelledByAdmin = "Cancelled by an admin."

func (s *ComputeService) CancelJob(ctx context.Context, req *krillv1.CancelJobRequest) (*krillv1.CancelJobResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	job, err := s.jobTx(ctx, tx, req.GetId())
	if err != nil {
		return nil, err
	}
	switch job.State {
	case rivertype.JobStateCompleted, rivertype.JobStateCancelled, rivertype.JobStateDiscarded:
		return nil, errJobDone
	}
	// A running job's context is cancelled, its GPU task leaves the
	// dispatcher, and its own failure handling cleans up. Anything else never
	// runs, so it is cleaned up here.
	if job.State != rivertype.JobStateRunning {
		if err := cleanUpCancelled(ctx, q, job); err != nil {
			return nil, err
		}
	}
	if _, err := s.jobs.JobCancelTx(ctx, tx, job.ID); err != nil {
		return nil, rpc.Internal(err, "cancel job")
	}
	if err := s.recordJob(ctx, q, audit.JobCancel, job, nil); err != nil {
		return nil, err
	}
	out, err := s.finishJob(ctx, tx, job.ID)
	if err != nil {
		return nil, err
	}
	return &krillv1.CancelJobResponse{Job: out}, nil
}

func cleanUpCancelled(ctx context.Context, q *db.Queries, job *rivertype.JobRow) error {
	a := parseArgs(job)
	var err error
	switch job.Kind {
	case ingest.Args{}.Kind():
		err = q.FailIngest(ctx, db.FailIngestParams{ID: a.VideoID, Error: cancelledByAdmin})
	case export.Args{}.Kind():
		err = q.FailDataset(ctx, db.FailDatasetParams{ID: a.DatasetID, Error: cancelledByAdmin})
	case annotation.TrackArgs{}.Kind():
		// A track created for this job has no boxes yet.
		_, err = q.DeleteTrackIfEmpty(ctx, a.TrackID)
	}
	if err != nil {
		return rpc.Internal(err, "clean up cancelled job")
	}
	return nil
}

func (s *ComputeService) SetJobPriority(ctx context.Context, req *krillv1.SetJobPriorityRequest) (*krillv1.SetJobPriorityResponse, error) {
	p := req.GetPriority()
	if p < minPriority || p > maxPriority {
		return nil, rpc.Invalid("priority must be from %d to %d", minPriority, maxPriority)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	job, err := s.jobTx(ctx, tx, req.GetId())
	if err != nil {
		return nil, err
	}
	// River's client can't change a priority, and its table isn't in the
	// schema sqlc reads. Priority only orders jobs that have not started.
	tag, err := tx.Exec(ctx, `UPDATE river_job SET priority = $2 WHERE id = $1 AND state IN ('available', 'scheduled', 'pending', 'retryable')`, job.ID, p)
	if err != nil {
		return nil, rpc.Internal(err, "set priority")
	}
	if tag.RowsAffected() == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("only jobs that have not started can be reprioritized"))
	}
	if err := s.recordJob(ctx, q, audit.JobPriority, job, audit.Change(job.Priority, p)); err != nil {
		return nil, err
	}
	out, err := s.finishJob(ctx, tx, job.ID)
	if err != nil {
		return nil, err
	}
	return &krillv1.SetJobPriorityResponse{Job: out}, nil
}
