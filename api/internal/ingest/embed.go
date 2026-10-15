package ingest

import (
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// EmbedQueue is separate from the tracking queue so a clip waiting for the
// GPU worker never holds up a click.
const EmbedQueue = "embed"

// EmbedArgs computes SAM image embeddings for every frame of a clip.
type EmbedArgs struct {
	ClipID int64 `json:"clip_id"`
}

func (EmbedArgs) Kind() string { return "embed" }

// A clip waiting to be embedded is not queued twice, but one embedded before
// can be embedded again, for example after switching models.
func (EmbedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       EmbedQueue,
		MaxAttempts: 5,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRetryable,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
			},
		},
	}
}
