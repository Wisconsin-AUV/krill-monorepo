package annotation

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wauv/krill/api/internal/db"
)

// boxOp is what an edit does to a track's box on one frame.
type boxOp int

const (
	// opSet draws, moves, or resizes the box.
	opSet boxOp = iota
	opDelete
	// opRelabel changes the track's label type.
	opRelabel
)

type rect struct{ x, y, w, h float64 }

func (r rect) same(s db.ListBoxStatesRow) bool {
	const eps = 1e-6
	return math.Abs(r.x-s.X.Float64) < eps && math.Abs(r.y-s.Y.Float64) < eps &&
		math.Abs(r.w-s.Width.Float64) < eps && math.Abs(r.h-s.Height.Float64) < eps
}

// boxEvent names what the edit means for labeler stats, or returns "" when
// it is ordinary labeling. Touching a proposal decides it. Changing a box on a
// frame someone else already marked done corrects their work. to is the new
// box for opSet.
func boxEvent(me uuid.UUID, s db.ListBoxStatesRow, op boxOp, to rect) string {
	if s.BoxStatus.Valid && s.BoxStatus.String == "proposed" {
		switch op {
		case opSet:
			// Keeping a model's box under a class a labeler changed is an edit.
			if !to.same(s) || s.ClassEdited && modelSource(s) {
				return "edited"
			}
			return "accepted"
		case opDelete:
			return "rejected"
		}
		return ""
	}
	marked := s.FrameStatus != "unlabeled" && s.StatusBy.Valid && uuid.UUID(s.StatusBy.Bytes) != me
	if !marked {
		return ""
	}
	exists := s.BoxStatus.Valid && s.BoxStatus.String != "rejected"
	switch op {
	case opSet:
		if !exists {
			return "added"
		}
		if !to.same(s) {
			return "moved"
		}
	case opDelete:
		if exists {
			return "deleted"
		}
	case opRelabel:
		if exists {
			return "relabeled"
		}
	}
	return ""
}

func modelSource(s db.ListBoxStatesRow) bool {
	return s.Source.Valid && (s.Source.String == "yolo" || s.Source.String == "cv")
}

// rejectsPrelabel reports whether the edit tells pre-labelers that the
// proposal's box is not an object of the track's current label type: it is
// deleted, its track moves off the class the pre-labeler gave it, or it is
// moved too far for the moved box to cover where it was.
func rejectsPrelabel(s db.ListBoxStatesRow, op boxOp, to rect) bool {
	if !s.BoxStatus.Valid || s.BoxStatus.String != "proposed" || !modelSource(s) {
		return false
	}
	switch op {
	case opDelete:
		return true
	case opRelabel:
		// Moving between classes a labeler picked rejects none of the model's.
		return s.ProposedLabelTypeID.Valid && s.ProposedLabelTypeID.Int64 == s.LabelTypeID
	case opSet:
		from := Rect{X: s.X.Float64, Y: s.Y.Float64, W: s.Width.Float64, H: s.Height.Float64}
		return iou(Rect{X: to.x, Y: to.y, W: to.w, H: to.h}, from) <= DuplicateIoU
	}
	return false
}

func isCorrection(kind string) bool {
	return kind != "accepted" && kind != "edited" && kind != "rejected"
}

// recordBoxEvents records what an edit by me does to the track's boxes, on
// frameID only when it is set. Run it in the edit's transaction, before the
// edit.
func recordBoxEvents(ctx context.Context, q *db.Queries, me uuid.UUID, trackID int64, frameID *int64, op boxOp, to rect) error {
	frame := pgtype.Int8{}
	if frameID != nil {
		frame = pgtype.Int8{Int64: *frameID, Valid: true}
	}
	states, err := q.ListBoxStates(ctx, db.ListBoxStatesParams{TrackID: trackID, FrameID: frame})
	if err != nil {
		return fmt.Errorf("load boxes of track %d: %w", trackID, err)
	}
	return insertBoxEvents(ctx, q, me, states, op, to)
}

func insertBoxEvents(ctx context.Context, q *db.Queries, me uuid.UUID, states []db.ListBoxStatesRow, op boxOp, to rect) error {
	p := db.InsertBoxEventsParams{UserID: me}
	var rej db.InsertRejectedPrelabelsParams
	for _, s := range states {
		if rejectsPrelabel(s, op, to) {
			rej.FrameIds = append(rej.FrameIds, s.FrameID)
			rej.LabelTypeIds = append(rej.LabelTypeIds, s.LabelTypeID)
			rej.Xs = append(rej.Xs, s.X.Float64)
			rej.Ys = append(rej.Ys, s.Y.Float64)
			rej.Widths = append(rej.Widths, s.Width.Float64)
			rej.Heights = append(rej.Heights, s.Height.Float64)
		}
		kind := boxEvent(me, s, op, to)
		if kind == "" {
			continue
		}
		source := "human"
		if s.Source.Valid {
			source = s.Source.String
		}
		labeler, markedAt := uuid.Nil, pgtype.Timestamptz{}
		if isCorrection(kind) {
			labeler, markedAt = s.StatusBy.Bytes, s.StatusAt
		}
		p.Kinds = append(p.Kinds, kind)
		p.FrameIds = append(p.FrameIds, s.FrameID)
		p.TrackIds = append(p.TrackIds, s.TrackID)
		p.LabelTypeIds = append(p.LabelTypeIds, s.LabelTypeID)
		p.Sources = append(p.Sources, source)
		p.LabelerIds = append(p.LabelerIds, labeler)
		p.MarkedAts = append(p.MarkedAts, markedAt)
	}
	if len(p.Kinds) > 0 {
		if err := q.InsertBoxEvents(ctx, p); err != nil {
			return fmt.Errorf("record box events: %w", err)
		}
	}
	if len(rej.FrameIds) > 0 {
		if err := q.InsertRejectedPrelabels(ctx, rej); err != nil {
			return fmt.Errorf("record rejected proposals: %w", err)
		}
	}
	return nil
}

// recordCopies records the boxes CopyAnnotations just added to a frame. They
// are new, so their states are read back as if the boxes weren't there yet.
func recordCopies(ctx context.Context, q *db.Queries, me uuid.UUID, to db.Frame, copied []db.Annotation) error {
	if to.Status == "unlabeled" || !to.StatusBy.Valid || to.StatusBy.Bytes == me {
		return nil
	}
	states := make([]db.ListBoxStatesRow, 0, len(copied))
	for _, a := range copied {
		rows, err := q.ListBoxStates(ctx, db.ListBoxStatesParams{TrackID: a.TrackID, FrameID: pgtype.Int8{Int64: to.ID, Valid: true}})
		if err != nil {
			return fmt.Errorf("load box of track %d: %w", a.TrackID, err)
		}
		for _, s := range rows {
			s.BoxStatus, s.Source = pgtype.Text{}, pgtype.Text{}
			states = append(states, s)
		}
	}
	return insertBoxEvents(ctx, q, me, states, opSet, rect{})
}
