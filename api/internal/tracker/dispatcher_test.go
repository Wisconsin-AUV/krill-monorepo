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
	d.push(embed1)
	d.push(click1)
	d.push(embed2)
	d.push(click2)

	for _, want := range []*task{click1, click2, embed1, embed2} {
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
