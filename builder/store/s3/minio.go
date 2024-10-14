package s3

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"opencsg.com/csghub-server/common/config"
)

func NewMinio(cfg *config.Config) (*Client, error) {
	minioClient, err := minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.S3.AccessKeyID, cfg.S3.AccessKeySecret, ""),
		Secure:       cfg.S3.EnableSSL,
		BucketLookup: minio.BucketLookupAuto,
		Region:       cfg.S3.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init s3 client, error:%w", err)
	}
	client := &Client{
		Client: minioClient,
	}
	if len(cfg.S3.InternalEndpoint) > 0 {
		minioClientInternal, err := minio.New(cfg.S3.InternalEndpoint, &minio.Options{
			Creds:        credentials.NewStaticV4(cfg.S3.AccessKeyID, cfg.S3.AccessKeySecret, ""),
			Secure:       cfg.S3.EnableSSL,
			BucketLookup: minio.BucketLookupAuto,
			Region:       cfg.S3.Region,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to init s3 internal client, error:%w", err)
		}
		client.internalClient = minioClientInternal
	}
	return client, nil
}

type Client struct {
	*minio.Client
	internalClient *minio.Client
}

// PresignedGetObject is a wrapper around minio.Client.PresignedGetObject that adds some extra customization.
func (c *Client) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	if c.useInternalClient(ctx) && c.internalClient != nil {
		slog.Info("use internal s3 client for presigned get object", slog.String("bucket_name", bucketName), slog.String("object_name", objectName))
		return c.internalClient.PresignedGetObject(ctx, bucketName, objectName, expires, reqParams)
	}
	return c.Client.PresignedGetObject(ctx, bucketName, objectName, expires, reqParams)
}

func (c *Client) useInternalClient(ctx context.Context) bool {
	v, success := ctx.Value("X-OPENCSG-S3-Internal").(bool)
	if !success {
		return false
	}

	return v
}
