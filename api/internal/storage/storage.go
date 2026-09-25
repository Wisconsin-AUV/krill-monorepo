package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Setting the region up front stops minio-go from making a GetBucketLocation
// call, which would fail for the presign client when the public endpoint is
// not reachable from inside the API container.
const region = "us-east-1"

type Config struct {
	Endpoint       string
	PublicEndpoint string
	Bucket         string
	AccessKey      string
	SecretKey      string
}

type Store struct {
	client  *minio.Client
	presign *minio.Client
	bucket  string
}

func New(ctx context.Context, cfg Config) (*Store, error) {
	client, err := newClient(cfg.Endpoint, cfg)
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	presign := client
	if cfg.PublicEndpoint != "" && cfg.PublicEndpoint != cfg.Endpoint {
		presign, err = newClient(cfg.PublicEndpoint, cfg)
		if err != nil {
			return nil, fmt.Errorf("s3 presign client: %w", err)
		}
	}

	s := &Store{client: client, presign: presign, bucket: cfg.Bucket}
	if err := s.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func newClient(endpoint string, cfg Config) (*minio.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint %q: %w", endpoint, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("endpoint %q has no host", endpoint)
	}
	return minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: u.Scheme == "https",
		Region: region,
	})
}

func (s *Store) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check bucket %s: %w", s.bucket, err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: region}); err != nil {
		return fmt.Errorf("create bucket %s: %w", s.bucket, err)
	}
	return nil
}

func (s *Store) PresignPut(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.presign.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return "", fmt.Errorf("presign put %s: %w", key, err)
	}
	return u.String(), nil
}

func (s *Store) PresignGet(ctx context.Context, key string, expiry time.Duration, filename string) (string, error) {
	params := url.Values{}
	if filename != "" {
		params.Set("response-content-disposition", fmt.Sprintf("attachment; filename=%q", filename))
	}
	u, err := s.presign.PresignedGetObject(ctx, s.bucket, key, expiry, params)
	if err != nil {
		return "", fmt.Errorf("presign get %s: %w", key, err)
	}
	return u.String(), nil
}

// PresignGetInternal presigns against the API's own endpoint, for services
// such as the GPU worker that share its network rather than the browser's.
func (s *Store) PresignGetInternal(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("presign get %s: %w", key, err)
	}
	return u.String(), nil
}

// Put uploads r. Pass size -1 when the length is unknown; the object is then
// sent as a multipart upload.
func (s *Store) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	return obj, nil
}

func (s *Store) Download(ctx context.Context, key, path string) error {
	if err := s.client.FGetObject(ctx, s.bucket, key, path, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("download %s: %w", key, err)
	}
	return nil
}

func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", key, err)
}

func (s *Store) RemovePrefix(ctx context.Context, prefix string) error {
	objects := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	var errs []error
	for res := range s.client.RemoveObjects(ctx, s.bucket, objects, minio.RemoveObjectsOptions{}) {
		errs = append(errs, fmt.Errorf("remove %s: %w", res.ObjectName, res.Err))
	}
	return errors.Join(errs...)
}
