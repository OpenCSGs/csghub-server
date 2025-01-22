package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestGitHTTPComponent_InfoRefs(t *testing.T) {

	cases := []struct {
		rpc     string
		private bool
	}{
		{"foo", true},
		{"git-receive-pack", false},
		{"foo", false},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			ctx := context.TODO()
			gc := initializeTestGitHTTPComponent(ctx, t)

			gc.mocks.stores.RepoMock().EXPECT().FindByPath(
				ctx, types.ModelRepo, "ns", "n",
			).Return(&database.Repository{
				Private: c.private,
			}, nil)
			if c.rpc == "git-receive-pack" {
				gc.mocks.components.repo.EXPECT().AllowWriteAccess(ctx, types.ModelRepo, "ns", "n", "user").Return(true, nil)
			}
			if c.private {
				gc.mocks.components.repo.EXPECT().AllowReadAccess(ctx, types.ModelRepo, "ns", "n", "user").Return(true, nil)
			}

			gc.mocks.gitServer.EXPECT().InfoRefsResponse(ctx, gitserver.InfoRefsReq{
				Namespace:   "ns",
				Name:        "n",
				Rpc:         c.rpc,
				RepoType:    types.ModelRepo,
				GitProtocol: "",
			}).Return(nil, nil)

			r, err := gc.InfoRefs(ctx, types.InfoRefsReq{
				Namespace:   "ns",
				Name:        "n",
				Rpc:         c.rpc,
				RepoType:    types.ModelRepo,
				CurrentUser: "user",
			})
			require.Nil(t, err)
			require.Equal(t, nil, r)

		})
	}

}

func TestGitHTTPComponent_GitUploadPack(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitHTTPComponent(ctx, t)

	gc.mocks.stores.RepoMock().EXPECT().FindByPath(
		ctx, types.ModelRepo, "ns", "n",
	).Return(&database.Repository{
		Private: true,
	}, nil)
	gc.mocks.components.repo.EXPECT().AllowReadAccess(ctx, types.ModelRepo, "ns", "n", "user").Return(true, nil)
	gc.mocks.gitServer.EXPECT().UploadPack(ctx, gitserver.UploadPackReq{
		Namespace: "ns",
		Name:      "n",
		Request:   nil,
		RepoType:  types.ModelRepo,
	}).Return(nil)
	err := gc.GitUploadPack(ctx, types.GitUploadPackReq{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
	})
	require.Nil(t, err)

}

func TestGitHTTPComponent_GitReceivePack(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitHTTPComponent(ctx, t)

	gc.mocks.stores.RepoMock().EXPECT().FindByPath(
		ctx, types.ModelRepo, "ns", "n",
	).Return(&database.Repository{
		Private: true,
	}, nil)
	gc.mocks.components.repo.EXPECT().AllowWriteAccess(ctx, types.ModelRepo, "ns", "n", "user").Return(true, nil)
	gc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{}, nil)
	gc.mocks.gitServer.EXPECT().ReceivePack(ctx, gitserver.UploadPackReq{
		Namespace: "ns",
		Name:      "n",
		Request:   nil,
		RepoType:  types.ModelRepo,
	}).Return(nil)
	err := gc.GitReceivePack(ctx, types.GitUploadPackReq{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
	})
	require.Nil(t, err)

}

