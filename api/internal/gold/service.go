package gold

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/taxonomy"
	"github.com/wauv/krill/api/internal/video"
)

const (
	// checkEvery is how many frames a labeler finishes between gold checks.
	checkEvery     = 100
	maxAnswerBoxes = 200
)

var errAnswered = connect.NewError(connect.CodeFailedPrecondition, errors.New("you already answered this gold frame"))

type Service struct {
	krillv1connect.UnimplementedGoldServiceHandler
	q       *db.Queries
	present *video.Presenter
}

func NewService(pool *pgxpool.Pool, store *storage.Store) *Service {
	return &Service{q: db.New(pool), present: video.NewPresenter(store)}
}

// Due returns a gold frame for the labeler to answer, or 0 when they are not
// due a check or have answered every one they can.
func Due(ctx context.Context, q *db.Queries, userID uuid.UUID) (int64, error) {
	n, err := q.FramesSinceGoldCheck(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count frames since gold check: %w", err)
	}
	if n < checkEvery {
		return 0, nil
	}
	id, err := q.NextGoldFrame(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("pick gold frame: %w", err)
	}
	return id, nil
}

func (s *Service) SetGoldFrame(ctx context.Context, req *krillv1.SetGoldFrameRequest) (*krillv1.SetGoldFrameResponse, error) {
	if !req.GetGold() {
		if err := s.q.DeleteGoldFrame(ctx, req.GetFrameId()); err != nil {
			return nil, rpc.Internal(err, "delete gold frame")
		}
		return &krillv1.SetGoldFrameResponse{}, nil
	}
	f, err := s.q.GetFrame(ctx, req.GetFrameId())
	if err != nil {
		return nil, rpc.DBError(err, "frame")
	}
	if f.Status == "unlabeled" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("finish labeling the frame first"))
	}
	rows, err := s.q.ListFrameVerifiedBoxes(ctx, f.ID)
	if err != nil {
		return nil, rpc.Internal(err, "list boxes")
	}
	boxes := make([]Box, len(rows))
	for i, r := range rows {
		values, err := taxonomy.ParseValues(r.Attributes)
		if err != nil {
			return nil, rpc.Internal(err, "decode attributes")
		}
		boxes[i] = Box{LabelTypeID: r.LabelTypeID, Attributes: values, X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
	}
	raw, err := json.Marshal(boxes)
	if err != nil {
		return nil, rpc.Internal(err, "encode boxes")
	}
	err = s.q.InsertGoldFrame(ctx, db.InsertGoldFrameParams{FrameID: f.ID, Boxes: raw, CreatedBy: auth.CallerID(ctx)})
	if err != nil {
		return nil, rpc.Internal(err, "save gold frame")
	}
	return &krillv1.SetGoldFrameResponse{}, nil
}

func (s *Service) GetGoldCheck(ctx context.Context, req *krillv1.GetGoldCheckRequest) (*krillv1.GetGoldCheckResponse, error) {
	g, err := s.q.GetGoldFrame(ctx, req.GetFrameId())
	if err != nil {
		return nil, rpc.DBError(err, "gold frame")
	}
	done, err := s.q.HasGoldAttempt(ctx, db.HasGoldAttemptParams{FrameID: g.FrameID, UserID: auth.CallerID(ctx)})
	if err != nil {
		return nil, rpc.Internal(err, "load attempt")
	}
	if done {
		return nil, errAnswered
	}
	url, err := s.present.FrameURL(ctx, g.VideoID, g.Idx)
	if err != nil {
		return nil, rpc.Internal(err, "sign frame url")
	}
	return &krillv1.GetGoldCheckResponse{FrameId: g.FrameID, Url: url, Width: g.Width, Height: g.Height}, nil
}

func (s *Service) answer(ctx context.Context, in []*krillv1.GoldBox) ([]Box, error) {
	if len(in) > maxAnswerBoxes {
		return nil, rpc.Invalid("at most %d boxes", maxAnswerBoxes)
	}
	types, err := s.q.ListLabelTypes(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list label types")
	}
	defs := make(map[int64][]taxonomy.Attribute, len(types))
	for _, t := range types {
		if defs[t.LabelType.ID], err = taxonomy.ParseAttributes(t.LabelType.Attributes); err != nil {
			return nil, rpc.Internal(err, "decode label type")
		}
	}
	out := make([]Box, len(in))
	for i, b := range in {
		attrs, ok := defs[b.GetLabelTypeId()]
		if !ok {
			return nil, rpc.Invalid("label type %d not found", b.GetLabelTypeId())
		}
		if err := taxonomy.CheckValues(attrs, b.GetAttributes()); err != nil {
			return nil, rpc.Invalid("%s", err)
		}
		x, y, w, h, err := annotation.ClampBox(b.GetBox())
		if err != nil {
			return nil, rpc.Invalid("%s", err)
		}
		values := b.GetAttributes()
		if values == nil {
			values = map[string]string{}
		}
		out[i] = Box{LabelTypeID: b.GetLabelTypeId(), Attributes: values, X: x, Y: y, Width: w, Height: h}
	}
	return out, nil
}

