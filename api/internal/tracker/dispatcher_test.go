package tracker

import (
	"errors"
	"testing"
	"time"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func TestDispatcherHandsOutClicksBeforeEmbeddings(t *testing.T) {
	d := NewDispatcher()
	embed1 := newEmbedTask(1, &krillv1.EmbedTask{Id: 1})
	embed2 := newEmbedTask(2, &krillv1.EmbedTask{Id: 2})
	click1 := newTask(3, 1, &krillv1.TrackTask{Id: 3})
	click2 := newTask(4, 1, &krillv1.TrackTask{Id: 4})
	exemplar := newExemplarTask(5, &krillv1.ExemplarTask{Id: 5})
	d.push(embed1)
	d.push(click1)
	d.push(embed2)
	d.push(exemplar)
	d.push(click2)

	for _, want := range []*task{click1, exemplar, click2, embed1, embed2} {
		got, err := d.next(t.Context(), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("got task %d, want %d", got.id, want.id)
		}
	}
}

func TestDispatcherOfferTimesOutWithoutWorker(t *testing.T) {
	d := NewDispatcher()
	task := newEmbedTask(1, &krillv1.EmbedTask{Id: 1})
	if err := d.offer(t.Context(), task, time.Millisecond); !errors.Is(err, errNoWorker) {
		t.Fatalf("err = %v, want errNoWorker", err)
	}
	// A withdrawn task must not reach a worker that polls later.
	got, err := d.next(t.Context(), time.Millisecond)
	if err != nil || got != nil {
		t.Errorf("next = %v, %v, want nothing", got, err)
	}
}

func TestTrainingWorkerOnlyTakesOtherWork(t *testing.T) {
	d := NewDispatcher()
	train := NewTask(1, Training, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Train{Train: &krillv1.TrainTask{Id: 1}}}, 0)
	embed := newEmbedTask(2, &krillv1.EmbedTask{Id: 2})
	click := newTask(3, 1, &krillv1.TrackTask{Id: 3})
	d.push(train)
	d.push(embed)
	d.push(click)

	// A worker checking in between batches must never be handed a second
	// training run, or it would train inside its own training.
	for _, want := range []*task{click, embed} {
		if got := d.TakeWaiting(); got != want.msg {
			t.Fatalf("TakeWaiting = %v, want task %d", got, want.id)
		}
	}
	if got := d.TakeWaiting(); got != nil {
		t.Fatalf("TakeWaiting handed out %v", got)
	}
	if got, _ := d.next(t.Context(), time.Millisecond); got != train {
		t.Fatalf("an idle worker should get the training task, got %v", got)
	}
}