func TestGitHTTPComponent_BuildObjectResponse(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitHTTPComponent(ctx, t)

	oid1 := "a3f8e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e"
	oid2 := "c39e7f5f1d61fa58ec6dbcd3b60a50870c577f0988d7c080fc88d1b460e7f5f1"
	gc.mocks.stores.RepoMock().EXPECT().FindByPath(
		ctx, types.ModelRepo, "ns", "n",
	).Return(&database.Repository{
		ID:      123,
		Private: true,
	}, nil)
	gc.mocks.s3Client.EXPECT().StatObject(
		ctx, "",
		"lfs/a3/f8/e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e",
		minio.StatObjectOptions{},
	).Return(
		minio.ObjectInfo{Size: 100}, nil,
	)
	gc.mocks.s3Client.EXPECT().StatObject(
		ctx, "",
		"lfs/c3/9e/7f5f1d61fa58ec6dbcd3b60a50870c577f0988d7c080fc88d1b460e7f5f1",
		minio.StatObjectOptions{},
	).Return(
		minio.ObjectInfo{Size: 100}, nil,
	)
	gc.mocks.stores.LfsMetaObjectMock().EXPECT().FindByOID(ctx, int64(123), oid1).Return(
		&database.LfsMetaObject{}, nil,
	)
	gc.mocks.stores.LfsMetaObjectMock().EXPECT().FindByOID(ctx, int64(123), oid2).Return(
		nil, nil,
	)
	gc.mocks.components.repo.EXPECT().AllowWriteAccess(
		ctx, types.ModelRepo, "ns", "n", "user",
	).Return(true, nil)
	gc.mocks.stores.LfsMetaObjectMock().EXPECT().Create(ctx, database.LfsMetaObject{
		Oid:          oid2,
		Size:         100,
		RepositoryID: 123,
		Existing:     true,
	}).Return(nil, nil)

	resp, err := gc.BuildObjectResponse(ctx, types.BatchRequest{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
		Objects: []types.Pointer{
			{
				Oid:  oid1,
				Size: 5,
			},
			{
				Oid:  oid2,
				Size: 100,
			},
		},
	}, true)
	require.Nil(t, err)
	require.Equal(t, &types.BatchResponse{
		Objects: []*types.ObjectResponse{
			{
				Pointer: types.Pointer{Oid: oid1, Size: 5},
				Error: &types.ObjectError{
					Code:    422,
					Message: "Object a3f8e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e is not 5 bytes",
				},
				Actions: nil,
			},
			{
				Pointer: types.Pointer{Oid: oid2, Size: 100},
				Actions: map[string]*types.Link{},
			},
		},
	}, resp)

}

func TestGitHTTPComponent_LfsUpload(t *testing.T) {

	for _, exist := range []bool{false, true} {
		t.Run(fmt.Sprintf("exist %v", exist), func(t *testing.T) {
			ctx := context.TODO()
			gc := initializeTestGitHTTPComponent(ctx, t)

			rc := io.NopCloser(&io.LimitedReader{})
			oid := "a3f8e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e"
			gc.mocks.stores.RepoMock().EXPECT().FindByPath(
				ctx, types.ModelRepo, "ns", "n",
			).Return(&database.Repository{
				ID:      123,
				Private: true,
			}, nil)
			if exist {
				gc.mocks.s3Client.EXPECT().StatObject(
					ctx, "",
					"lfs/a3/f8/e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e",
					minio.StatObjectOptions{},
				).Return(
					minio.ObjectInfo{Size: 100}, nil,
				)
			} else {
				gc.mocks.s3Client.EXPECT().StatObject(
					ctx, "",
					"lfs/a3/f8/e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e",
					minio.StatObjectOptions{},
				).Return(
					minio.ObjectInfo{Size: 100}, errors.New("zzzz"),
				)
			}

			gc.mocks.components.repo.EXPECT().AllowWriteAccess(
				ctx, types.ModelRepo, "ns", "n", "user",
			).Return(true, nil)

			if !exist {
				gc.mocks.s3Client.EXPECT().PutObject(
					ctx, "",
					"lfs/a3/f8/e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e",
					rc, int64(100), minio.PutObjectOptions{
						ContentType:           "application/octet-stream",
						SendContentMd5:        true,
						ConcurrentStreamParts: true,
						NumThreads:            5,
					}).Return(minio.UploadInfo{Size: 100}, nil)

				gc.mocks.stores.LfsMetaObjectMock().EXPECT().Create(ctx, database.LfsMetaObject{
					Oid:          oid,
					Size:         100,
					RepositoryID: 123,
					Existing:     true,
				}).Return(nil, nil)
			}

			err := gc.LfsUpload(ctx, rc, types.UploadRequest{
				Oid:         oid,
				Size:        100,
				CurrentUser: "user",
				Namespace:   "ns",
				Name:        "n",
				RepoType:    types.ModelRepo,
			})
			require.Nil(t, err)
		})
	}

}

