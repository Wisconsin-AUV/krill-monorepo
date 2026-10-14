package tracker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/annotation"
)

type priority int

// Lower values are handed out first.
const (
	interactive priority = iota
	background
	numPriorities
)

type task struct {
	id       int64
	priority priority
	// trackID is 0 for exemplar and embed tasks.
	trackID int64
	msg     *krillv1.NextTaskResponse
	frames  map[int64]bool
	// model is set by ReportEmbed before done is signaled.
	model atomic.Pointer[string]
	// picked is closed when a worker takes the task, at pickedAt.
	picked   chan struct{}
	pickedAt time.Time
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
	frames := make(map[int64]bool, len(msg.GetFrames()))
	for _, f := range msg.GetFrames() {
		frames[f.GetId()] = true
	}
	t := newBareTask(id, interactive, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Track{Track: msg}})
	t.trackID = trackID
	t.frames = frames
	return t
}

func newExemplarTask(id int64, msg *krillv1.ExemplarTask) *task {
	t := newBareTask(id, interactive, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Exemplar{Exemplar: msg}})
	t.frames = map[int64]bool{msg.GetFrame().GetId(): true}
	return t
}

func newEmbedTask(id int64, msg *krillv1.EmbedTask) *task {
	return newBareTask(id, background, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Embed{Embed: msg}})
}

func newBareTask(id int64, p priority, msg *krillv1.NextTaskResponse) *task {
	return &task{
		id:       id,
		priority: p,
		msg:      msg,
		picked:   make(chan struct{}),
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
// It is in memory because only one API process runs. Interactive tasks are
// always handed out before background ones so bulk embedding never delays a
// click by more than the background task already running.
type Dispatcher struct {
	ids     atomic.Int64
	mu      sync.Mutex
	pending [numPriorities][]*task
	// ready wakes a polling worker when a task is pushed.
	ready   chan struct{}
	running map[int64]*task
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{ready: make(chan struct{}, 1), running: map[int64]*task{}}
}

func (d *Dispatcher) nextID() int64 {
	return d.ids.Add(1)
}

func (d *Dispatcher) wake() {
	select {
	case d.ready <- struct{}{}:
	default:
	}
}

func (d *Dispatcher) push(t *task) {
	d.mu.Lock()
	d.pending[t.priority] = append(d.pending[t.priority], t)
	d.mu.Unlock()
	d.wake()
}

// take pops the oldest task of the highest priority, or nil.
func (d *Dispatcher) take() *task {
	d.mu.Lock()
	defer d.mu.Unlock()
	for p := range d.pending {
		if q := d.pending[p]; len(q) > 0 {
			t := q[0]
			d.pending[p] = q[1:]
			t.pickedAt = time.Now()
			close(t.picked)
			if d.hasPending() {
				d.wake()
			}
			return t
		}
	}
	return nil
}

func (d *Dispatcher) hasPending() bool {
	for _, q := range d.pending {
		if len(q) > 0 {
			return true
		}
	}
	return false
}

// withdraw removes t if no worker has taken it yet and reports whether it did.
func (d *Dispatcher) withdraw(t *task) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	q := d.pending[t.priority]
	for i, p := range q {
		if p == t {
			d.pending[t.priority] = append(q[:i:i], q[i+1:]...)
			return true
		}
	}
	return false
}

// offer queues t and waits for a worker to take it.
func (d *Dispatcher) offer(ctx context.Context, t *task, timeout time.Duration) error {
	d.push(t)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-t.picked:
		return nil
	case <-timer.C:
		if d.withdraw(t) {
			return errNoWorker
		}
	case <-ctx.Done():
		if d.withdraw(t) {
			return ctx.Err()
		}
	}
	// A worker took it while we gave up waiting.
	return nil
}

// next waits up to timeout for a task. It returns nil when none arrives.
func (d *Dispatcher) next(ctx context.Context, timeout time.Duration) (*task, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if t := d.take(); t != nil {
			return t, nil
		}
		select {
		case <-d.ready:
		case <-timer.C:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
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

// Queue counts the tasks waiting for a worker.
func (d *Dispatcher) Queue() (waitingInteractive, waitingBackground int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending[interactive]), len(d.pending[background])
}

// TaskInfo describes a task a worker is running.
type TaskInfo struct {
	ID        int64
	Kind      string
	Frames    int
	StartedAt time.Time
}

// Running describes task id if a worker has taken it and not finished.
func (d *Dispatcher) Running(id int64) (TaskInfo, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t, ok := d.running[id]
	if !ok || t.pickedAt.IsZero() {
		return TaskInfo{}, false
	}
	info := TaskInfo{ID: id, StartedAt: t.pickedAt}
	switch m := t.msg.GetTask().(type) {
	case *krillv1.NextTaskResponse_Track:
		info.Kind, info.Frames = "track", len(m.Track.GetFrames())
	case *krillv1.NextTaskResponse_Exemplar:
		info.Kind, info.Frames = "exemplar", 1
	case *krillv1.NextTaskResponse_Embed:
		info.Kind, info.Frames = "embed", len(m.Embed.GetFrames())
	}
	return info, true
}
