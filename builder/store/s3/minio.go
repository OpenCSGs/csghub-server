package s3

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/sha256-simd"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	S3_TMP_DIR         = "lfs/tmp/"
	MINIO_PUT_PARALLEL = 5
	MINIO_PART_SIZE    = 5 << 20
)

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("builder.store.s3")
}

type MinioClient interface {
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64,
		opts minio.PutObjectOptions,
	) (info minio.UploadInfo, err error)
	StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (u *url.URL, err error)
	CopyObject(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
}

type Client interface {
	MinioClient
	UploadAndValidate(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64) (minio.UploadInfo, error)
}

func uploadAndValidate(ctx context.Context, client MinioClient, bucketName, objectName string, reader io.Reader, objectSize int64) (minio.UploadInfo, error) {
	ctx, span := tracer.Start(ctx, "s3.UploadAndValidate")
	defer span.End()
	span.SetAttributes(attribute.Int64("size", objectSize))

	h := sha256.New()
	reader = io.TeeReader(reader, h)
	rawName := strings.TrimPrefix(objectName, "lfs/")
	tmpName := S3_TMP_DIR + rawName
	oid := strings.Join(strings.Split(rawName, "/"), "")

	// upload to tmp dir
	info, err := client.PutObject(ctx, bucketName, tmpName, reader, objectSize, minio.PutObjectOptions{
		ContentType:           "application/octet-stream",
		SendContentMd5:        true,
		ConcurrentStreamParts: true,
		NumThreads:            MINIO_PUT_PARALLEL,
		PartSize:              MINIO_PART_SIZE,
	})
	span.AddEvent("put object to tmp bucket done")
	if err != nil {
		return info, err
	}

	defer func() {
		func() {
			err := client.RemoveObject(ctx, bucketName, tmpName, minio.RemoveObjectOptions{})
			if err != nil {
				slog.Error("minio remove file failed", slog.Any("error", err))
			}
		}()
	}()

	if info.Size != objectSize {
		err := fmt.Errorf("LFSObject: expected size %d, got %d", objectSize, info.Size)
		return minio.UploadInfo{}, err
	}
	checksum := hex.EncodeToString(h.Sum(nil))
	if !bytes.Equal([]byte(checksum), []byte(oid)) {
		err := fmt.Errorf("LFSObject: expected sha256 %s, got %s", oid, checksum)
		return minio.UploadInfo{}, err
	}
	span.AddEvent("validate checksum done")

	return client.CopyObject(ctx, minio.CopyDestOptions{
		Bucket: bucketName,
		Object: objectName,
	}, minio.CopySrcOptions{
		Bucket: bucketName,
		Object: tmpName,
	})
}
