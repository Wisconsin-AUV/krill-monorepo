package taxonomy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

const maxDescriptionLen = 500

type Service struct {
	krillv1connect.UnimplementedLabelServiceHandler
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: db.New(pool)}
}

func toProto(lt db.LabelType, tracks, boxes int32) (*krillv1.LabelType, error) {
	attrs, err := ParseAttributes(lt.Attributes)
	if err != nil {
		return nil, err
	}
	out := &krillv1.LabelType{
		Id:          lt.ID,
		Name:        lt.Name,
		Color:       lt.Color,
		Description: lt.Description,
		Position:    lt.Position,
		TrackCount:  tracks,
		BoxCount:    boxes,
	}
	for _, a := range attrs {
		out.Attributes = append(out.Attributes, &krillv1.LabelAttribute{Name: a.Name, Options: a.Options})
	}
	return out, nil
}

func fromProto(attrs []*krillv1.LabelAttribute) []Attribute {
	out := make([]Attribute, len(attrs))
	for i, a := range attrs {
		out[i] = Attribute{Name: a.GetName(), Options: a.GetOptions()}
	}
	return out
}

type typeInput struct {
	name, color, description string
	attrs                    []byte
}

func validate(name, color, description string, attrs []*krillv1.LabelAttribute) (typeInput, error) {
	name, color, clean, err := NormalizeType(name, color, fromProto(attrs))
	if err != nil {
		return typeInput{}, rpc.Invalid("%s", err)
	}
	description = strings.TrimSpace(description)
	if len(description) > maxDescriptionLen {
		return typeInput{}, rpc.Invalid("description must be at most %d characters", maxDescriptionLen)
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return typeInput{}, rpc.Internal(err, "encode attributes")
	}
	return typeInput{name: name, color: color, description: description, attrs: raw}, nil
}

func duplicateName(err error, name string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("a label type named %q already exists", name))
	}
	return nil
}

func (s *Service) ListLabelTypes(ctx context.Context, _ *krillv1.ListLabelTypesRequest) (*krillv1.ListLabelTypesResponse, error) {
	types, err := s.list(ctx, s.q)
	if err != nil {
		return nil, err
	}
	return &krillv1.ListLabelTypesResponse{LabelTypes: types}, nil
}

func (s *Service) list(ctx context.Context, q *db.Queries) ([]*krillv1.LabelType, error) {
	rows, err := q.ListLabelTypes(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list label types")
	}
	out := make([]*krillv1.LabelType, len(rows))
	for i, r := range rows {
		if out[i], err = toProto(r.LabelType, r.TrackCount, r.BoxCount); err != nil {
			return nil, rpc.Internal(err, "decode label type")
		}
	}
	return out, nil
}

func (s *Service) CreateLabelType(ctx context.Context, req *krillv1.CreateLabelTypeRequest) (*krillv1.CreateLabelTypeResponse, error) {
	in, err := validate(req.GetName(), req.GetColor(), req.GetDescription(), req.GetAttributes())
	if err != nil {
		return nil, err
	}
	lt, err := s.q.CreateLabelType(ctx, db.CreateLabelTypeParams{
		Name: in.name, Color: in.color, Description: in.description, Attributes: in.attrs,
	})
	if err != nil {
		if dup := duplicateName(err, in.name); dup != nil {
			return nil, dup
		}
		return nil, rpc.Internal(err, "create label type")
	}
	out, err := toProto(lt, 0, 0)
	if err != nil {
		return nil, rpc.Internal(err, "decode label type")
	}
	return &krillv1.CreateLabelTypeResponse{LabelType: out}, nil
}

func (s *Service) UpdateLabelType(ctx context.Context, req *krillv1.UpdateLabelTypeRequest) (*krillv1.UpdateLabelTypeResponse, error) {
	in, err := validate(req.GetName(), req.GetColor(), req.GetDescription(), req.GetAttributes())
	if err != nil {
		return nil, err
	}
	lt, err := s.q.UpdateLabelType(ctx, db.UpdateLabelTypeParams{
		ID: req.GetId(), Name: in.name, Color: in.color, Description: in.description, Attributes: in.attrs,
	})
	if err != nil {
		if dup := duplicateName(err, in.name); dup != nil {
			return nil, dup
		}
		return nil, rpc.DBError(err, "label type")
	}
	types, err := s.list(ctx, s.q)
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		if t.GetId() == lt.ID {
			return &krillv1.UpdateLabelTypeResponse{LabelType: t}, nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("label type not found"))
}

func (s *Service) DeleteLabelType(ctx context.Context, req *krillv1.DeleteLabelTypeRequest) (*krillv1.DeleteLabelTypeResponse, error) {
	tracks, err := s.q.CountTracksForLabelType(ctx, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "count tracks")
	}
	if tracks > 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("%d tracks use this label type; delete or relabel them first", tracks))
	}
	n, err := s.q.DeleteLabelType(ctx, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "delete label type")
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("label type not found"))
	}
	return &krillv1.DeleteLabelTypeResponse{}, nil
}

func (s *Service) ReorderLabelTypes(ctx context.Context, req *krillv1.ReorderLabelTypesRequest) (*krillv1.ReorderLabelTypesResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	for i, id := range req.GetIds() {
		//nolint:gosec // the number of label types fits in int32
		if err := q.SetLabelTypePosition(ctx, db.SetLabelTypePositionParams{ID: id, Position: int32(i)}); err != nil {
			return nil, rpc.Internal(err, "set position")
		}
	}
	types, err := s.list(ctx, q)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.ReorderLabelTypesResponse{LabelTypes: types}, nil
}