func (s *Service) SubmitGoldCheck(ctx context.Context, req *krillv1.SubmitGoldCheckRequest) (*krillv1.SubmitGoldCheckResponse, error) {
	g, err := s.q.GetGoldFrame(ctx, req.GetFrameId())
	if err != nil {
		return nil, rpc.DBError(err, "gold frame")
	}
	var reference []Box
	if err := json.Unmarshal(g.Boxes, &reference); err != nil {
		return nil, rpc.Internal(err, "decode gold boxes")
	}
	answer, err := s.answer(ctx, req.GetBoxes())
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(answer)
	if err != nil {
		return nil, rpc.Internal(err, "encode boxes")
	}

	res := Score(reference, answer)
	n, err := s.q.InsertGoldAttempt(ctx, db.InsertGoldAttemptParams{
		FrameID:        g.FrameID,
		UserID:         auth.CallerID(ctx),
		Boxes:          raw,
		ReferenceCount: small(len(reference)),
		AnswerCount:    small(len(answer)),
		Matched:        small(len(res.Matches)),
		Correct:        small(res.Correct),
		IouSum:         res.IoUSum,
		Score:          res.Score,
	})
	if err != nil {
		return nil, rpc.Internal(err, "save attempt")
	}
	if n == 0 {
		return nil, errAnswered
	}

	out := &krillv1.GoldResult{
		Reference: boxesToProto(reference),
		Answer:    boxesToProto(answer),
		Matches:   make([]*krillv1.GoldMatch, len(res.Matches)),
		Score:     res.Score,
	}
	for i, m := range res.Matches {
		out.Matches[i] = &krillv1.GoldMatch{Reference: small(m.Reference), Answer: small(m.Answer), Iou: m.IoU, Correct: m.Correct}
	}
	return &krillv1.SubmitGoldCheckResponse{Result: out}, nil
}

func (s *Service) GetGoldStats(ctx context.Context, _ *krillv1.GetGoldStatsRequest) (*krillv1.GetGoldStatsResponse, error) {
	labelers, err := s.q.ListGoldLabelers(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list labelers")
	}
	frames, err := s.q.ListGoldFrames(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list gold frames")
	}

	out := &krillv1.GetGoldStatsResponse{
		Labelers: make([]*krillv1.GoldLabeler, len(labelers)),
		Frames:   make([]*krillv1.GoldFrame, len(frames)),
	}
	for i, l := range labelers {
		g := &krillv1.GoldLabeler{
			User:     auth.PublicProfile(l.ID, l.Name, l.Username, l.Role, l.CreatedAt),
			Attempts: l.Attempts,
			Accuracy: 1,
		}
		if l.Scored > 0 {
			g.Accuracy = float64(l.Correct) / float64(l.Scored)
		}
		if l.Matched > 0 {
			g.MeanIou = l.IouSum / float64(l.Matched)
		}
		out.Labelers[i] = g
	}
	slices.SortStableFunc(out.Labelers, func(a, b *krillv1.GoldLabeler) int {
		return cmp.Or(cmp.Compare(a.GetAccuracy(), b.GetAccuracy()), cmp.Compare(a.GetUser().GetName(), b.GetUser().GetName()))
	})

	for i, f := range frames {
		url, err := s.present.FrameURL(ctx, f.VideoID, f.Idx)
		if err != nil {
			return nil, rpc.Internal(err, "sign frame url")
		}
		out.Frames[i] = &krillv1.GoldFrame{
			FrameId:      f.FrameID,
			ClipId:       f.ClipID,
			Index:        f.Idx,
			VideoName:    f.VideoName,
			ThumbnailUrl: url,
			BoxCount:     f.BoxCount,
			Attempts:     f.Attempts,
			MeanScore:    f.MeanScore,
		}
	}
	return out, nil
}

func small(n int) int32 {
	return int32(n) //nolint:gosec // box counts and indexes are far below int32
}

func boxesToProto(boxes []Box) []*krillv1.GoldBox {
	out := make([]*krillv1.GoldBox, len(boxes))
	for i, b := range boxes {
		out[i] = &krillv1.GoldBox{
			LabelTypeId: b.LabelTypeID,
			Attributes:  b.Attributes,
			Box:         &krillv1.Box{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height},
		}
	}
	return out
}
