package taxonomy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
)

const (
	maxTitleLen       = 60
	maxDescriptionLen = 500
	maxGuidelineLen   = 4000
	maxCaptionLen     = 200
	uploadExpiry      = 15 * time.Minute
	// Matches the clip page's frame URLs so a long session keeps working.
	urlExpiry = 12 * time.Hour
)

var exampleName = regexp.MustCompile(`^[0-9a-f]{32}$`)

type Service struct {
	krillv1connect.UnimplementedLabelServiceHandler
	pool  *pgxpool.Pool
	q     *db.Queries
	store *storage.Store
}

func NewService(pool *pgxpool.Pool, store *storage.Store) *Service {
	return &Service{pool: pool, q: db.New(pool), store: store}
}

func toProto(lt db.LabelType, tracks, boxes int32) (*krillv1.LabelType, error) {
	attrs, err := ParseAttributes(lt.Attributes)
	if err != nil {
		return nil, err
	}
	out := &krillv1.LabelType{
		Id:          lt.ID,
		Name:        lt.Name,
		Title:       lt.Title,
		Color:       lt.Color,
		Description: lt.Description,
		Guideline:   lt.Guideline,
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
	name, title, color, description, guideline string
	attrs                                      []byte
}

type typeRequest interface {
	GetName() string
	GetTitle() string
	GetColor() string
	GetDescription() string
	GetGuideline() string
	GetAttributes() []*krillv1.LabelAttribute
}

func validate(req typeRequest) (typeInput, error) {
	name, color, clean, err := NormalizeType(req.GetName(), req.GetColor(), fromProto(req.GetAttributes()))
	if err != nil {
		return typeInput{}, rpc.Invalid("%s", err)
	}
	in := typeInput{
		name:        name,
		title:       strings.TrimSpace(req.GetTitle()),
		color:       color,
		description: strings.TrimSpace(req.GetDescription()),
		guideline:   strings.TrimSpace(req.GetGuideline()),
	}
	for _, f := range []struct {
		field, value string
		max          int
	}{
		{"title", in.title, maxTitleLen},
		{"description", in.description, maxDescriptionLen},
		{"guideline", in.guideline, maxGuidelineLen},
	} {
		if len(f.value) > f.max {
			return typeInput{}, rpc.Invalid("%s must be at most %d characters", f.field, f.max)
		}
	}
	if in.attrs, err = json.Marshal(clean); err != nil {
		return typeInput{}, rpc.Internal(err, "encode attributes")
	}
	return in, nil
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
	examples, err := q.ListLabelExamples(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list label examples")
	}
	byType := map[int64][]*krillv1.LabelExample{}
	for _, e := range examples {
		byType[e.LabelTypeID] = append(byType[e.LabelTypeID], s.exampleProto(ctx, e))
	}
	out := make([]*krillv1.LabelType, len(rows))
	for i, r := range rows {
		if out[i], err = toProto(r.LabelType, r.TrackCount, r.BoxCount); err != nil {
			return nil, rpc.Internal(err, "decode label type")
		}
		out[i].Examples = byType[r.LabelType.ID]
	}
	return out, nil
}

func (s *Service) exampleProto(ctx context.Context, e db.LabelExample) *krillv1.LabelExample {
	url, err := s.store.PresignGet(ctx, e.ObjectKey, urlExpiry, "")
	if err != nil {
		slog.WarnContext(ctx, "presign label example", "id", e.ID, "err", err)
	}
	return &krillv1.LabelExample{Id: e.ID, Url: url, Caption: e.Caption}
}

func (s *Service) CreateLabelType(ctx context.Context, req *krillv1.CreateLabelTypeRequest) (*krillv1.CreateLabelTypeResponse, error) {
	in, err := validate(req)
	if err != nil {
		return nil, err
	}
	lt, err := s.q.CreateLabelType(ctx, db.CreateLabelTypeParams{
		Name: in.name, Title: in.title, Color: in.color, Description: in.description,
		Guideline: in.guideline, Attributes: in.attrs,
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
	in, err := validate(req)
	if err != nil {
		return nil, err
	}
	lt, err := s.q.UpdateLabelType(ctx, db.UpdateLabelTypeParams{
		ID: req.GetId(), Name: in.name, Title: in.title, Color: in.color, Description: in.description,
		Guideline: in.guideline, Attributes: in.attrs,
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
	if err := s.store.RemovePrefix(ctx, storage.LabelExamplesPrefix(req.GetId())); err != nil {
		slog.WarnContext(ctx, "remove label examples", "label_type_id", req.GetId(), "err", err)
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

func (s *Service) CreateLabelExampleUpload(ctx context.Context, req *krillv1.CreateLabelExampleUploadRequest) (*krillv1.CreateLabelExampleUploadResponse, error) {
	if _, err := s.q.GetLabelType(ctx, req.GetLabelTypeId()); err != nil {
		return nil, rpc.DBError(err, "label type")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, rpc.Internal(err, "generate key")
	}
	key := storage.LabelExamplesPrefix(req.GetLabelTypeId()) + hex.EncodeToString(b)
	url, err := s.store.PresignPut(ctx, key, uploadExpiry)
	if err != nil {
		return nil, rpc.Internal(err, "presign upload")
	}
	return &krillv1.CreateLabelExampleUploadResponse{Key: key, UploadUrl: url}, nil
}

// ValidExampleKey reports whether key is one CreateLabelExampleUpload could
// have issued for the type. Every labeler gets a read URL for the key, so
// accepting any key would expose videos and datasets.
func ValidExampleKey(labelTypeID int64, key string) bool {
	name, ok := strings.CutPrefix(key, storage.LabelExamplesPrefix(labelTypeID))
	return ok && exampleName.MatchString(name)
}

func (s *Service) AddLabelExample(ctx context.Context, req *krillv1.AddLabelExampleRequest) (*krillv1.AddLabelExampleResponse, error) {
	if !ValidExampleKey(req.GetLabelTypeId(), req.GetKey()) {
		return nil, rpc.Invalid("key was not issued for this label type")
	}
	caption := strings.TrimSpace(req.GetCaption())
	if len(caption) > maxCaptionLen {
		return nil, rpc.Invalid("caption must be at most %d characters", maxCaptionLen)
	}
	exists, err := s.store.Exists(ctx, req.GetKey())
	if err != nil {
		return nil, rpc.Internal(err, "check upload")
	}
	if !exists {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("example image has not been uploaded"))
	}
	e, err := s.q.CreateLabelExample(ctx, db.CreateLabelExampleParams{
		LabelTypeID: req.GetLabelTypeId(), ObjectKey: req.GetKey(), Caption: caption,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("example was already added"))
		}
		return nil, rpc.Internal(err, "add label example")
	}
	return &krillv1.AddLabelExampleResponse{Example: s.exampleProto(ctx, e)}, nil
}

func (s *Service) DeleteLabelExample(ctx context.Context, req *krillv1.DeleteLabelExampleRequest) (*krillv1.DeleteLabelExampleResponse, error) {
	e, err := s.q.DeleteLabelExample(ctx, req.GetId())
	if err != nil {
		return nil, rpc.DBError(err, "label example")
	}
	if err := s.store.Remove(ctx, e.ObjectKey); err != nil {
		slog.WarnContext(ctx, "remove label example", "id", e.ID, "err", err)
	}
	return &krillv1.DeleteLabelExampleResponse{}, nil
}
