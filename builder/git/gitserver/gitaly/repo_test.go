package gitaly

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	gitalypb_mock "opencsg.com/csghub-server/_mocks/gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
)

func TestGitalyRepo_CloneProjectStorage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.TODO()
		fromMock := gitalypb_mock.NewMockRepositoryServiceClient(t)
		toMock := gitalypb_mock.NewMockRepositoryServiceClient(t)
		helper := &CloneStorageHelper{
			from: fromMock,
			to:   toMock,
		}
		req := &ProjectStorageCloneRequest{
			CurrentGitalyAddress: "gca",
			CurrentGitalyToken:   "gct",
			CurrentGitalyStorage: "gcs",
			NewGitalyAddress:     "nca",
			NewGitalyToken:       "nct",
			NewGitalyStorage:     "ncs",
			FilesServer:          "http://foo.com/",
		}
		fromMock.EXPECT().RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "gcs",
				RelativePath: "foo",
			}}).Return(&gitalypb.RepositoryExistsResponse{Exists: true}, nil)
		toMock.EXPECT().RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "ncs",
				RelativePath: "foo",
			}}).Return(&gitalypb.RepositoryExistsResponse{Exists: false}, nil)
		fromMock.EXPECT().GetSnapshot(ctx, &gitalypb.GetSnapshotRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "gcs",
				RelativePath: "foo",
			},
		}).Return(&MockGrpcStreamClient[*gitalypb.GetSnapshotResponse]{
			data: []*gitalypb.GetSnapshotResponse{
				{Data: []byte("foobar")},
			},
		}, nil)
		toMock.EXPECT().CreateRepositoryFromSnapshot(ctx, &gitalypb.CreateRepositoryFromSnapshotRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "ncs",
				RelativePath: "foo",
			},
			HttpUrl: req.FilesServer + "foo.tar",
		}).Return(&gitalypb.CreateRepositoryFromSnapshotResponse{}, nil)
		err := helper.CloneRepoStorage(ctx, "foo", req)
		require.NoError(t, err)
	})
	t.Run("exists", func(t *testing.T) {
		ctx := context.TODO()
		fromMock := gitalypb_mock.NewMockRepositoryServiceClient(t)
		toMock := gitalypb_mock.NewMockRepositoryServiceClient(t)
		helper := &CloneStorageHelper{
			from: fromMock,
			to:   toMock,
		}
		req := &ProjectStorageCloneRequest{
			CurrentGitalyAddress: "gca",
			CurrentGitalyToken:   "gct",
			CurrentGitalyStorage: "gcs",
			NewGitalyAddress:     "nca",
			NewGitalyToken:       "nct",
			NewGitalyStorage:     "ncs",
			FilesServer:          "http://foo.com/",
		}
		fromMock.EXPECT().RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "gcs",
				RelativePath: "foo",
			}}).Return(&gitalypb.RepositoryExistsResponse{Exists: true}, nil)
		toMock.EXPECT().RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "ncs",
				RelativePath: "foo",
			}}).Return(&gitalypb.RepositoryExistsResponse{Exists: true}, nil)
		err := helper.CloneRepoStorage(ctx, "foo", req)
		require.NoError(t, err)
	})
}
