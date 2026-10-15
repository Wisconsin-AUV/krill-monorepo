package tracker

import (
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/compute"
)

func TestHeartbeatCancelsTasksNobodyWaitsOn(t *testing.T) {
	d := NewDispatcher()
	s := (&Service{dispatcher: d, nodes: compute.NewRegistry()}).WithModel(func() string { return "facebook/sam3" })
	d.add(newTask(7, 1, &krillv1.TrackTask{Id: 7}))

	beat := func(task int64, commandID uint64) *krillv1.HeartbeatResponse {
		t.Helper()
		res, err := s.Heartbeat(t.Context(), &krillv1.HeartbeatRequest{InstanceId: "w1", TaskId: task, CommandId: commandID})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res := beat(7, 0); res.GetCancelTaskId() != 0 || res.GetModel() != "facebook/sam3" {
		t.Errorf("running task: %v", res)
	}
	// Task 9's job was cancelled, or the API restarted while it ran.
	if res := beat(9, 0); res.GetCancelTaskId() != 9 {
		t.Errorf("unknown task: cancel = %d, want 9", res.GetCancelTaskId())
	}
	if res := beat(0, 0); res.GetCancelTaskId() != 0 {
		t.Errorf("idle: cancel = %d", res.GetCancelTaskId())
	}

	if err := s.nodes.Command("w1", krillv1.WorkerAction_WORKER_ACTION_UNLOAD); err != nil {
		t.Fatal(err)
	}
	cmd := beat(0, 0).GetCommand()
	if cmd.GetAction() != krillv1.WorkerAction_WORKER_ACTION_UNLOAD {
		t.Fatalf("command = %v", cmd)
	}
	// A lost response is made up for by the next one.
	if again := beat(0, 0).GetCommand(); again.GetId() != cmd.GetId() {
		t.Errorf("resent command %v, want %v", again, cmd)
	}
	if done := beat(0, cmd.GetId()).GetCommand(); done != nil {
		t.Errorf("command %v still sent after the worker finished it", done)
	}
}

func TestCommandIDsOutgrowWhatWorkersSaw(t *testing.T) {
	r := compute.NewRegistry()
	// The worker finished a command from an API whose clock ran ahead.
	const seen = uint64(1) << 62
	r.RecordWorker("w1", compute.Report{CommandID: seen})
	if err := r.Command("w1", krillv1.WorkerAction_WORKER_ACTION_WARM_UP); err != nil {
		t.Fatal(err)
	}
	if cmd := r.RecordWorker("w1", compute.Report{CommandID: seen}); cmd.GetId() <= seen {
		t.Errorf("command id %d would be skipped by a worker that saw %d", cmd.GetId(), seen)
	}
	if err := r.Command("gone", krillv1.WorkerAction_WORKER_ACTION_WARM_UP); err == nil {
		t.Error("command for an unknown worker was queued")
	}
}
