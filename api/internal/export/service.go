package export

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
)

const (
	downloadExpiry = 12 * time.Hour
	maxNameLen     = 100
)

var unsafeFilename = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

var statusToProto = map[string]krillv1.ExportStatus{
	"queued":  krillv1.ExportStatus_EXPORT_STATUS_QUEUED,
	"running": krillv1.ExportStatus_EXPORT_STATUS_RUNNING,
	"ready":   krillv1.ExportStatus_EXPORT_STATUS_READY,
	"failed":  krillv1.ExportStatus_EXPORT_STATUS_FAILED,
}

type Service struct {
	krillv1connect.UnimplementedExportServiceHandler
	pool  *pgxpool.Pool
	q     *db.Queries
	store *storage.Store
	jobs  *river.Client[pgx.Tx]
}

func NewService(pool *pgxpool.Pool, store *storage.Store, jobs *river.Client[pgx.Tx]) *Service {
	return &Service{pool: pool, q: db.New(pool), store: store, jobs: jobs}
}

func (s *Service) PreviewExport(ctx context.Context, req *krillv1.PreviewExportRequest) (*krillv1.PreviewExportResponse, error) {
	opts, err := Normalize(req.GetOptions())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	plan, err := Load(ctx, s.q, opts)
	if err != nil {
		return nil, rpc.Internal(err, "plan export")
	}
	return &krillv1.PreviewExportResponse{Stats: plan.Stats}, nil
}

func (s *Service) CreateExport(ctx context.Context, req *krillv1.CreateExportRequest) (*krillv1.CreateExportResponse, error) {
	opts, err := Normalize(req.GetOptions())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		name = "krill-" + time.Now().Format("2006-01-02")
	}
	if len(name) > maxNameLen {
		return nil, rpc.Invalid("name must be at most %d characters", maxNameLen)
	}

	plan, err := Load(ctx, s.q, opts)
	if err != nil {
		return nil, rpc.Internal(err, "plan export")
	}
	if len(plan.Items) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("no frames to export; mark frames labeled or empty first"))
	}
	optsJSON, err := protojson.Marshal(opts)
	if err != nil {
		return nil, rpc.Internal(err, "encode options")
	}
	statsJSON, err := protojson.Marshal(plan.Stats)
	if err != nil {
		return nil, rpc.Internal(err, "encode stats")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	ds, err := s.q.WithTx(tx).CreateDataset(ctx, db.CreateDatasetParams{Name: name, Options: optsJSON, Stats: statsJSON})
	if err != nil {
		return nil, rpc.Internal(err, "create export")
	}
	if _, err := s.jobs.InsertTx(ctx, tx, Args{DatasetID: ds.ID}, nil); err != nil {
		return nil, rpc.Internal(err, "queue export")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}

	out, err := s.toProto(ctx, ds)
	if err != nil {
		return nil, err
	}
	return &krillv1.CreateExportResponse{Export: out}, nil
}

func (s *Service) ListExports(ctx context.Context, _ *krillv1.ListExportsRequest) (*krillv1.ListExportsResponse, error) {
	rows, err := s.q.ListDatasets(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list exports")
	}
	out := make([]*krillv1.Export, len(rows))
	for i, r := range rows {
		if out[i], err = s.toProto(ctx, r); err != nil {
			return nil, err
		}
	}
	return &krillv1.ListExportsResponse{Exports: out}, nil
}

func (s *Service) DeleteExport(ctx context.Context, req *krillv1.DeleteExportRequest) (*krillv1.DeleteExportResponse, error) {
	n, err := s.q.DeleteDataset(ctx, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "delete export")
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("export not found"))
	}
	if err := s.store.RemovePrefix(ctx, storage.DatasetPrefix(req.GetId())); err != nil {
		slog.Error("remove export objects", "dataset_id", req.GetId(), "err", err)
	}
	return &krillv1.DeleteExportResponse{}, nil
}

func (s *Service) toProto(ctx context.Context, d db.Dataset) (*krillv1.Export, error) {
	opts := &krillv1.ExportOptions{}
	if err := protojson.Unmarshal(d.Options, opts); err != nil {
		return nil, rpc.Internal(err, fmt.Sprintf("decode options of export %d", d.ID))
	}
	stats := &krillv1.ExportStats{}
	if err := protojson.Unmarshal(d.Stats, stats); err != nil {
		return nil, rpc.Internal(err, fmt.Sprintf("decode stats of export %d", d.ID))
	}
	out := &krillv1.Export{
		Id:        d.ID,
		Name:      d.Name,
		Status:    statusToProto[d.Status],
		Error:     d.Error,
		Progress:  d.Progress,
		Options:   opts,
		Stats:     stats,
		SizeBytes: d.SizeBytes,
		CreatedAt: timestamppb.New(d.CreatedAt.Time),
	}
	if d.FinishedAt.Valid {
		out.FinishedAt = timestamppb.New(d.FinishedAt.Time)
	}
	if d.Status == "ready" {
		filename := strings.Trim(unsafeFilename.ReplaceAllString(d.Name, "-"), "-") + ".zip"
		u, err := s.store.PresignGet(ctx, storage.DatasetKey(d.ID), downloadExpiry, filename)
		if err != nil {
			return nil, rpc.Internal(err, "sign download url")
		}
		out.DownloadUrl = u
	}
	return out, nil
}
