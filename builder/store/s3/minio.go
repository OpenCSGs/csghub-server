package s3

import (
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"opencsg.com/csghub-server/common/config"
)

func NewMinio(cfg *config.Config) (*minio.Client, error) {
	return minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.S3.AccessKeyID, cfg.S3.AccessKeySecret, ""),
		Secure:       cfg.S3.EnableSSL,
		BucketLookup: minio.BucketLookupAuto,
		Region:       cfg.S3.Region,
	})
}
