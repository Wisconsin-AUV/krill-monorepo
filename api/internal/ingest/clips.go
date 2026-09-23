package ingest

import "math"

const (
	clipTargetSeconds = 15
	clipMinSeconds    = 5
)

type clipRange struct {
	Start int
	Count int
}

// chunkClips splits total frames into clips of about clipTargetSeconds. A
// trailing clip shorter than clipMinSeconds is merged into the one before it,
// so clips land in the 10 to 20 second range.
func chunkClips(total int, fps float64) []clipRange {
	if total <= 0 {
		return nil
	}
	per := max(int(math.Round(clipTargetSeconds*fps)), 1)
	minLen := int(math.Round(clipMinSeconds * fps))

	var clips []clipRange
	for start := 0; start < total; start += per {
		clips = append(clips, clipRange{Start: start, Count: min(per, total-start)})
	}
	if n := len(clips); n > 1 && clips[n-1].Count < minLen {
		clips[n-2].Count += clips[n-1].Count
		clips = clips[:n-1]
	}
	return clips
}
