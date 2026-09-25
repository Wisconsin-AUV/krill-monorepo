package tracker

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func TestNextTaskHandsOffQueuedTask(t *testing.T) {
	d := NewDispatcher()
	s := &Service{dispatcher: d}
	want := newTask(7, 1, &krillv1.TrackTask{Id: 7})
	go func() { d.queue <- want }()

	res, err := s.NextTask(t.Context(), &krillv1.NextTaskRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetTrack().GetId() != 7 {
		t.Errorf("got task %d, want 7", res.GetTrack().GetId())
	}
}

func TestNextTaskStopsWhenCanceled(t *testing.T) {
	s := &Service{dispatcher: NewDispatcher()}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := s.NextTask(ctx, &krillv1.NextTaskRequest{})
	if connect.CodeOf(err) != connect.CodeCanceled {
		t.Errorf("err = %v, want canceled", err)
	}
}

func TestReportTrack(t *testing.T) {
	d := NewDispatcher()
	s := &Service{dispatcher: d}
	task := newTask(7, 1, &krillv1.TrackTask{Id: 7, Frames: []*krillv1.TaskFrame{{Id: 100}}})
	d.add(task)

	_, err := s.ReportTrack(t.Context(), &krillv1.ReportTrackRequest{TaskId: 8})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown task: err = %v, want not found", err)
	}

	outside := &krillv1.TrackedBox{FrameId: 101, Box: &krillv1.Box{Width: 0.1, Height: 0.1}}
	_, err = s.ReportTrack(t.Context(), &krillv1.ReportTrackRequest{TaskId: 7, Boxes: []*krillv1.TrackedBox{outside}})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("frame outside task: err = %v, want invalid argument", err)
	}

	if _, err := s.ReportTrack(t.Context(), &krillv1.ReportTrackRequest{TaskId: 7}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-task.progress:
	default:
		t.Error("report did not signal progress")
	}

	if _, err := s.ReportTrack(t.Context(), &krillv1.ReportTrackRequest{TaskId: 7, Done: true}); err != nil {
		t.Fatal(err)
	}
	if err := <-task.done; err != nil {
		t.Errorf("done = %v, want nil", err)
	}
}

func TestFailTask(t *testing.T) {
	d := NewDispatcher()
	s := &Service{dispatcher: d}
	task := newTask(7, 1, &krillv1.TrackTask{Id: 7})
	d.add(task)

	if _, err := s.FailTask(t.Context(), &krillv1.FailTaskRequest{TaskId: 7, Error: "out of memory"}); err != nil {
		t.Fatal(err)
	}
	if err := <-task.done; err == nil || err.Error() != "out of memory" {
		t.Errorf("done = %v, want the worker's error", err)
	}
}
