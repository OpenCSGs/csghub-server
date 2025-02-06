package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
)

func TestMinIO_UploadAndValidate(t *testing.T) {
	cases := []struct {
		name           string
		putFailed      bool
		checksumFailed bool
		sizeFailed     bool
		copyFailed     bool
		removeFailed   bool
		err            string
	}{
		{name: "success"},
		{
			name: "invalid checksum", checksumFailed: true,
			err: "LFSObject: expected sha256 c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2, got a7452118bfc838ee7b2aac14a8bc88c50a1ae4620903c4f8cdd327bb79961899",
		},
		{
			name: "invalid size", sizeFailed: true,
			err: "LFSObject: expected size 6, got 4",
		},
		{name: "put tmp failed", putFailed: true, err: "put failed"},
		{name: "copy failed", copyFailed: true, err: "copy failed"},
		{name: "remove failed", removeFailed: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mockClient := s3.NewMockClient(t)
			ctx := context.TODO()
			checksum := fmt.Sprintf("%x", sha256.Sum256([]byte("foobar")))
			key := checksum[:1] + "/" + checksum[1:2] + "/" + checksum[2:]
			reader := bytes.NewReader([]byte("foobar"))

			mockClient.EXPECT().PutObject(
				mock.Anything, "foo", "lfs/tmp/"+key, mock.Anything, int64(6), minio.PutObjectOptions{
					ContentType:           "application/octet-stream",
					SendContentMd5:        true,
					ConcurrentStreamParts: true,
					NumThreads:            MINIO_PUT_PARALLEL,
					PartSize:              MINIO_PART_SIZE,
				}).RunAndReturn(
				func(
					ctx context.Context, s1, s2 string,
					r io.Reader, i int64, poo minio.PutObjectOptions,
				) (minio.UploadInfo, error) {
					if c.putFailed {
						return minio.UploadInfo{}, errors.New("put failed")
					}
					if c.checksumFailed {
						_, err := r.Read(make([]byte, 4))
						require.NoError(t, err)
					} else {
						_, err := r.Read(make([]byte, 6))
						require.NoError(t, err)
					}
					if c.sizeFailed {
						return minio.UploadInfo{Size: 4}, nil
					}
					return minio.UploadInfo{Size: 6}, nil
				},
			)

			if !c.putFailed && !c.checksumFailed && !c.sizeFailed {
				mockClient.EXPECT().CopyObject(mock.Anything, minio.CopyDestOptions{
					Bucket: "foo",
					Object: "lfs/" + key,
				}, minio.CopySrcOptions{
					Bucket: "foo",
					Object: "lfs/tmp/" + key,
				}).RunAndReturn(
					func(
						ctx context.Context, cdo minio.CopyDestOptions, cso minio.CopySrcOptions,
					) (minio.UploadInfo, error) {
						if c.copyFailed {
							return minio.UploadInfo{}, errors.New("copy failed")
						}
						return minio.UploadInfo{VersionID: "abc"}, nil
					})
			}

			if !c.putFailed {
				mockClient.EXPECT().RemoveObject(
					mock.Anything, "foo", "lfs/tmp/"+key, minio.RemoveObjectOptions{},
				).RunAndReturn(func(
					ctx context.Context, s1, s2 string, roo minio.RemoveObjectOptions,
				) error {
					if c.removeFailed {
						return errors.New("remove failed")
					}
					return nil
				})
			}

			info, err := uploadAndValidate(
				ctx, mockClient, "foo", "lfs/"+key,
				reader, 6,
			)

			if c.err != "" {
				require.Equal(t, c.err, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, minio.UploadInfo{VersionID: "abc"}, info)
			}
		})
	}
}