func TestGitHTTPComponent_LfsVerify(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitHTTPComponent(ctx, t)

	gc.mocks.stores.RepoMock().EXPECT().FindByPath(
		ctx, types.ModelRepo, "ns", "n",
	).Return(&database.Repository{
		ID:      123,
		Private: true,
	}, nil)

	gc.mocks.s3Client.EXPECT().StatObject(ctx, "", "lfs/oid", minio.StatObjectOptions{}).Return(
		minio.ObjectInfo{Size: 100}, nil,
	)
	gc.mocks.stores.LfsMetaObjectMock().EXPECT().FindByOID(ctx, int64(123), "oid").Return(nil, sql.ErrNoRows)
	gc.mocks.stores.LfsMetaObjectMock().EXPECT().Create(ctx, database.LfsMetaObject{
		Oid:          "oid",
		Size:         100,
		RepositoryID: 123,
		Existing:     true,
	}).Return(nil, nil)

	err := gc.LfsVerify(ctx, types.VerifyRequest{
		CurrentUser: "user",
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
	}, types.Pointer{Oid: "oid", Size: 100})
	require.Nil(t, err)

}

func TestGitHTTPComponent_CreateLock(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitHTTPComponent(ctx, t)

	gc.mocks.stores.RepoMock().EXPECT().FindByPath(
		ctx, types.ModelRepo, "ns", "n",
	).Return(&database.Repository{
		ID:      123,
		Private: true,
	}, nil)

	gc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{}, nil)
	gc.mocks.components.repo.EXPECT().AllowWriteAccess(
		ctx, types.ModelRepo, "ns", "n", "user",
	).Return(true, nil)
	lfslock := &database.LfsLock{Path: "path", RepositoryID: 123}
	gc.mocks.stores.LfsLockMock().EXPECT().FindByPath(ctx, int64(123), "path").Return(
		lfslock, sql.ErrNoRows,
	)
	gc.mocks.stores.LfsLockMock().EXPECT().Create(ctx, *lfslock).Return(lfslock, nil)

	l, err := gc.CreateLock(ctx, types.LfsLockReq{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
		Path:        "path",
	})
	require.Nil(t, err)
	require.Equal(t, lfslock, l)

}

func TestGitHTTPComponent_ListLocks(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitHTTPComponent(ctx, t)

	gc.mocks.stores.RepoMock().EXPECT().FindByPath(
		ctx, types.ModelRepo, "ns", "n",
	).Return(&database.Repository{
		ID:      123,
		Private: true,
	}, nil)

	gc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{}, nil)
	gc.mocks.components.repo.EXPECT().AllowReadAccess(
		ctx, types.ModelRepo, "ns", "n", "user",
	).Return(true, nil)

	gc.mocks.stores.LfsLockMock().EXPECT().FindByID(ctx, int64(123)).Return(
		&database.LfsLock{ID: 11, RepositoryID: 123}, nil,
	)
	gc.mocks.stores.LfsLockMock().EXPECT().FindByPath(ctx, int64(123), "foo/bar").Return(
		&database.LfsLock{ID: 12, RepositoryID: 123}, nil,
	)
	gc.mocks.stores.LfsLockMock().EXPECT().FindByRepoID(ctx, int64(123), 1, 10).Return(
		[]database.LfsLock{{ID: 13, RepositoryID: 123}}, nil,
	)

	req := types.ListLFSLockReq{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
		Cursor:      1,
		Limit:       10,
	}
	req1 := req
	req1.ID = 123
	ll, err := gc.ListLocks(ctx, req1)
	require.Nil(t, err)
	require.Equal(t, &types.LFSLockList{
		Locks: []*types.LFSLock{{ID: "11", Owner: &types.LFSLockOwner{}}},
	}, ll)
	req2 := req
	req2.Path = "foo/bar"
	ll, err = gc.ListLocks(ctx, req2)
	require.Nil(t, err)
	require.Equal(t, &types.LFSLockList{
		Locks: []*types.LFSLock{{ID: "12", Owner: &types.LFSLockOwner{}}},
	}, ll)
	ll, err = gc.ListLocks(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &types.LFSLockList{
		Locks: []*types.LFSLock{{ID: "13", Owner: &types.LFSLockOwner{}}},
	}, ll)
}

