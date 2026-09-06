package objectstore

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"sync"
	"tokoloop/internal/config"
)

type Store interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}
type MinIO struct {
	client *minio.Client
	bucket string
	mu     sync.Mutex
	ready  bool
}

func NewMinIO(c config.MinIOConfig) (*MinIO, error) {
	client, e := minio.New(c.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""), Secure: c.UseSSL})
	if e != nil {
		return nil, e
	}
	return &MinIO{client: client, bucket: c.Bucket}, nil
}
func (s *MinIO) ensure(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ready {
		return nil
	}
	ok, e := s.client.BucketExists(ctx, s.bucket)
	if e != nil {
		return e
	}
	if !ok {
		if e = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); e != nil {
			code := minio.ToErrorResponse(e).Code
			if code != "BucketAlreadyOwnedByYou" && code != "BucketAlreadyExists" {
				return fmt.Errorf("object bucket unavailable: %s", code)
			}
		}
	}
	s.ready = true
	return nil
}
func (s *MinIO) Put(ctx context.Context, key string, r io.Reader, size int64, mime string) error {
	if e := s.ensure(ctx); e != nil {
		return e
	}
	_, e := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: mime})
	return e
}
func (s *MinIO) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	r, e := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if e != nil {
		return nil, e
	}
	if _, e = r.Stat(); e != nil {
		r.Close()
		return nil, e
	}
	return r, nil
}
func (s *MinIO) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
