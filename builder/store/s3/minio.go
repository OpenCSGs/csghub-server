package s3

import (
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"opencsg.com/starhub-server/common/config"
)

func NewMinio(cfg *config.Config) (*minio.Client, error) {
	if !cfg.S3.Enable {
		return nil, fmt.Errorf("S3 storage is not enabled. Please config STARHUB_SERVER_S3_* to enable S3 storage.")
	}
	return minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.S3.AccessKeyID, cfg.S3.AccessKeySecret, ""),
		Secure:       true,
		BucketLookup: minio.BucketLookupAuto,
		Region:       cfg.S3.Region,
	})
}
