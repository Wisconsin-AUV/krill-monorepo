package tracker

import (
	"sync"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

type task struct {
	id      int64
	trackID int64
	msg     *krillv1.TrackTask
	frames  map[int64]bool
	// progress gets a signal on each report, and done gets the result.
	progress chan struct{}
	done     chan error
}

func newTask(id, trackID int64, msg *krillv1.TrackTask) *task {
	frames := make(map[int64]bool, len(msg.GetFrames()))
	for _, f := range msg.GetFrames() {
		frames[f.GetId()] = true
	}
	return &task{
		id:       id,
		trackID:  trackID,
		msg:      msg,
		frames:   frames,
		progress: make(chan struct{}, 1),
		done:     make(chan error, 1),
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
