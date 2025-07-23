package s3

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/sha256-simd"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"opencsg.com/csghub-server/common/config"
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

func NewMinioCore(cfg *config.Config) (Core, error) {
	var bucketLookupType minio.BucketLookupType
	if val, ok := bucketLookupMapping[cfg.S3.BucketLookup]; ok {
		bucketLookupType = val
	} else {
		bucketLookupType = minio.BucketLookupAuto
	}
	core, err := minio.NewCore(cfg.S3.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.S3.AccessKeyID, cfg.S3.AccessKeySecret, ""),
		Secure:       cfg.S3.EnableSSL,
		BucketLookup: bucketLookupType,
		Region:       cfg.S3.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init s3 core, error:%w", err)
	}

	return core, nil
}

type MinioClient interface {
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64,
		opts minio.PutObjectOptions,
	) (info minio.UploadInfo, err error)
	StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (u *url.URL, err error)
	PresignHeader(ctx context.Context, method, bucketName, objectName string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (u *url.URL, err error)
	CopyObject(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
}

type Client interface {
	MinioClient
	UploadAndValidate(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64) (minio.UploadInfo, error)
}

type Core interface {
	PresignHeader(ctx context.Context, method, bucketName, objectName string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (u *url.URL, err error)
	NewMultipartUpload(ctx context.Context, bucket, object string, opts minio.PutObjectOptions) (uploadID string, err error)
	CompleteMultipartUpload(ctx context.Context, bucket, object, uploadID string, parts []minio.CompletePart, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	PutObjectPart(ctx context.Context, bucket, object, uploadID string, partID int, data io.Reader, size int64, opts minio.PutObjectPartOptions) (minio.ObjectPart, error)
	ListObjectParts(ctx context.Context, bucket, object, uploadID string, partNumberMarker, maxParts int) (result minio.ListObjectPartsResult, err error)
	AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string) error
}

func uploadAndValidate(ctx context.Context, client MinioClient, bucketName, objectName string, reader io.Reader, objectSize int64) (minio.UploadInfo, error) {
	var rawName, tmpName, oid string
	ctx, span := tracer.Start(ctx, "s3.UploadAndValidate")
	defer span.End()
	span.SetAttributes(attribute.Int64("size", objectSize))

	h := sha256.New()
	reader = io.TeeReader(reader, h)
	if strings.HasPrefix(objectName, "lfs/") {
		rawName = strings.TrimPrefix(objectName, "lfs/")
		tmpName = S3_TMP_DIR + rawName
		oid = strings.Join(strings.Split(rawName, "/"), "")
	} else {
		tmpName = objectName
		oid = strings.Join(strings.Split(objectName, "/")[4:], "")
	}

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

	if strings.HasPrefix(objectName, "repos/") {
		return info, nil
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
