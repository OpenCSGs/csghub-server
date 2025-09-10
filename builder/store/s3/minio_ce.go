//go:build !ee && !saas

package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"opencsg.com/csghub-server/common/config"
)

type minioClient struct {
	*minio.Client
}

func NewMinio(cfg *config.Config) (Client, error) {
	var bucketLookupType minio.BucketLookupType
	if val, ok := bucketLookupMapping[cfg.S3.BucketLookup]; ok {
		bucketLookupType = val
	} else {
		bucketLookupType = minio.BucketLookupAuto
	}
	mClient, err := minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.S3.AccessKeyID, cfg.S3.AccessKeySecret, ""),
		Secure:       cfg.S3.EnableSSL,
		BucketLookup: bucketLookupType,
		Region:       cfg.S3.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init s3 client, error:%w", err)
	}
	client := &minioClient{
		Client: mClient,
	}

	return client, nil
}

func (c *minioClient) UploadAndValidate(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64) (minio.UploadInfo, error) {
	return uploadAndValidate(ctx, c.Client, bucketName, objectName, reader, objectSize)
}
