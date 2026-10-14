package pipeline

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

var trainingStatuses = map[string]krillv1.TrainingStatus{
	"queued":     krillv1.TrainingStatus_TRAINING_STATUS_QUEUED,
	"preparing":  krillv1.TrainingStatus_TRAINING_STATUS_PREPARING,
	"training":   krillv1.TrainingStatus_TRAINING_STATUS_TRAINING,
	"evaluating": krillv1.TrainingStatus_TRAINING_STATUS_EVALUATING,
	"succeeded":  krillv1.TrainingStatus_TRAINING_STATUS_SUCCEEDED,
	"failed":     krillv1.TrainingStatus_TRAINING_STATUS_FAILED,
	"cancelled":  krillv1.TrainingStatus_TRAINING_STATUS_CANCELLED,
}

var triggers = map[string]krillv1.TrainingTrigger{
	triggerManual:   krillv1.TrainingTrigger_TRAINING_TRIGGER_MANUAL,
	triggerSchedule: krillv1.TrainingTrigger_TRAINING_TRIGGER_SCHEDULE,
}

var prelabelStatuses = map[string]krillv1.PrelabelStatus{
	"queued":    krillv1.PrelabelStatus_PRELABEL_STATUS_QUEUED,
	"running":   krillv1.PrelabelStatus_PRELABEL_STATUS_RUNNING,
	"succeeded": krillv1.PrelabelStatus_PRELABEL_STATUS_SUCCEEDED,
	"failed":    krillv1.PrelabelStatus_PRELABEL_STATUS_FAILED,
	"cancelled": krillv1.PrelabelStatus_PRELABEL_STATUS_CANCELLED,
}

var prelabelSources = map[string]krillv1.PrelabelSource{
	sourceYolo: krillv1.PrelabelSource_PRELABEL_SOURCE_YOLO,
	sourceCV:   krillv1.PrelabelSource_PRELABEL_SOURCE_CV,
}

const (
	triggerManual   = "manual"
	triggerSchedule = "schedule"
	sourceYolo      = "yolo"
	sourceCV        = "cv"
)

func ts(t pgtype.Timestamptz) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}

// unmarshal decodes protobuf JSON, leaving m empty for NULL.
func unmarshal(raw []byte, m proto.Message) error {
	if len(raw) == 0 {
		return nil
	}
	return protojson.Unmarshal(raw, m)
}

func evalMetrics(raw []byte) (*krillv1.EvalMetrics, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	m := &krillv1.EvalMetrics{}
	if err := protojson.Unmarshal(raw, m); err != nil {
		return nil, err
	}
	return m, nil
}

func trainingRun(r db.TrainingRun, createdBy string, withLog bool) (*krillv1.TrainingRun, error) {
	out := &krillv1.TrainingRun{
		Id:            r.ID,
		Status:        trainingStatuses[r.Status],
		Trigger:       triggers[r.Trigger],
		CreatedBy:     createdBy,
		Config:        &krillv1.TrainConfig{},
		Dataset:       &krillv1.ExportStats{},
		Epoch:         r.Epoch,
		BaselineRunId: r.BaselineRunID.Int64,
		Pinned:        r.Pinned,
		Error:         r.Error,
		CreatedAt:     ts(r.CreatedAt),
		StartedAt:     ts(r.StartedAt),
		FinishedAt:    ts(r.FinishedAt),
		WeightsBytes:  r.WeightsBytes,
		Device:        r.Device,
		VerifiedBoxes: r.VerifiedBoxes,
	}
	if withLog {
		out.Log = r.Log
	}
	err := unmarshal(r.Config, out.Config)
	if err == nil {
		err = unmarshal(r.Dataset, out.Dataset)
	}
	if err == nil {
		out.History, err = history(r.History)
	}
	for _, m := range []struct {
		raw []byte
		dst **krillv1.EvalMetrics
	}{
		{r.ValMetrics, &out.Val},
		{r.TestMetrics, &out.Test},
		{r.BaselineValMetrics, &out.BaselineVal},
		{r.BaselineTestMetrics, &out.BaselineTest},
	} {
		if err == nil {
			*m.dst, err = evalMetrics(m.raw)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("decode training run %d: %w", r.ID, err)
	}
	return out, nil
}

func history(raw []byte) ([]*krillv1.EpochMetrics, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	out := make([]*krillv1.EpochMetrics, len(items))
	for i, item := range items {
		out[i] = &krillv1.EpochMetrics{}
		if err := protojson.Unmarshal(item, out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func prelabelRun(r db.PrelabelRun, createdBy string) *krillv1.PrelabelRun {
	return &krillv1.PrelabelRun{
		Id:            r.ID,
		Source:        prelabelSources[r.Source],
		TrainingRunId: r.TrainingRunID.Int64,
		Status:        prelabelStatuses[r.Status],
		ClipsTotal:    r.ClipsTotal,
		ClipsDone:     r.ClipsDone,
		Frames:        r.Frames,
		Proposals:     r.Proposals,
		Error:         r.Error,
		CreatedBy:     createdBy,
		CreatedAt:     ts(r.CreatedAt),
		FinishedAt:    ts(r.FinishedAt),
	}
}
