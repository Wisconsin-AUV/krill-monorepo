package video

import (
	"context"
	"fmt"

	"github.com/wauv/krill/api/internal/db"
)

// SplitTest is the held-out test split.
const SplitTest = "test"

// HeldOut reports whether a video with this split is held out. Test metrics
// are only honest if no model has seen or proposed boxes on the footage.
func HeldOut(split string) bool {
	return split == SplitTest
}

// IsHeldOut must be checked by model pre-labelers (yolo, cv) before proposing
// boxes on a video. Human-initiated tracking (sam) is allowed on held-out
// videos.
func IsHeldOut(ctx context.Context, q *db.Queries, videoID int64) (bool, error) {
	v, err := q.GetVideo(ctx, videoID)
	if err != nil {
		return false, fmt.Errorf("get video %d: %w", videoID, err)
	}
	return HeldOut(v.Split), nil
}
