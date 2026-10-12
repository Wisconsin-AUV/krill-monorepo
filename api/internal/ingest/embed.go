package ingest

import "github.com/riverqueue/river"

// EmbedQueue is separate from the tracking queue so a clip waiting for the
// GPU worker never holds up a click.
const EmbedQueue = "embed"

// EmbedArgs computes SAM image embeddings for every frame of a clip.
type EmbedArgs struct {
	ClipID int64 `json:"clip_id"`
}

func (EmbedArgs) Kind() string { return "embed" }

func (EmbedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: EmbedQueue, MaxAttempts: 5}
}
