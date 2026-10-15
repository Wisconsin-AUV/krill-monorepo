// Package pipeline trains YOLO on verified labels, pre-labels unlabeled
// frames with the pinned model or the classic CV plugins, and schedules
// retrains. Held-out test videos are never trained on or pre-labeled.
package pipeline

import (
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/tracker"
)

const (
	// TrainQueue runs one training run at a time, since each one holds its
	// job for hours.
	TrainQueue = "training"
	// PrelabelQueue runs one pre-labeling pass at a time.
	PrelabelQueue = "prelabel"
)

type Deps struct {
	Pool       *pgxpool.Pool
	Store      *storage.Store
	Dispatcher *tracker.Dispatcher
	// Config returns the live settings.
	Config  func() config.Config
	Version string
}

// Pipeline holds what the River workers and the RPC services share. Only one
// API process runs, so cancelling and collecting results stays in memory.
type Pipeline struct {
	Deps
	q    *db.Queries
	jobs *river.Client[pgx.Tx]
	// cancels stops the job working a run, keyed by jobKey.
	cancels sync.Map
	// trains maps a train task id to what FinishTrain needs.
	trains sync.Map
	// prelabels maps a prelabel task id to its *collector.
	prelabels sync.Map
	// previews maps a preview task id to a chan *previewResult.
	previews sync.Map
}

func New(d Deps) *Pipeline {
	return &Pipeline{Deps: d, q: db.New(d.Pool)}
}

// SetJobs gives the pipeline the client to queue jobs with, once the
// workers are registered and the client exists.
func (p *Pipeline) SetJobs(jobs *river.Client[pgx.Tx]) {
	p.jobs = jobs
}

// AddWorkers registers the River workers for training, pre-labeling, and the
// retrain schedule.
func (p *Pipeline) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &TrainWorker{p: p})
	river.AddWorker(workers, &PrelabelWorker{p: p})
	river.AddWorker(workers, &ScheduleWorker{p: p})
}

// Queues are the River queues the workers need.
func Queues() map[string]river.QueueConfig {
	return map[string]river.QueueConfig{
		TrainQueue:    {MaxWorkers: 1},
		PrelabelQueue: {MaxWorkers: 1},
	}
}

type jobKey struct {
	kind string
	id   int64
}

// track registers cancel for the job working a run until the returned func
// is called.
func (p *Pipeline) track(kind string, id int64, cancel func()) func() {
	k := jobKey{kind, id}
	p.cancels.Store(k, cancel)
	return func() { p.cancels.Delete(k) }
}

// stop cancels the job working a run, if one is.
func (p *Pipeline) stop(kind string, id int64) {
	if c, ok := p.cancels.Load(jobKey{kind, id}); ok {
		c.(func())()
	}
}
