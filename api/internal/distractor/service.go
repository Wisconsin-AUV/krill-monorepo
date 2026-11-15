// Package distractor manages the library of distractor images, scenes with
// none of the labeled objects in them that augmentation puts behind objects.
package distractor

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	// The formats a distractor may be uploaded as.
	_ "image/jpeg"
	_ "image/png"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	_ "golang.org/x/image/webp"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
)

const (
	uploadExpiry = 30 * time.Minute
	urlExpiry    = 12 * time.Hour
	maxFilename  = 255
	maxBytes     = 50 << 20
	// maxPixels keeps decoding an upload within reason, and lets a 33
	// megapixel camera's photos through.
	maxPixels = 40_000_000
)

var formats = map[string]bool{"jpeg": true, "png": true, "webp": true}

var statuses = map[string]krillv1.DistractorStatus{
	"uploading":  krillv1.DistractorStatus_DISTRACTOR_STATUS_UPLOADING,
	"queued":     krillv1.DistractorStatus_DISTRACTOR_STATUS_QUEUED,
	"processing": krillv1.DistractorStatus_DISTRACTOR_STATUS_PROCESSING,
	"ready":      krillv1.DistractorStatus_DISTRACTOR_STATUS_READY,
	"failed":     krillv1.DistractorStatus_DISTRACTOR_STATUS_FAILED,
}

type Service struct {
	krillv1connect.UnimplementedDistractorServiceHandler
	pool  *pgxpool.Pool
	q     *db.Queries
	store *storage.Store
	jobs  *river.Client[pgx.Tx]
}

func NewService(pool *pgxpool.Pool, store *storage.Store, jobs *river.Client[pgx.Tx]) *Service {
	return &Service{pool: pool, q: db.New(pool), store: store, jobs: jobs}
}

func (s *Service) ListDistractors(ctx context.Context, _ *krillv1.ListDistractorsRequest) (*krillv1.ListDistractorsResponse, error) {
	rows, err := s.q.ListDistractors(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list distractors")
	}
	out := make([]*krillv1.Distractor, len(rows))
	for i, r := range rows {
		out[i] = s.toProto(ctx, db.Distractor{
			ID: r.ID, Filename: r.Filename, Width: r.Width, Height: r.Height, SizeBytes: r.SizeBytes,
			CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, Status: r.Status, Error: r.Error,
		}, r.CreatedByName.String)
	}
	return &krillv1.ListDistractorsResponse{Distractors: out}, nil
}

