package annotation

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/taxonomy"
)

type Service struct {
	krillv1connect.UnimplementedAnnotationServiceHandler
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: db.New(pool)}
}

var errWrongClip = connect.NewError(connect.CodeInvalidArgument, errors.New("frame is not in the track's clip"))

func (s *Service) typeAttributes(ctx context.Context, q *db.Queries, labelTypeID int64) ([]taxonomy.Attribute, error) {
	lt, err := q.GetLabelType(ctx, labelTypeID)
	if err != nil {
		return nil, rpc.DBError(err, "label type")
	}
	attrs, err := taxonomy.ParseAttributes(lt.Attributes)
	if err != nil {
		return nil, rpc.Internal(err, "decode label type")
	}
	return attrs, nil
}

func (s *Service) CreateTrack(ctx context.Context, req *krillv1.CreateTrackRequest) (*krillv1.CreateTrackResponse, error) {
	x, y, w, h, err := clampBox(req.GetBox())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	frame, err := s.q.GetFrame(ctx, req.GetFrameId())
	if err != nil {
		return nil, rpc.DBError(err, "frame")
	}
	if frame.ClipID != req.GetClipId() {
		return nil, errWrongClip
	}
	defs, err := s.typeAttributes(ctx, s.q, req.GetLabelTypeId())
	if err != nil {
		return nil, err
	}
	values := req.GetAttributes()
	if values == nil {
		values = map[string]string{}
	}
	if err := taxonomy.CheckValues(defs, values); err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, rpc.Internal(err, "encode attributes")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	track, err := q.CreateTrack(ctx, db.CreateTrackParams{ClipID: frame.ClipID, LabelTypeID: req.GetLabelTypeId(), Attributes: raw})
	if err != nil {
		return nil, rpc.Internal(err, "create track")
	}
	ann, err := q.UpsertAnnotation(ctx, db.UpsertAnnotationParams{
		TrackID: track.ID, FrameID: frame.ID, X: x, Y: y, Width: w, Height: h,
	})
	if err != nil {
		return nil, rpc.Internal(err, "create box")
	}
	if err := q.ClearEmptyFrame(ctx, frame.ID); err != nil {
		return nil, rpc.Internal(err, "update frame status")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}

	out, err := Track(track)
	if err != nil {
		return nil, rpc.Internal(err, "decode track")
	}
	return &krillv1.CreateTrackResponse{Track: out, Annotation: Annotation(ann)}, nil
}

func (s *Service) UpdateTrack(ctx context.Context, req *krillv1.UpdateTrackRequest) (*krillv1.UpdateTrackResponse, error) {
	track, err := s.q.GetTrack(ctx, req.GetId())
	if err != nil {
		return nil, rpc.DBError(err, "track")
	}
	values, err := taxonomy.ParseValues(track.Attributes)
	if err != nil {
		return nil, rpc.Internal(err, "decode track")
	}

	labelTypeID := track.LabelTypeID
	if req.LabelTypeId != nil {
		labelTypeID = req.GetLabelTypeId()
	}
	defs, err := s.typeAttributes(ctx, s.q, labelTypeID)
	if err != nil {
		return nil, err
	}
	if req.GetSetAttributes() {
		values = req.GetAttributes()
		if values == nil {
			values = map[string]string{}
		}
		if err := taxonomy.CheckValues(defs, values); err != nil {
			return nil, rpc.Invalid("%s", err)
		}
	} else {
		values = taxonomy.KeepValid(defs, values)
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, rpc.Internal(err, "encode attributes")
	}

	updated, err := s.q.UpdateTrack(ctx, db.UpdateTrackParams{ID: track.ID, LabelTypeID: labelTypeID, Attributes: raw})
	if err != nil {
		return nil, rpc.DBError(err, "track")
	}
	out, err := Track(updated)
	if err != nil {
		return nil, rpc.Internal(err, "decode track")
	}
	return &krillv1.UpdateTrackResponse{Track: out}, nil
}

func (s *Service) DeleteTrack(ctx context.Context, req *krillv1.DeleteTrackRequest) (*krillv1.DeleteTrackResponse, error) {
	n, err := s.q.DeleteTrack(ctx, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "delete track")
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("track not found"))
	}
	return &krillv1.DeleteTrackResponse{}, nil
}

