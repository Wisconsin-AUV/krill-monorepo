package storage

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/minio/minio-go/v7"
)

type Object struct {
	Key      string
	Size     int64
	Modified time.Time
}

// Upload is a multipart upload that was started and never completed or
// aborted. Its parts take space without being an object.
type Upload struct {
	Key       string
	ID        string
	Size      int64
	Initiated time.Time
}

// List yields every object under prefix in key order.
func (s *Store) List(ctx context.Context, prefix string) iter.Seq2[Object, error] {
	return func(yield func(Object, error) bool) {
		// Cancelling stops minio-go's listing goroutine when the caller
		// breaks early.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for o := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if o.Err != nil {
				yield(Object{}, fmt.Errorf("list %s: %w", prefix, o.Err))
				return
			}
			if !yield(Object{Key: o.Key, Size: o.Size, Modified: o.LastModified}, nil) {
				return
			}
		}
	}
}

// ListUploads yields the unfinished multipart uploads under prefix.
func (s *Store) ListUploads(ctx context.Context, prefix string) iter.Seq2[Upload, error] {
	return func(yield func(Upload, error) bool) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for u := range s.client.ListIncompleteUploads(ctx, s.bucket, prefix, true) {
			if u.Err != nil {
				yield(Upload{}, fmt.Errorf("list uploads %s: %w", prefix, u.Err))
				return
			}
			size, err := s.uploadSize(ctx, u.Key, u.UploadID)
			if !yield(Upload{Key: u.Key, ID: u.UploadID, Size: size, Initiated: u.Initiated}, err) || err != nil {
				return
			}
		}
	}
}

func (s *Store) uploadSize(ctx context.Context, key, uploadID string) (int64, error) {
	core := minio.Core{Client: s.client}
	var size int64
	marker := 0
	for {
		res, err := core.ListObjectParts(ctx, s.bucket, key, uploadID, marker, 1000)
		if minio.ToErrorResponse(err).Code == minio.NoSuchUpload {
			return size, nil
		}
		if err != nil {
			return 0, fmt.Errorf("list parts %s: %w", key, err)
		}
		for _, p := range res.ObjectParts {
			size += p.Size
		}
		if !res.IsTruncated {
			return size, nil
		}
		marker = res.NextPartNumberMarker
	}
}

// Stat returns false when key doesn't exist.
func (s *Store) Stat(ctx context.Context, key string) (Object, bool, error) {
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
		return Object{}, false, nil
	}
	if err != nil {
		return Object{}, false, fmt.Errorf("stat %s: %w", key, err)
	}
	return Object{Key: key, Size: info.Size, Modified: info.LastModified}, true, nil
}

// RemoveKeys deletes keys in batches. Keys that are already gone are not an
// error.
func (s *Store) RemoveKeys(ctx context.Context, keys []string) error {
	objects := make(chan minio.ObjectInfo)
	go func() {
		defer close(objects)
		for _, k := range keys {
			select {
			case objects <- minio.ObjectInfo{Key: k}:
			case <-ctx.Done():
				return
			}
		}
	}()
	var errs []error
	for res := range s.client.RemoveObjects(ctx, s.bucket, objects, minio.RemoveObjectsOptions{}) {
		errs = append(errs, fmt.Errorf("remove %s: %w", res.ObjectName, res.Err))
	}
	if err := ctx.Err(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// AbortUpload aborts one multipart upload, leaving any others for the key.
func (s *Store) AbortUpload(ctx context.Context, key, uploadID string) error {
	err := minio.Core{Client: s.client}.AbortMultipartUpload(ctx, s.bucket, key, uploadID)
	if err != nil && minio.ToErrorResponse(err).Code != minio.NoSuchUpload {
		return fmt.Errorf("abort upload %s: %w", key, err)
	}
	return nil
}
