package parquet

import (
	"context"
	"io"
	"os"

	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/builder/store/s3"
)

type ReaderAtCloser interface {
	io.ReaderAt
	io.Closer
}

type FileObject struct {
	size   int64
	reader ReaderAtCloser
}

type IOClient interface {
	GetFileObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*FileObject, error)
}

type MinIOClient struct {
	minioClient s3.Client
}

func NewMinIOClient(client s3.Client) *MinIOClient {
	return &MinIOClient{minioClient: client}
}

func (c *MinIOClient) GetFileObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*FileObject, error) {
	obj, err := c.minioClient.GetObject(ctx, bucketName, objectName, opts)
	if err != nil {
		return nil, err
	}

	stats, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, err
	}

	return &FileObject{size: stats.Size, reader: obj}, nil
}

// used in unit test or local manually test
type OSFileClient struct{}

func (c *OSFileClient) GetFileObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*FileObject, error) {
	f, err := os.Open(objectName)
	if err != nil {
		return nil, err
	}
	stats, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return &FileObject{size: stats.Size(), reader: f}, nil
}
