package tracker

import (
	"context"
	"time"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

// Task is GPU work that another package hands to the worker through the
// dispatcher, such as training or pre-labeling.
type Task = task

// Priority orders tasks for the GPU worker.
type Priority = priority

const (
	Interactive = interactive
	Background  = background
	Training    = training
)

// NextID returns an id for a new task's message.
func (d *Dispatcher) NextID() int64 {
	return d.nextID()
}

// NewTask wraps msg, whose task id must be id. idle is how long the worker may
// go without reporting before the task fails, or 0 for the default.
func NewTask(id int64, p Priority, msg *krillv1.NextTaskResponse, idle time.Duration) *Task {
	t := newBareTask(id, p, msg)
	t.idle = idle
	return t
}

// Run hands t to a worker and waits for it to finish or fail.
func (d *Dispatcher) Run(ctx context.Context, t *Task, pickup time.Duration) error {
	return run(ctx, d, t, pickup)
}

// Lookup finds a task that has been offered and not finished.
func (d *Dispatcher) Lookup(id int64) (*Task, bool) {
	return d.get(id)
}

// TakeWaiting pops the next waiting task other than training, without
// waiting for one. A worker that is training calls it between batches.
func (d *Dispatcher) TakeWaiting() *krillv1.NextTaskResponse {
	if t := d.takeBefore(training); t != nil {
		return t.msg
	}
	return nil
}

func (t *task) ID() int64 {
	return t.id
}

func (t *task) Msg() *krillv1.NextTaskResponse {
	return t.msg
}

// Touch tells the dispatcher the worker is still on it.
func (t *task) Touch() {
	t.touch()
}

// Finish ends the task. Only the first call counts.
func (t *task) Finish(err error) {
	t.finish(err)
}
