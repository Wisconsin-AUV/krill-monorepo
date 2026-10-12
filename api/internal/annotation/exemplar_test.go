package annotation

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/riverqueue/river/rivertype"
)

func TestExemplarSearchHidesOtherJobs(t *testing.T) {
	job := &rivertype.JobRow{
		Kind:   "export",
		State:  rivertype.JobStateDiscarded,
		Errors: []rivertype.AttemptError{{Error: "s3: access denied for key AKIA..."}},
	}
	if _, err := exemplarSearch(job); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("err = %v, want not found", err)
	}
}

func TestExemplarSearchReportsWorkerError(t *testing.T) {
	msg := "facebook/sam2.1-hiera-small cannot search by exemplar"
	job := &rivertype.JobRow{
		Kind:   ExemplarArgs{}.Kind(),
		State:  rivertype.JobStateDiscarded,
		Errors: []rivertype.AttemptError{{Error: msg}},
	}
	res, err := exemplarSearch(job)
	if err != nil {
		t.Fatal(err)
	}
	if !res.GetDone() || res.GetError() != msg {
		t.Errorf("got done=%v error=%q, want done with the worker's error", res.GetDone(), res.GetError())
	}
}

func TestExemplarSearchReturnsTracks(t *testing.T) {
	job := &rivertype.JobRow{
		Kind:     ExemplarArgs{}.Kind(),
		State:    rivertype.JobStateCompleted,
		Metadata: []byte(`{"output":{"track_ids":[4,5]}}`),
	}
	res, err := exemplarSearch(job)
	if err != nil {
		t.Fatal(err)
	}
	if !res.GetDone() || len(res.GetTrackIds()) != 2 {
		t.Errorf("got %v, want done with two tracks", res)
	}
}
