package storagescan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

// shownOrphans is how many of the largest orphans the report lists.
const shownOrphans = 50

var errBusy = connect.NewError(connect.CodeFailedPrecondition, errors.New("a storage scan or cleanup is already running"))

type Service struct {
	krillv1connect.UnimplementedStorageServiceHandler
	pool *pgxpool.Pool
	q    *db.Queries
	jobs *river.Client[pgx.Tx]
}

func NewService(pool *pgxpool.Pool, jobs *river.Client[pgx.Tx]) *Service {
	return &Service{pool: pool, q: db.New(pool), jobs: jobs}
}

var jobStatuses = map[string]krillv1.JobStatus{
	"queued":  krillv1.JobStatus_JOB_STATUS_QUEUED,
	"running": krillv1.JobStatus_JOB_STATUS_RUNNING,
	"done":    krillv1.JobStatus_JOB_STATUS_DONE,
	"failed":  krillv1.JobStatus_JOB_STATUS_FAILED,
}

var kindsToProto = map[Kind]krillv1.ObjectKind{
	KindSource:       krillv1.ObjectKind_OBJECT_KIND_SOURCE,
	KindFrame:        krillv1.ObjectKind_OBJECT_KIND_FRAME,
	KindEmbedding:    krillv1.ObjectKind_OBJECT_KIND_EMBEDDING,
	KindExport:       krillv1.ObjectKind_OBJECT_KIND_EXPORT,
	KindLabelExample: krillv1.ObjectKind_OBJECT_KIND_LABEL_EXAMPLE,
	KindBackup:       krillv1.ObjectKind_OBJECT_KIND_BACKUP,
	KindUpload:       krillv1.ObjectKind_OBJECT_KIND_UPLOAD,
	KindOther:        krillv1.ObjectKind_OBJECT_KIND_OTHER,
}

func ts(t pgtype.Timestamptz) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}

func bytes(n int64) uint64 {
	return uint64(max(n, 0))
}

func (s *Service) GetStorageReport(ctx context.Context, _ *krillv1.GetStorageReportRequest) (*krillv1.GetStorageReportResponse, error) {
	latest, err := s.q.GetLatestStorageScanID(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return &krillv1.GetStorageReportResponse{}, nil
	}
	if err != nil {
		return nil, rpc.Internal(err, "find latest scan")
	}
	out := &krillv1.GetStorageReportResponse{}
	if out.Scan, err = s.scan(ctx, latest); err != nil {
		return nil, err
	}
	shown := out.Scan
	if out.Scan.GetStatus() != krillv1.JobStatus_JOB_STATUS_DONE {
		id, err := s.q.GetLatestFinishedStorageScanID(ctx)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return out, nil
		case err != nil:
			return nil, rpc.Internal(err, "find previous scan")
		}
		if out.Previous, err = s.scan(ctx, id); err != nil {
			return nil, err
		}
		shown = out.Previous
	}
	orphans, err := s.q.ListLargestStorageOrphans(ctx, db.ListLargestStorageOrphansParams{ScanID: shown.GetId(), Limit: shownOrphans})
	if err != nil {
		return nil, rpc.Internal(err, "list orphans")
	}
	for _, o := range orphans {
		out.Orphans = append(out.Orphans, &krillv1.Orphan{
			Key:        o.Key,
			Kind:       kindsToProto[Kind(o.Kind)],
			Bytes:      bytes(o.SizeBytes),
			ModifiedAt: ts(o.ModifiedAt),
		})
	}
	return out, nil
}