func (s *Service) CreateDistractor(ctx context.Context, req *krillv1.CreateDistractorRequest) (*krillv1.CreateDistractorResponse, error) {
	filename := strings.TrimSpace(path.Base(strings.ReplaceAll(req.GetFilename(), `\`, "/")))
	if filename == "." || filename == "/" {
		filename = ""
	}
	if len(filename) > maxFilename {
		return nil, rpc.Invalid("filename must be at most %d bytes", maxFilename)
	}
	if size := req.GetSize(); size <= 0 || size > maxBytes {
		return nil, rpc.Invalid("the image must be at most %d MB", maxBytes>>20)
	}
	id := auth.CallerID(ctx)
	d, err := s.q.CreateDistractor(ctx, db.CreateDistractorParams{
		Filename:  filename,
		CreatedBy: pgtype.UUID{Bytes: id, Valid: id != uuid.Nil},
	})
	if err != nil {
		return nil, rpc.Internal(err, "create distractor")
	}
	url, err := s.store.PresignPut(ctx, storage.DistractorUploadKey(d.ID), uploadExpiry)
	if err != nil {
		return nil, rpc.Internal(err, "presign upload")
	}
	sess, _ := auth.SessionFrom(ctx)
	return &krillv1.CreateDistractorResponse{Distractor: s.toProto(ctx, d, sess.User.Name), UploadUrl: url}, nil
}

func (s *Service) ProcessDistractor(ctx context.Context, req *krillv1.ProcessDistractorRequest) (*krillv1.ProcessDistractorResponse, error) {
	id := req.GetId()
	d, err := s.q.GetDistractor(ctx, id)
	if err != nil {
		return nil, rpc.DBError(err, "distractor")
	}
	if d.Status != "uploading" && d.Status != "failed" {
		return nil, failed("the distractor is not waiting to be processed")
	}
	key := storage.DistractorUploadKey(id)
	obj, ok, err := s.store.Stat(ctx, key)
	if err != nil {
		return nil, rpc.Internal(err, "check upload")
	}
	if !ok || obj.Size == 0 {
		return nil, failed("the image has not been uploaded")
	}
	if reject, err := s.check(ctx, key, obj.Size); err != nil {
		return nil, err
	} else if reject != nil {
		if err := s.delete(ctx, []int64{id}); err != nil {
			slog.WarnContext(ctx, "delete rejected distractor", "id", id, "err", err)
		}
		return nil, connect.NewError(connect.CodeFailedPrecondition, reject)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if _, err := q.QueueDistractor(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return nil, failed("the distractor is not waiting to be processed")
	} else if err != nil {
		return nil, rpc.DBError(err, "distractor")
	}
	if _, err := s.jobs.InsertTx(ctx, tx, Args{DistractorID: id}, nil); err != nil {
		return nil, rpc.Internal(err, "queue processing")
	}
	if d.Status == "uploading" {
		err = audit.Record(ctx, q, audit.Entry{
			Action:     audit.DistractorAdd,
			TargetType: audit.TargetDistractor,
			TargetID:   strconv.FormatInt(id, 10),
			TargetName: d.Filename,
			Details:    map[string]any{"bytes": obj.Size},
		})
		if err != nil {
			return nil, rpc.Internal(err, "record audit entry")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.ProcessDistractorResponse{}, nil
}

// check reads the uploaded image's header. reject says why it isn't an image
// Krill can use, which no retry would change.
func (s *Service) check(ctx context.Context, key string, size int64) (reject, err error) {
	if size > maxBytes {
		return fmt.Errorf("the image is larger than %d MB", maxBytes>>20), nil
	}
	rc, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, rpc.Internal(err, "read upload")
	}
	defer func() { _ = rc.Close() }()
	_, reject = checkImage(rc)
	return reject, nil
}

func checkImage(r io.Reader) (image.Config, error) {
	cfg, format, err := image.DecodeConfig(r)
	if err != nil || !formats[format] {
		return image.Config{}, errors.New("the image must be a JPEG, PNG, or WebP")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return image.Config{}, fmt.Errorf("the image must be at most %d megapixels", maxPixels/1_000_000)
	}
	return cfg, nil
}

func (s *Service) DeleteDistractors(ctx context.Context, req *krillv1.DeleteDistractorsRequest) (*krillv1.DeleteDistractorsResponse, error) {
	if err := s.delete(ctx, req.GetIds()); err != nil {
		return nil, err
	}
	return &krillv1.DeleteDistractorsResponse{}, nil
}

// delete skips IDs that are already gone, so a canceled upload and a delete
// from the page don't fail each other.
func (s *Service) delete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	rows, err := q.DeleteDistractors(ctx, ids)
	if err != nil {
		return rpc.Internal(err, "delete distractors")
	}
	for _, d := range rows {
		// Nothing was audited for an upload that never finished.
		if d.Status == "uploading" {
			continue
		}
		err = audit.Record(ctx, q, audit.Entry{
			Action:     audit.DistractorDelete,
			TargetType: audit.TargetDistractor,
			TargetID:   strconv.FormatInt(d.ID, 10),
			TargetName: d.Filename,
		})
		if err != nil {
			return rpc.Internal(err, "record audit entry")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return rpc.Internal(err, "commit")
	}
	for _, d := range rows {
		if err := s.store.RemovePrefix(ctx, storage.DistractorPrefix(d.ID)); err != nil {
			slog.WarnContext(ctx, "remove distractor", "id", d.ID, "err", err)
		}
	}
	return nil
}

func (s *Service) toProto(ctx context.Context, d db.Distractor, by string) *krillv1.Distractor {
	out := &krillv1.Distractor{
		Id:        d.ID,
		Filename:  d.Filename,
		Width:     d.Width,
		Height:    d.Height,
		SizeBytes: d.SizeBytes,
		CreatedBy: by,
		CreatedAt: timestamppb.New(d.CreatedAt.Time),
		Status:    statuses[d.Status],
		Error:     d.Error,
	}
	if d.Status != "ready" {
		return out
	}
	var err error
	if out.Url, err = s.store.PresignGet(ctx, storage.DistractorKey(d.ID), urlExpiry, ""); err != nil {
		slog.WarnContext(ctx, "presign distractor", "id", d.ID, "err", err)
	}
	if out.ThumbnailUrl, err = s.store.PresignGet(ctx, storage.DistractorThumbnailKey(d.ID), urlExpiry, ""); err != nil {
		slog.WarnContext(ctx, "presign distractor thumbnail", "id", d.ID, "err", err)
	}
	return out
}

func failed(format string, args ...any) error {
	return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf(format, args...))
}
