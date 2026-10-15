package backup

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
)

const (
	// A dump holds password hashes and sessions, so its link is short-lived.
	downloadExpiry = 5 * time.Minute
	// Tables estimated below this are counted exactly.
	exactCountBelow = 100_000
)

type Service struct {
	krillv1connect.UnimplementedDatabaseServiceHandler
	pool     *pgxpool.Pool
	q        *db.Queries
	store    *storage.Store
	jobs     *river.Client[pgx.Tx]
	settings func() config.Config
}

func NewService(pool *pgxpool.Pool, store *storage.Store, jobs *river.Client[pgx.Tx], settings func() config.Config) *Service {
	return &Service{pool: pool, q: db.New(pool), store: store, jobs: jobs, settings: settings}
}

func filename(key string) string {
	return path.Base(key)
}

func ts(t pgtype.Timestamptz) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}

var statuses = map[string]krillv1.JobStatus{
	"queued":  krillv1.JobStatus_JOB_STATUS_QUEUED,
	"running": krillv1.JobStatus_JOB_STATUS_RUNNING,
	"done":    krillv1.JobStatus_JOB_STATUS_DONE,
	"failed":  krillv1.JobStatus_JOB_STATUS_FAILED,
}

func (s *Service) GetDatabase(ctx context.Context, _ *krillv1.GetDatabaseRequest) (*krillv1.GetDatabaseResponse, error) {
	out := &krillv1.GetDatabaseResponse{}
	size, err := s.q.DatabaseSize(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "database size")
	}
	out.SizeBytes = uint64(max(size, 0))
	if out.ServerVersion, err = s.q.ServerVersion(ctx); err != nil {
		return nil, rpc.Internal(err, "server version")
	}
	if out.SchemaVersion, out.LatestSchemaVersion, err = db.SchemaVersion(ctx, s.pool); err != nil {
		return nil, rpc.Internal(err, "schema version")
	}
	migrator, err := rivermigrate.New(riverpgxv5.New(s.pool), nil)
	if err != nil {
		return nil, rpc.Internal(err, "river migrator")
	}
	applied, err := migrator.ExistingVersions(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "river version")
	}
	if len(applied) > 0 {
		out.RiverVersion = int64(applied[len(applied)-1].Version)
	}
	if all := migrator.AllVersions(); len(all) > 0 {
		out.LatestRiverVersion = int64(all[len(all)-1].Version)
	}
	if out.Tables, err = s.tables(ctx); err != nil {
		return nil, rpc.Internal(err, "table stats")
	}
	return out, nil
}

