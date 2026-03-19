package component

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
)

const defaultPresignExpiry = 1 * time.Hour

type storageImpl struct {
	config   *config.Config
	s3Client s3.Client
}

// NewStorage creates a Storage implementation that uses S3 for generated content (image/audio/video).
// Returns nil if S3 client cannot be created (e.g. missing config); callers should treat nil as "storage disabled".
func NewStorage(cfg *config.Config) (types.Storage, error) {
	if cfg == nil {
		return nil, nil
	}
	client, err := s3.NewMinio(cfg)
	if err != nil {
		return nil, fmt.Errorf("create S3 client for generated content: %w", err)
	}
	return &storageImpl{config: cfg, s3Client: client}, nil
}

func (s *storageImpl) PutAndPresignGet(ctx context.Context, bucket, key string, data []byte, contentType string) (string, error) {
	if bucket == "" {
		return "", fmt.Errorf("bucket is required")
	}
	expiry := defaultPresignExpiry
	if s.config.AIGateway.PresignExpirySeconds > 0 {
		expiry = time.Duration(s.config.AIGateway.PresignExpirySeconds) * time.Second
	}
	reader := bytes.NewReader(data)
	_, err := s.s3Client.PutObject(ctx, bucket, key, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}
	u, err := s.s3Client.PresignedGetObject(ctx, bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("presign get object: %w", err)
	}
	return u.String(), nil
}
