package taxonomy

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

// OrientQueue is apart from the tracking and embedding queues so a fit that
// takes an hour never holds up either.
const OrientQueue = "orient"

// OrientArgs fits a rotated box to each box of an oriented label type that
// has none, or with Keypoints, the reference quad to each box of a type with
// keypoints that has none labeled.
type OrientArgs struct {
	LabelTypeID int64 `json:"label_type_id"`
	Keypoints   bool  `json:"keypoints,omitempty"`
}

func (OrientArgs) Kind() string { return "orient" }

// A type waiting for or in the middle of a fit is not queued twice.
func (OrientArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       OrientQueue,
		MaxAttempts: 1,
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

func (s *Service) SetLabelReference(ctx context.Context, req *krillv1.SetLabelReferenceRequest) (*krillv1.SetLabelReferenceResponse, error) {
	key := req.GetKey()
	if key != "" {
		if !ValidExampleKey(req.GetLabelTypeId(), key) {
			return nil, rpc.Invalid("key was not issued for this label type")
		}
		exists, err := s.store.Exists(ctx, key)
		if err != nil {
			return nil, rpc.Internal(err, "check upload")
		}
		if !exists {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("reference image has not been uploaded"))
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	old, err := q.SetLabelReference(ctx, db.SetLabelReferenceParams{
		ID: req.GetLabelTypeId(), ReferenceKey: pgtype.Text{String: key, Valid: key != ""},
	})
	if err != nil {
		return nil, rpc.DBError(err, "label type")
	}
	if old.String != key {
		lt, err := q.GetLabelType(ctx, req.GetLabelTypeId())
		if err != nil {
			return nil, rpc.DBError(err, "label type")
		}
		if err := record(ctx, q, audit.LabelTypeUpdate, lt, map[string]any{"changed": []string{"reference"}}); err != nil {
			return nil, err
		}
	}
	types, err := s.list(ctx, q)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	if old.Valid && old.String != key {
		if err := s.store.Remove(ctx, old.String); err != nil {
			slog.WarnContext(ctx, "remove label reference", "label_type_id", req.GetLabelTypeId(), "err", err)
		}
	}
	for _, t := range types {
		if t.GetId() == req.GetLabelTypeId() {
			return &krillv1.SetLabelReferenceResponse{LabelType: t}, nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("label type not found"))
}

func (s *Service) FitRotations(ctx context.Context, req *krillv1.FitRotationsRequest) (*krillv1.FitRotationsResponse, error) {
	lt, err := s.q.GetLabelType(ctx, req.GetLabelTypeId())
	if err != nil {
		return nil, rpc.DBError(err, "label type")
	}
	if !lt.Oriented {
		return nil, rpc.Invalid("label type is not oriented")
	}
	n, err := s.q.CountUnrotatedBoxes(ctx, lt.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count boxes")
	}
	if n == 0 {
		return &krillv1.FitRotationsResponse{}, nil
	}
	if _, err := s.jobs.Insert(ctx, OrientArgs{LabelTypeID: lt.ID}, nil); err != nil {
		return nil, rpc.Internal(err, "queue fitting")
	}
	return &krillv1.FitRotationsResponse{BoxCount: n}, nil
}

func (s *Service) FitKeypoints(ctx context.Context, req *krillv1.FitKeypointsRequest) (*krillv1.FitKeypointsResponse, error) {
	lt, err := s.q.GetLabelType(ctx, req.GetLabelTypeId())
	if err != nil {
		return nil, rpc.DBError(err, "label type")
	}
	pose, err := ParsePose(lt)
	if err != nil {
		return nil, rpc.Internal(err, "decode label type")
	}
	if !pose.HasReference() {
		return nil, rpc.Invalid("label type has no quad covered by its reference design")
	}
	if !lt.ReferenceKey.Valid {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("label type has no reference design"))
	}
	n, err := s.q.CountUnposedBoxes(ctx, lt.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count boxes")
	}
	if n == 0 {
		return &krillv1.FitKeypointsResponse{}, nil
	}
	if _, err := s.jobs.Insert(ctx, OrientArgs{LabelTypeID: lt.ID, Keypoints: true}, nil); err != nil {
		return nil, rpc.Internal(err, "queue fitting")
	}
	return &krillv1.FitKeypointsResponse{BoxCount: n}, nil
}