func (s *Service) scan(ctx context.Context, id int64) (*krillv1.StorageScan, error) {
	r, err := s.q.GetStorageScan(ctx, id)
	if err != nil {
		return nil, rpc.DBError(err, "scan")
	}
	out := &krillv1.StorageScan{
		Id:          r.ID,
		Status:      jobStatuses[r.Status],
		Error:       r.Error,
		RequestedBy: r.CreatedByName.String,
		CreatedAt:   ts(r.CreatedAt),
		FinishedAt:  ts(r.FinishedAt),
	}
	if r.CleanupStatus != "" {
		out.Cleanup = &krillv1.StorageCleanup{
			Status:         jobStatuses[r.CleanupStatus],
			RequestedBy:    r.CleanupByName.String,
			FinishedAt:     ts(r.CleanupFinishedAt),
			DeletedObjects: r.DeletedObjects,
			DeletedBytes:   bytes(r.DeletedBytes),
			SkippedObjects: r.SkippedObjects,
			Error:          r.CleanupError,
		}
	}
	if r.Status != "done" {
		return out, nil
	}
	var cats []category
	if err := json.Unmarshal(r.Categories, &cats); err != nil {
		return nil, rpc.Internal(err, "decode categories")
	}
	for _, c := range cats {
		out.Categories = append(out.Categories, &krillv1.ObjectCategory{
			Kind:           kindsToProto[c.Kind],
			Bytes:          bytes(c.Bytes),
			Objects:        c.Objects,
			OrphanBytes:    bytes(c.OrphanBytes),
			OrphanObjects:  c.OrphanObjects,
			MissingObjects: c.Missing,
			MissingSamples: c.MissingSamples,
		})
	}
	videos, err := s.q.ListStorageScanVideos(ctx, id)
	if err != nil {
		return nil, rpc.Internal(err, "list scanned videos")
	}
	for _, v := range videos {
		out.Videos = append(out.Videos, &krillv1.VideoUsage{
			VideoId:        v.VideoID,
			Name:           v.VideoName.String,
			FrameBytes:     bytes(v.FrameBytes),
			Frames:         v.Frames,
			EmbeddingBytes: bytes(v.EmbeddingBytes),
			Embeddings:     v.Embeddings,
			SourceBytes:    bytes(v.SourceBytes),
			OrphanBytes:    bytes(v.OrphanBytes),
		})
	}
	return out, nil
}

func isBusy(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Service) ScanStorage(ctx context.Context, _ *krillv1.ScanStorageRequest) (*krillv1.ScanStorageResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	caller := auth.CallerID(ctx)
	id, err := q.CreateStorageScan(ctx, pgtype.UUID{Bytes: caller, Valid: true})
	if isBusy(err) {
		return nil, errBusy
	}
	if err != nil {
		return nil, rpc.Internal(err, "create scan")
	}
	if _, err := s.jobs.InsertTx(ctx, tx, ScanArgs{ScanID: id}, nil); err != nil {
		return nil, rpc.Internal(err, "queue scan")
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.StorageScan,
		TargetType: audit.TargetScan,
		TargetID:   strconv.FormatInt(id, 10),
		TargetName: fmt.Sprintf("Scan %d", id),
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	scan, err := s.scan(ctx, id)
	if err != nil {
		return nil, err
	}
	return &krillv1.ScanStorageResponse{Scan: scan}, nil
}

func (s *Service) CleanUpStorage(ctx context.Context, req *krillv1.CleanUpStorageRequest) (*krillv1.CleanUpStorageResponse, error) {
	scan, err := s.scan(ctx, req.GetScanId())
	if err != nil {
		return nil, err
	}
	var objects int64
	var size uint64
	for _, c := range scan.GetCategories() {
		objects += c.GetOrphanObjects()
		size += c.GetOrphanBytes()
	}
	if objects == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the scan found nothing to clean up"))
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	n, err := q.RequestStorageCleanup(ctx, db.RequestStorageCleanupParams{
		ID: scan.GetId(), CleanupBy: pgtype.UUID{Bytes: auth.CallerID(ctx), Valid: true},
	})
	if isBusy(err) {
		return nil, errBusy
	}
	if err != nil {
		return nil, rpc.Internal(err, "request cleanup")
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("only the latest finished scan can be cleaned up, once"))
	}
	if _, err := s.jobs.InsertTx(ctx, tx, CleanupArgs{ScanID: scan.GetId()}, nil); err != nil {
		return nil, rpc.Internal(err, "queue cleanup")
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.StorageCleanup,
		TargetType: audit.TargetScan,
		TargetID:   strconv.FormatInt(scan.GetId(), 10),
		TargetName: fmt.Sprintf("Scan %d", scan.GetId()),
		Details:    map[string]any{"objects": objects, "bytes": size},
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	if scan, err = s.scan(ctx, scan.GetId()); err != nil {
		return nil, err
	}
	return &krillv1.CleanUpStorageResponse{Scan: scan}, nil
}
