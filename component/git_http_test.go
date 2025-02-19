package component

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
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

func TestGitHTTPComponent_Batch(t *testing.T) {
	existOID := "a3f8e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e"
	notExistOID := "c39e7f5f1d61fa58ec6dbcd3b60a50870c577f0988d7c080fc88d1b460e7f5f1"
	cases := []struct {
		name           string
		hasReadAccess  bool
		hasWriteAccess bool
		operation      types.LFSBatchOperation
		exist          bool
		err            error
		readAccessErr  error
		resp           *types.BatchResponse
		noUser         bool
	}{
		{
			name:          "download success",
			hasReadAccess: true,
			operation:     types.LFSBatchDownload,
			exist:         true,
			resp: &types.BatchResponse{
				Objects: []*types.ObjectResponse{
					{
						Pointer: types.Pointer{Oid: existOID, Size: 100},
						Actions: map[string]*types.Link{
							"download": {
								Href:   "http://foo.com/bar",
								Header: map[string]string{},
							},
						},
					},
				},
			},
		},
		{
			name:      "download no read access, no write access",
			operation: types.LFSBatchDownload,
			exist:     true,
			err:       ErrNotFound,
		},
		{
			name:           "download no read access, has write access",
			operation:      types.LFSBatchDownload,
			exist:          true,
			hasWriteAccess: true,
			err:            ErrNotFound,
		},
		{
			name:          "download file not exist",
			operation:     types.LFSBatchDownload,
			exist:         false,
			hasReadAccess: true,
			resp: &types.BatchResponse{
				Objects: []*types.ObjectResponse{
					{
						Error: &types.ObjectError{
							Code:    404,
							Message: "Object does not exist",
						},
					},
				},
			},
		},
		{
			name:           "upload success",
			operation:      types.LFSBatchUpload,
			hasWriteAccess: true,
			resp: &types.BatchResponse{
				Objects: []*types.ObjectResponse{
					{
						Pointer: types.Pointer{Oid: notExistOID, Size: 100},
						Actions: map[string]*types.Link{
							"upload": {
								Href:   "https://foo.com/models/ns/n.git/info/lfs/objects/" + notExistOID + "/100",
								Header: map[string]string{},
							},
							"verify": {
								Href: "https://foo.com/models/ns/n.git/info/lfs/verify",
								Header: map[string]string{
									"Accept": "application/vnd.git-lfs+json",
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "upload no read access, no write access",
			operation: types.LFSBatchUpload,
			err:       ErrNotFound,
		},
		{
			name:          "upload has read access, no write access",
			operation:     types.LFSBatchUpload,
			hasReadAccess: true,
			err:           ErrForbidden,
		},
		{
			name:           "upload file exist",
			operation:      types.LFSBatchUpload,
			hasWriteAccess: true,
			exist:          true,
			resp: &types.BatchResponse{
				Objects: []*types.ObjectResponse{
					{
						Pointer: types.Pointer{Oid: existOID, Size: 100},
						Actions: nil,
					},
				},
			},
		},
		{
			name:      "upload and current user empty, 401",
			operation: types.LFSBatchUpload,
			err:       ErrUnauthorized,
			noUser:    true,
		},
		{
			name:          "download and current user empty, 401",
			operation:     types.LFSBatchDownload,
			err:           ErrUnauthorized,
			readAccessErr: ErrUserNotFound,
			noUser:        true,
		},
		{
			name:          "download and user not found",
			operation:     types.LFSBatchDownload,
			err:           ErrUserNotFound,
			readAccessErr: ErrUserNotFound,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()
			gc := initializeTestGitHTTPComponent(ctx, t)
			oid := existOID
			if !c.exist {
				oid = notExistOID
			}
			path := path.Join(oid[0:2], oid[2:4], oid[4:])
			user := "user"
			if c.noUser {
				user = ""
			}

			gc.mocks.stores.RepoMock().EXPECT().FindByPath(
				ctx, types.ModelRepo, "ns", "n",
			).Return(&database.Repository{
				ID:      123,
				Private: true,
			}, nil).Maybe()

			gc.mocks.components.repo.EXPECT().AllowReadAccess(
				ctx, types.ModelRepo, "ns", "n", user,
			).Return(c.hasReadAccess, c.readAccessErr).Maybe()
			gc.mocks.components.repo.EXPECT().AllowWriteAccess(
				ctx, types.ModelRepo, "ns", "n", user,
			).Return(c.hasWriteAccess, nil).Maybe()
			gc.mocks.stores.LfsMetaObjectMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(
				[]database.LfsMetaObject{{
					Oid: existOID,
				}}, nil,
			).Maybe()
			if c.operation == types.LFSBatchDownload {
				reqParams := make(url.Values)
				url := &url.URL{Scheme: "http", Host: "foo.com", Path: "bar"}
				gc.mocks.s3Client.EXPECT().PresignedGetObject(
					ctx, "", "lfs/"+path, types.OssFileExpire, reqParams,
				).Return(url, nil).Maybe()
			}

			resp, err := gc.LFSBatch(ctx, types.BatchRequest{
				Operation:   c.operation,
				Namespace:   "ns",
				Name:        "n",
				RepoType:    types.ModelRepo,
				CurrentUser: user,
				Objects: []types.Pointer{
					{Oid: oid, Size: 100},
				},
			})
			if err != nil {
				require.ErrorIs(t, err, c.err)
			} else {
				require.Equal(t, c.resp, resp)
			}
		})
	}
}

func TestGitHTTPComponent_LfsUpload(t *testing.T) {

	for _, exist := range []bool{false, true} {
		t.Run(fmt.Sprintf("exist %v", exist), func(t *testing.T) {
			ctx := context.TODO()
			gc := initializeTestGitHTTPComponent(ctx, t)

			rc := io.NopCloser(&io.LimitedReader{})
			oid := "a3f8e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e"
			var size int64 = 100
			if exist {
				oid = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
				size = 0
			}
			gc.mocks.stores.RepoMock().EXPECT().FindByPath(
				ctx, types.ModelRepo, "ns", "n",
			).Return(&database.Repository{
				ID:      123,
				Private: true,
			}, nil)
			if exist {
				gc.mocks.s3Client.EXPECT().StatObject(
					ctx, "",
					"lfs/e3/b0/c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					minio.StatObjectOptions{},
				).Return(
					minio.ObjectInfo{Size: size}, nil,
				)
			} else {
				gc.mocks.s3Client.EXPECT().StatObject(
					ctx, "",
					"lfs/a3/f8/e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e",
					minio.StatObjectOptions{},
				).Return(
					minio.ObjectInfo{Size: 100}, os.ErrNotExist,
				)
			}
			gc.mocks.components.repo.EXPECT().AllowWriteAccess(
				ctx, types.ModelRepo, "ns", "n", "user",
			).Return(true, nil)

			if !exist {
				gc.mocks.s3Client.EXPECT().UploadAndValidate(
					ctx, "",
					"lfs/a3/f8/e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e",
					rc, int64(100)).Return(minio.UploadInfo{Size: 100}, nil)

				gc.mocks.stores.LfsMetaObjectMock().EXPECT().Create(ctx, database.LfsMetaObject{
					Oid:          oid,
					Size:         size,
					RepositoryID: 123,
					Existing:     true,
				}).Return(nil, nil)
			}

			err := gc.LfsUpload(ctx, rc, types.UploadRequest{
				Oid:         oid,
				Size:        size,
				CurrentUser: "user",
				Namespace:   "ns",
				Name:        "n",
				RepoType:    types.ModelRepo,
			})
			require.Nil(t, err)
		})
	}

	t.Run("exist but sha256 not match", func(t *testing.T) {
		ctx := context.TODO()
		gc := initializeTestGitHTTPComponent(ctx, t)

		rc := io.NopCloser(&io.LimitedReader{})
		oid := "a3f8e1b4f77bb24e508906c6972f81928f0d926e6daef1b29d12e348b8a3547e"
		var size int64

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
			minio.ObjectInfo{Size: size}, nil,
		)
		gc.mocks.components.repo.EXPECT().AllowWriteAccess(
			ctx, types.ModelRepo, "ns", "n", "user",
		).Return(true, nil)

		err := gc.LfsUpload(ctx, rc, types.UploadRequest{
			Oid:         oid,
			Size:        size,
			CurrentUser: "user",
			Namespace:   "ns",
			Name:        "n",
			RepoType:    types.ModelRepo,
		})
		require.Equal(t, "invalid lfs size or oid", err.Error())
	})

}

func TestGitHTTPComponent_LfsVerify(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitHTTPComponent(ctx, t)

	gc.mocks.s3Client.EXPECT().StatObject(ctx, "", "lfs/oid", minio.StatObjectOptions{}).Return(
		minio.ObjectInfo{Size: 100}, nil,
	)

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
	gc.mocks.s3Client.EXPECT().PresignedGetObject(ctx, "", "lfs/oid", types.OssFileExpire, reqParams).Return(url, nil)

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
