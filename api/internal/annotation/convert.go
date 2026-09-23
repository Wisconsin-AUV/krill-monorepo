package annotation

import (
	"fmt"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/taxonomy"
)

var frameStatusToProto = map[string]krillv1.FrameStatus{
	"unlabeled": krillv1.FrameStatus_FRAME_STATUS_UNLABELED,
	"labeled":   krillv1.FrameStatus_FRAME_STATUS_LABELED,
	"empty":     krillv1.FrameStatus_FRAME_STATUS_EMPTY,
}

var sourceToProto = map[string]krillv1.AnnotationSource{
	"human": krillv1.AnnotationSource_ANNOTATION_SOURCE_HUMAN,
	"sam":   krillv1.AnnotationSource_ANNOTATION_SOURCE_SAM,
	"yolo":  krillv1.AnnotationSource_ANNOTATION_SOURCE_YOLO,
	"cv":    krillv1.AnnotationSource_ANNOTATION_SOURCE_CV,
}

var statusToProto = map[string]krillv1.AnnotationStatus{
	"proposed": krillv1.AnnotationStatus_ANNOTATION_STATUS_PROPOSED,
	"verified": krillv1.AnnotationStatus_ANNOTATION_STATUS_VERIFIED,
	"rejected": krillv1.AnnotationStatus_ANNOTATION_STATUS_REJECTED,
}

func FrameStatus(s string) krillv1.FrameStatus {
	return frameStatusToProto[s]
}

func frameStatusFromProto(s krillv1.FrameStatus) (string, bool) {
	for k, v := range frameStatusToProto {
		if v == s {
			return k, true
		}
	}
	return "", false
}

func Track(t db.Track) (*krillv1.Track, error) {
	values, err := taxonomy.ParseValues(t.Attributes)
	if err != nil {
		return nil, fmt.Errorf("track %d: %w", t.ID, err)
	}
	return &krillv1.Track{Id: t.ID, ClipId: t.ClipID, LabelTypeId: t.LabelTypeID, Attributes: values}, nil
}

func Annotation(a db.Annotation) *krillv1.Annotation {
	return &krillv1.Annotation{
		Id:      a.ID,
		TrackId: a.TrackID,
		FrameId: a.FrameID,
		Box:     &krillv1.Box{X: a.X, Y: a.Y, Width: a.Width, Height: a.Height},
		Source:  sourceToProto[a.Source],
		Status:  statusToProto[a.Status],
	}
}