func TestGitHTTPComponent_UnLock(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitHTTPComponent(ctx, t)

	gc.mocks.stores.RepoMock().EXPECT().FindByPath(
		ctx, types.ModelRepo, "ns", "n",
	).Return(&database.Repository{
		ID:      123,
		Private: true,
	}, nil)

	gc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{}, nil)
	gc.mocks.components.repo.EXPECT().AllowWriteAccess(
		ctx, types.ModelRepo, "ns", "n", "user",
	).Return(true, nil)

	gc.mocks.stores.LfsLockMock().EXPECT().FindByID(ctx, int64(123)).Return(
		&database.LfsLock{ID: 11, RepositoryID: 123}, nil,
	)
	gc.mocks.stores.LfsLockMock().EXPECT().RemoveByID(ctx, int64(123)).Return(nil)

	lk, err := gc.UnLock(ctx, types.UnlockLFSReq{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
		ID:          123,
	})
	require.Nil(t, err)
	require.Equal(t, &database.LfsLock{
		ID:           11,
		RepositoryID: 123,
	}, lk)

}

func TestGitHTTPComponent_VerifyLock(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitHTTPComponent(ctx, t)

	gc.mocks.stores.RepoMock().EXPECT().FindByPath(
		ctx, types.ModelRepo, "ns", "n",
	).Return(&database.Repository{
		ID:      123,
		Private: true,
	}, nil)

	gc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{}, nil)
	gc.mocks.components.repo.EXPECT().AllowReadAccess(
		ctx, types.ModelRepo, "ns", "n", "user",
	).Return(true, nil)

	gc.mocks.stores.LfsLockMock().EXPECT().FindByRepoID(ctx, int64(123), 10, 1).Return(
		[]database.LfsLock{{ID: 11, RepositoryID: 123, User: database.User{Username: "zzz"}}}, nil,
	)

	lk, err := gc.VerifyLock(ctx, types.VerifyLFSLockReq{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
		Cursor:      10,
		Limit:       1,
	})
	require.Nil(t, err)
	require.Equal(t, &types.LFSLockListVerify{
		Ours: []*types.LFSLock{{ID: "11", Owner: &types.LFSLockOwner{Name: "zzz"}}},
		Next: "11",
	}, lk)

}

func TestGitHTTPComponent_LfsDownload(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitHTTPComponent(ctx, t)

	gc.mocks.stores.RepoMock().EXPECT().FindByPath(
		ctx, types.ModelRepo, "ns", "n",
	).Return(&database.Repository{
		ID:      123,
		Private: true,
	}, nil)

	gc.mocks.components.repo.EXPECT().AllowReadAccess(
		ctx, types.ModelRepo, "ns", "n", "user",
	).Return(true, nil)
	gc.mocks.stores.LfsMetaObjectMock().EXPECT().FindByOID(ctx, int64(123), "oid").Return(nil, nil)
	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", "sa"))
	url := &url.URL{Scheme: "http"}
	gc.mocks.s3Client.EXPECT().PresignedGetObject(ctx, "", "lfs/oid", ossFileExpire, reqParams).Return(url, nil)

	u, err := gc.LfsDownload(ctx, types.DownloadRequest{
		Oid:         "oid",
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		SaveAs:      "sa",
		CurrentUser: "user",
	})
	require.Nil(t, err)
	require.Equal(t, url, u)

}
