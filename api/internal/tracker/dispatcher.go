package tracker

import (
	"context"
	"errors"
	"sync"
	"time"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/annotation"
)

const (
	// pickupTimeout is how long a task waits for a GPU worker to poll.
	pickupTimeout = 2 * time.Minute
	// idleTimeout fails a task whose worker stopped reporting, for example
	// because it crashed.
	idleTimeout = 2 * time.Minute
)

var errNoWorker = errors.New("no GPU worker picked up the task")

type task struct {
	id int64
	// trackID is 0 for exemplar tasks.
	trackID int64
	msg     *krillv1.NextTaskResponse
	frames  map[int64]bool
	// progress gets a signal on each report, and done gets the result.
	progress chan struct{}
	done     chan error
	// found gets an exemplar task's boxes before done is signalled.
	found chan exemplarResult
}

type exemplarResult struct {
	model string
	boxes []annotation.Box
}

func newTask(id, trackID int64, msg *krillv1.TrackTask) *task {
	t := blankTask(id, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Track{Track: msg}})
	t.trackID = trackID
	for _, f := range msg.GetFrames() {
		t.frames[f.GetId()] = true
	}
	return t
}

func newExemplarTask(id int64, msg *krillv1.ExemplarTask) *task {
	t := blankTask(id, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Exemplar{Exemplar: msg}})
	t.frames[msg.GetFrame().GetId()] = true
	return t
}

func blankTask(id int64, msg *krillv1.NextTaskResponse) *task {
	return &task{
		id:       id,
		msg:      msg,
		frames:   map[int64]bool{},
		progress: make(chan struct{}, 1),
		done:     make(chan error, 1),
		found:    make(chan exemplarResult, 1),
	}
}

func (t *task) touch() {
	select {
	case t.progress <- struct{}{}:
	default:
	}
}

func (t *task) finish(err error) {
	select {
	case t.done <- err:
	default:
	}
}

// Dispatcher hands tasks from River jobs to the GPU worker polling NextTask.
// It is in memory because only one API process runs.
type Dispatcher struct {
	queue   chan *task
	mu      sync.Mutex
	running map[int64]*task
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{queue: make(chan *task), running: map[int64]*task{}}
}

func (d *Dispatcher) add(t *task) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running[t.id] = t
}

func (d *Dispatcher) remove(id int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.running, id)
}

func (d *Dispatcher) get(id int64) (*task, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t, ok := d.running[id]
	return t, ok
}

// run hands the task to the next GPU worker that polls and waits for it to
// finish.
func (d *Dispatcher) run(ctx context.Context, t *task) error {
	d.add(t)
	defer d.remove(t.id)

	pickup := time.NewTimer(pickupTimeout)
	defer pickup.Stop()
	select {
	case d.queue <- t:
	case <-pickup.C:
		return errNoWorker
	case <-ctx.Done():
		return ctx.Err()
	}

	idle := time.NewTimer(idleTimeout)
	defer idle.Stop()
	for {
		select {
		case err := <-t.done:
			return err
		case <-t.progress:
			idle.Reset(idleTimeout)
		case <-idle.C:
			return errors.New("GPU worker stopped reporting")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