func (s *Service) checkSameClip(ctx context.Context, trackID, frameID int64) error {
	track, err := s.q.GetTrack(ctx, trackID)
	if err != nil {
		return rpc.DBError(err, "track")
	}
	frame, err := s.q.GetFrame(ctx, frameID)
	if err != nil {
		return rpc.DBError(err, "frame")
	}
	if frame.ClipID != track.ClipID {
		return errWrongClip
	}
	return nil
}

func (s *Service) SetBox(ctx context.Context, req *krillv1.SetBoxRequest) (*krillv1.SetBoxResponse, error) {
	x, y, w, h, err := clampBox(req.GetBox())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	if err := s.checkSameClip(ctx, req.GetTrackId(), req.GetFrameId()); err != nil {
		return nil, err
	}
	ann, err := s.q.UpsertAnnotation(ctx, db.UpsertAnnotationParams{
		TrackID: req.GetTrackId(), FrameID: req.GetFrameId(), X: x, Y: y, Width: w, Height: h,
	})
	if err != nil {
		return nil, rpc.Internal(err, "save box")
	}
	// A frame marked empty that now has a box needs to be confirmed again.
	if err := s.q.ClearEmptyFrame(ctx, req.GetFrameId()); err != nil {
		return nil, rpc.Internal(err, "update frame status")
	}
	return &krillv1.SetBoxResponse{Annotation: Annotation(ann)}, nil
}

func (s *Service) DeleteBox(ctx context.Context, req *krillv1.DeleteBoxRequest) (*krillv1.DeleteBoxResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	n, err := q.DeleteAnnotation(ctx, db.DeleteAnnotationParams{TrackID: req.GetTrackId(), FrameID: req.GetFrameId()})
	if err != nil {
		return nil, rpc.Internal(err, "delete box")
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("box not found"))
	}
	left, err := q.CountTrackAnnotations(ctx, req.GetTrackId())
	if err != nil {
		return nil, rpc.Internal(err, "count boxes")
	}
	if left == 0 {
		if _, err := q.DeleteTrack(ctx, req.GetTrackId()); err != nil {
			return nil, rpc.Internal(err, "delete empty track")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.DeleteBoxResponse{TrackDeleted: left == 0}, nil
}

func (s *Service) CopyBoxes(ctx context.Context, req *krillv1.CopyBoxesRequest) (*krillv1.CopyBoxesResponse, error) {
	from, err := s.q.GetFrame(ctx, req.GetFromFrameId())
	if err != nil {
		return nil, rpc.DBError(err, "source frame")
	}
	to, err := s.q.GetFrame(ctx, req.GetToFrameId())
	if err != nil {
		return nil, rpc.DBError(err, "target frame")
	}
	if from.ClipID != to.ClipID {
		return nil, rpc.Invalid("frames are in different clips")
	}
	trackIDs := req.GetTrackIds()
	if trackIDs == nil {
		trackIDs = []int64{}
	}
	rows, err := s.q.CopyAnnotations(ctx, db.CopyAnnotationsParams{
		FromFrameID: from.ID, ToFrameID: to.ID, TrackIds: trackIDs,
	})
	if err != nil {
		return nil, rpc.Internal(err, "copy boxes")
	}
	if len(rows) > 0 {
		if err := s.q.ClearEmptyFrame(ctx, to.ID); err != nil {
			return nil, rpc.Internal(err, "update frame status")
		}
	}
	out := make([]*krillv1.Annotation, len(rows))
	for i, a := range rows {
		out[i] = Annotation(a)
	}
	return &krillv1.CopyBoxesResponse{Annotations: out}, nil
}

func (s *Service) SetFrameStatus(ctx context.Context, req *krillv1.SetFrameStatusRequest) (*krillv1.SetFrameStatusResponse, error) {
	status, ok := frameStatusFromProto(req.GetStatus())
	if !ok {
		return nil, rpc.Invalid("unknown frame status %v", req.GetStatus())
	}
	if status == "empty" {
		boxes, err := s.q.CountFrameAnnotations(ctx, req.GetFrameId())
		if err != nil {
			return nil, rpc.Internal(err, "count boxes")
		}
		if boxes > 0 {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("frame has boxes; delete them before marking it empty"))
		}
	}
	saved, err := s.q.SetFrameStatus(ctx, db.SetFrameStatusParams{ID: req.GetFrameId(), Status: status})
	if err != nil {
		return nil, rpc.DBError(err, "frame")
	}
	return &krillv1.SetFrameStatusResponse{Status: FrameStatus(saved)}, nil
}