// tables reads the system catalogs directly, since sqlc doesn't know them.
func (s *Service) tables(ctx context.Context) ([]*krillv1.TableStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT n.nspname, c.relname, c.reltuples::bigint,
			pg_total_relation_size(c.oid), pg_indexes_size(c.oid)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p')
			AND n.nspname NOT IN ('pg_catalog', 'information_schema')
			AND n.nspname NOT LIKE 'pg\_toast%'
		ORDER BY 4 DESC, 2`)
	if err != nil {
		return nil, err
	}
	var out []*krillv1.TableStats
	for rows.Next() {
		var t krillv1.TableStats
		var total, index int64
		if err := rows.Scan(&t.Schema, &t.Name, &t.Rows, &total, &index); err != nil {
			return nil, err
		}
		t.TotalBytes, t.IndexBytes = uint64(max(total, 0)), uint64(max(index, 0))
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, t := range out {
		// reltuples is -1 for a table that was never analyzed, which is
		// counted too.
		if t.GetRows() < exactCountBelow {
			table := pgx.Identifier{t.GetSchema(), t.GetName()}.Sanitize()
			if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&t.Rows); err != nil {
				return nil, fmt.Errorf("count %s: %w", table, err)
			}
			continue
		}
		t.RowsEstimated = true
	}
	return out, nil
}

func (s *Service) backupProto(b db.Backup, by string) *krillv1.Backup {
	return &krillv1.Backup{
		Id:         b.ID,
		Status:     statuses[b.Status],
		Scheduled:  b.Scheduled,
		CreatedBy:  by,
		CreatedAt:  ts(b.CreatedAt),
		FinishedAt: ts(b.FinishedAt),
		SizeBytes:  uint64(max(b.SizeBytes, 0)),
		Error:      b.Error,
		Filename:   filename(b.ObjectKey),
	}
}

func (s *Service) ListBackups(ctx context.Context, _ *krillv1.ListBackupsRequest) (*krillv1.ListBackupsResponse, error) {
	rows, err := s.q.ListBackups(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list backups")
	}
	cfg := s.settings()
	now := time.Now()
	zone, _ := now.Zone()
	if name := now.Location().String(); name != "Local" {
		zone = name
	}
	out := &krillv1.ListBackupsResponse{
		Schedule: &krillv1.BackupSchedule{
			Hour:     int32(cfg.BackupHour), //nolint:gosec // an hour of the day
			Keep:     int32(cfg.BackupKeep), //nolint:gosec // a small setting
			TimeZone: zone,
		},
	}
	if cfg.BackupHour >= 0 {
		out.Schedule.NextAt = timestamppb.New(nextDue(now, cfg.BackupHour))
	}
	if err := Available(); err != nil {
		out.Unavailable = err.Error()
	}
	for _, r := range rows {
		b := db.Backup{
			ID: r.ID, Status: r.Status, Scheduled: r.Scheduled, ObjectKey: r.ObjectKey, SizeBytes: r.SizeBytes,
			Error: r.Error, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, FinishedAt: r.FinishedAt,
		}
		out.Backups = append(out.Backups, s.backupProto(b, r.CreatedByName.String))
	}
	return out, nil
}

func (s *Service) CreateBackup(ctx context.Context, _ *krillv1.CreateBackupRequest) (*krillv1.CreateBackupResponse, error) {
	if err := Available(); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	b, err := create(ctx, tx, s.jobs, false, auth.CallerID(ctx))
	if errors.Is(err, ErrBusy) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if err != nil {
		return nil, rpc.Internal(err, "create backup")
	}
	err = audit.Record(ctx, db.New(tx), audit.Entry{
		Action:     audit.BackupCreate,
		TargetType: audit.TargetBackup,
		TargetID:   strconv.FormatInt(b.ID, 10),
		TargetName: filename(b.ObjectKey),
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	sess, _ := auth.SessionFrom(ctx)
	return &krillv1.CreateBackupResponse{Backup: s.backupProto(b, sess.User.Name)}, nil
}

func (s *Service) GetBackupDownload(ctx context.Context, req *krillv1.GetBackupDownloadRequest) (*krillv1.GetBackupDownloadResponse, error) {
	b, err := s.q.GetBackup(ctx, req.GetId())
	if err != nil {
		return nil, rpc.DBError(err, "backup")
	}
	if b.Status != "done" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("backup is not finished"))
	}
	url, err := s.store.PresignGet(ctx, b.ObjectKey, downloadExpiry, filename(b.ObjectKey))
	if err != nil {
		return nil, rpc.Internal(err, "presign download")
	}
	// Downloading is the one way a backup's contents leave the server.
	err = audit.Record(ctx, s.q, audit.Entry{
		Action:     audit.BackupDownload,
		TargetType: audit.TargetBackup,
		TargetID:   strconv.FormatInt(b.ID, 10),
		TargetName: filename(b.ObjectKey),
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	return &krillv1.GetBackupDownloadResponse{Url: url}, nil
}

func (s *Service) DeleteBackup(ctx context.Context, req *krillv1.DeleteBackupRequest) (*krillv1.DeleteBackupResponse, error) {
	err := remove(ctx, s.pool, s.store, req.GetId(), nil)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("backup not found or still running"))
	}
	if err != nil {
		return nil, rpc.Internal(err, "delete backup")
	}
	return &krillv1.DeleteBackupResponse{}, nil
}
