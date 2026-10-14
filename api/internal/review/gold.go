package review

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/wauv/krill/api/internal/db"
)

const (
	// RecentGoldChecks is how many of a labeler's latest gold checks decide
	// whether their work goes to review.
	RecentGoldChecks = 5
	// maxBackfill caps how many finished frames one low gold score queues.
	// The window is normally one gold interval, but grows when a labeler has
	// no gold frames left to answer.
	maxBackfill = 500
)

// GoldRouting sends a labeler's work to review while their recent gold
// scores are low.
type GoldRouting struct {
	Threshold   float64
	MinAttempts int
}

// Below reports whether a labeler with this many gold checks, and this mean
// score over their latest RecentGoldChecks, needs their work reviewed.
func (g GoldRouting) Below(attempts int32, recentScore float64) bool {
	return int(attempts) >= g.MinAttempts && recentScore < g.Threshold
}

func (g GoldRouting) below(ctx context.Context, q *db.Queries, userID uuid.UUID) (bool, error) {
	r, err := q.RecentGoldScore(ctx, db.RecentGoldScoreParams{UserID: userID, Recent: RecentGoldChecks})
	if err != nil {
		return false, fmt.Errorf("load recent gold score of %s: %w", userID, err)
	}
	return g.Below(r.Attempts, r.RecentScore), nil
}

// FrameDone queues a frame the labeler just marked done if they are below
// threshold.
func (g GoldRouting) FrameDone(ctx context.Context, q *db.Queries, frameID int64, userID uuid.UUID) error {
	below, err := g.below(ctx, q, userID)
	if err != nil || !below {
		return err
	}
	_, err = Enqueue(ctx, q, frameID, ReasonLowAccuracy, uuid.Nil)
	return err
}

// GoldChecked runs after the labeler's attempt on goldFrameID is saved. If it
// leaves them below threshold, the frames they finished since their previous
// gold check are queued, since that check was the last evidence vouching for
// them.
func (g GoldRouting) GoldChecked(ctx context.Context, q *db.Queries, userID uuid.UUID, goldFrameID int64) error {
	below, err := g.below(ctx, q, userID)
	if err != nil || !below {
		return err
	}
	_, err = q.EnqueueDoneSinceGoldCheck(ctx, db.EnqueueDoneSinceGoldCheckParams{
		Reason:      string(ReasonLowAccuracy),
		UserID:      userID,
		GoldFrameID: goldFrameID,
		Lim:         maxBackfill,
	})
	if err != nil {
		return fmt.Errorf("queue frames done by %s: %w", userID, err)
	}
	return nil
}
