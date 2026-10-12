package annotation

import (
	"context"
	"strconv"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/taxonomy"
)

const (
	defaultPageSize = 60
	maxPageSize     = 200
)

// parsePage returns the page size and the track id the page starts after.
func parsePage(size int32, token string) (int32, int64, error) {
	switch {
	case size < 0:
		return 0, 0, rpc.Invalid("page size must not be negative")
	case size == 0:
		size = defaultPageSize
	case size > maxPageSize:
		size = maxPageSize
	}
	if token == "" {
		return size, 0, nil
	}
	after, err := strconv.ParseInt(token, 10, 64)
	if err != nil || after <= 0 {
		return 0, 0, rpc.Invalid("bad page token")
	}
	return size, after, nil
}

func (s *Service) ListTracksByType(ctx context.Context, req *krillv1.ListTracksByTypeRequest) (*krillv1.ListTracksByTypeResponse, error) {
	size, after, err := parsePage(req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, err
	}
	if _, err := s.q.GetLabelType(ctx, req.GetLabelTypeId()); err != nil {
		return nil, rpc.DBError(err, "label type")
	}
	rows, err := s.q.ListTypeTracks(ctx, db.ListTypeTracksParams{
		LabelTypeID: req.GetLabelTypeId(),
		VideoID:     req.GetVideoId(),
		AfterID:     after,
		MissingOnly: req.GetMissingAttributes(),
		Lim:         size + 1,
	})
	if err != nil {
		return nil, rpc.Internal(err, "list tracks")
	}

	out := &krillv1.ListTracksByTypeResponse{}
	if len(rows) > int(size) {
		rows = rows[:size]
		out.NextPageToken = strconv.FormatInt(rows[len(rows)-1].ID, 10)
	}
	for _, r := range rows {
		values, err := taxonomy.ParseValues(r.Attributes)
		if err != nil {
			return nil, rpc.Internal(err, "decode track")
		}
		url, err := s.present.FrameURL(ctx, r.VideoID, r.FrameIdx)
		if err != nil {
			return nil, rpc.Internal(err, "sign frame url")
		}
		out.Tracks = append(out.Tracks, &krillv1.TrackCrop{
			Track:      &krillv1.Track{Id: r.ID, ClipId: r.ClipID, LabelTypeId: r.LabelTypeID, Attributes: values},
			VideoId:    r.VideoID,
			FrameId:    r.FrameID,
			FrameIndex: r.FrameIdx,
			Box:        &krillv1.Box{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height},
			FrameUrl:   url,
		})
	}
	return out, nil
}
