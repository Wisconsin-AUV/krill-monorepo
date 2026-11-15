// Package distractor manages the library of distractor images, scenes with
// none of the labeled objects in them that augmentation puts behind objects.
package distractor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	// The formats a distractor may be.
	_ "image/jpeg"
	_ "image/png"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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
	// maxPixels keeps decoding a distractor during an export within reason.
	maxPixels = 40_000_000
)

var keyName = regexp.MustCompile(`^[0-9a-f]{32}$`)

var formats = map[string]bool{"jpeg": true, "png": true, "webp": true}

type Service struct {
	krillv1connect.UnimplementedDistractorServiceHandler
	pool  *pgxpool.Pool
	q     *db.Queries
	store *storage.Store
}

func NewService(pool *pgxpool.Pool, store *storage.Store) *Service {
	return &Service{pool: pool, q: db.New(pool), store: store}
}

// ValidKey reports whether key is one CreateDistractorUpload could have
// issued. Any other key would let AddDistractor list, and hand out read URLs
// for, an arbitrary object such as a backup.
func ValidKey(key string) bool {
	name, ok := strings.CutPrefix(key, storage.DistractorsPrefix)
	return ok && keyName.MatchString(name)
}

func (s *Service) ListDistractors(ctx context.Context, _ *krillv1.ListDistractorsRequest) (*krillv1.ListDistractorsResponse, error) {
	rows, err := s.q.ListDistractors(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list distractors")
	}
	out := make([]*krillv1.Distractor, len(rows))
	for i, r := range rows {
		out[i] = s.toProto(ctx, db.Distractor{
			ID: r.ID, ObjectKey: r.ObjectKey, Filename: r.Filename, Width: r.Width, Height: r.Height,
			SizeBytes: r.SizeBytes, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt,
		}, r.CreatedByName.String)
	}
	return &krillv1.ListDistractorsResponse{Distractors: out}, nil
}

func (s *Service) CreateDistractorUpload(ctx context.Context, _ *krillv1.CreateDistractorUploadRequest) (*krillv1.CreateDistractorUploadResponse, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, rpc.Internal(err, "generate key")
	}
	key := storage.DistractorsPrefix + hex.EncodeToString(b)
	url, err := s.store.PresignPut(ctx, key, uploadExpiry)
	if err != nil {
		return nil, rpc.Internal(err, "presign upload")
	}
	return &krillv1.CreateDistractorUploadResponse{Key: key, UploadUrl: url}, nil
}

func (s *Service) AddDistractor(ctx context.Context, req *krillv1.AddDistractorRequest) (*krillv1.AddDistractorResponse, error) {
	key := req.GetKey()
	if !ValidKey(key) {
		return nil, rpc.Invalid("key was not issued for a distractor upload")
	}
	filename := strings.TrimSpace(path.Base(strings.ReplaceAll(req.GetFilename(), `\`, "/")))
	if filename == "." || filename == "/" {
		filename = ""
	}
	if len(filename) > maxFilename {
		return nil, rpc.Invalid("filename must be at most %d bytes", maxFilename)
	}
	obj, ok, err := s.store.Stat(ctx, key)
	if err != nil {
		return nil, rpc.Internal(err, "check upload")
	}
	if !ok || obj.Size == 0 {
		return nil, failed("the image has not been uploaded")
	}
	cfg, err := s.check(ctx, key, obj.Size)
	if err != nil {
		if rerr := s.store.Remove(ctx, key); rerr != nil {
			slog.WarnContext(ctx, "remove rejected distractor", "key", key, "err", rerr)
		}
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	id := auth.CallerID(ctx)
	d, err := q.CreateDistractor(ctx, db.CreateDistractorParams{
		ObjectKey: key,
		Filename:  filename,
		Width:     int32(cfg.Width),  //nolint:gosec // bounded by maxPixels
		Height:    int32(cfg.Height), //nolint:gosec // bounded by maxPixels
		SizeBytes: obj.Size,
		CreatedBy: pgtype.UUID{Bytes: id, Valid: id != uuid.Nil},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("distractor was already added"))
		}
		return nil, rpc.Internal(err, "add distractor")
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.DistractorAdd,
		TargetType: audit.TargetDistractor,
		TargetID:   strconv.FormatInt(d.ID, 10),
		TargetName: filename,
		Details:    map[string]any{"width": cfg.Width, "height": cfg.Height, "bytes": obj.Size},
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	sess, _ := auth.SessionFrom(ctx)
	return &krillv1.AddDistractorResponse{Distractor: s.toProto(ctx, d, sess.User.Name)}, nil
}

// check reads the uploaded image's header.
func (s *Service) check(ctx context.Context, key string, size int64) (image.Config, error) {
	if size > maxBytes {
		return image.Config{}, failed("the image is larger than %d MB", maxBytes>>20)
	}
	rc, err := s.store.Get(ctx, key)
	if err != nil {
		return image.Config{}, rpc.Internal(err, "read upload")
	}
	defer func() { _ = rc.Close() }()
	cfg, format, err := image.DecodeConfig(rc)
	if err != nil || !formats[format] {
		return image.Config{}, failed("the image must be a JPEG, PNG, or WebP")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return image.Config{}, failed("the image must be at most %d megapixels", maxPixels/1_000_000)
	}
	return cfg, nil
}

func (s *Service) DeleteDistractor(ctx context.Context, req *krillv1.DeleteDistractorRequest) (*krillv1.DeleteDistractorResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	d, err := q.DeleteDistractor(ctx, req.GetId())
	if err != nil {
		return nil, rpc.DBError(err, "distractor")
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.DistractorDelete,
		TargetType: audit.TargetDistractor,
		TargetID:   strconv.FormatInt(d.ID, 10),
		TargetName: d.Filename,
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	if err := s.store.Remove(ctx, d.ObjectKey); err != nil {
		slog.WarnContext(ctx, "remove distractor", "id", d.ID, "err", err)
	}
	return &krillv1.DeleteDistractorResponse{}, nil
}

func (s *Service) toProto(ctx context.Context, d db.Distractor, by string) *krillv1.Distractor {
	url, err := s.store.PresignGet(ctx, d.ObjectKey, urlExpiry, "")
	if err != nil {
		slog.WarnContext(ctx, "presign distractor", "id", d.ID, "err", err)
	}
	return &krillv1.Distractor{
		Id:        d.ID,
		Url:       url,
		Filename:  d.Filename,
		Width:     d.Width,
		Height:    d.Height,
		SizeBytes: d.SizeBytes,
		CreatedBy: by,
		CreatedAt: timestamppb.New(d.CreatedAt.Time),
	}
}

func failed(format string, args ...any) error {
	return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf(format, args...))
}
