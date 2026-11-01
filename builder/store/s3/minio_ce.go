//go:build !ee && !saas

package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/common/config"
)

type minioClient struct {
	*minio.Client
}

// NewMinio creates an S3 client with optional per-client transport customization.
func NewMinio(cfg *config.Config, options ...ClientOption) (Client, error) {
	var bucketLookupType minio.BucketLookupType
	if val, ok := bucketLookupMapping[cfg.S3.BucketLookup]; ok {
		bucketLookupType = val
	} else {
		bucketLookupType = minio.BucketLookupAuto
	}
	mClient, err := minio.New(cfg.S3.Endpoint, clientOptions(cfg, bucketLookupType, options))
	if err != nil {
		return nil, fmt.Errorf("failed to init s3 client, error:%w", err)
	}
	mClient.SetS3EnableDualstack(cfg.S3.EnableDualstack)
	client := &minioClient{
		Client: mClient,
	}

	return client, nil
}

func (c *minioClient) UploadAndValidate(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64) (minio.UploadInfo, error) {
	return uploadAndValidate(ctx, c.Client, bucketName, objectName, reader, objectSize)
}
