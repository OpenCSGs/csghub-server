package component

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/deploy"
	deployStatus "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoComponent_CreateRepo(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil)
	dbuser := database.User{
		ID:       123,
		RoleMask: "admin",
		Email:    "foo@bar.com",
	}
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(dbuser, nil)
	gitrepo := &gitserver.CreateRepoResp{
		GitPath:       "gp",
		DefaultBranch: "main",
		HttpCloneURL:  "http",
		SshCloneURL:   "ssh",
	}
	repo.mocks.stores.RecomMock().EXPECT().UpsertScore(ctx, mock.Anything).Return(nil)
	repo.mocks.gitServer.EXPECT().CreateRepo(ctx, mock.AnythingOfType("gitserver.CreateRepoReq")).Return(gitrepo, nil)
	dbrepo := &database.Repository{
		UserID:         123,
		Path:           "ns/name",
		GitPath:        "models_ns/name",
		Name:           "name",
		Nickname:       "nn",
		Description:    "desc",
		Private:        true,
		License:        "MIT",
		DefaultBranch:  "main",
		RepositoryType: types.ModelRepo,
	}
	repo.mocks.stores.RepoMock().EXPECT().CreateRepo(ctx, mock.AnythingOfType("database.Repository")).Return(dbrepo, nil)
	r1, r2, _, err := repo.CreateRepo(ctx, types.CreateRepoReq{
		Username:      "user",
		Namespace:     "ns",
		Name:          "name",
		Nickname:      "nn",
		License:       "MIT",
		DefaultBranch: "main",
		Readme:        "rr",
		Private:       true,
		RepoType:      types.ModelRepo,
		Description:   "desc",
		CommitFiles: []types.CommitFile{
			{
				Content: "content",
				Path:    "path",
			},
		},
	})
	require.Nil(t, err)
	require.Equal(t, gitrepo, r1)
	dbrepo.User = dbuser
	require.Equal(t, dbrepo, r2)

}

func TestRepoComponent_UpdateRepo(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbrepo := &database.Repository{
		UserID:         123,
		Path:           "ns/n",
		GitPath:        "gp",
		Name:           "n",
		Nickname:       "nn",
		Description:    "desc",
		Private:        true,
		License:        "MIT",
		DefaultBranch:  "main",
		RepositoryType: types.ModelRepo,
		HTTPCloneURL:   "http",
		SSHCloneURL:    "ssh",
	}
	repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "ns", string(types.ModelRepo), "n").Return(dbrepo, nil)
	repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil)
	dbuser := database.User{
		ID:       123,
		RoleMask: "admin",
		Email:    "foo@bar.com",
	}
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(dbuser, nil)
	gitrepo := &gitserver.CreateRepoResp{
		GitPath:       "gp",
		DefaultBranch: "main",
		HttpCloneURL:  "http",
		SshCloneURL:   "ssh",
	}
	repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, gitserver.UpdateRepoReq{
		Namespace:     "ns",
		Name:          "n",
		Nickname:      "nn2",
		Description:   "desc2",
		DefaultBranch: "main",
		Private:       true,
		RepoType:      types.ModelRepo,
	}).Return(gitrepo, nil)

	dbrepo.Nickname = "nn2"
	dbrepo.Description = "desc2"
	repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, *dbrepo).Return(dbrepo, nil)

	r1, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
		Username:    "user",
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		Nickname:    tea.String("nn2"),
		Description: tea.String("desc2"),
		Private:     tea.Bool(true),
	})
	require.Nil(t, err)
	require.Equal(t, dbrepo, r1)

}

func TestRepoComponent_DeleteRepo(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbrepo := &database.Repository{
		ID:             1,
		UserID:         123,
		Path:           "ns/n",
		GitPath:        "gp",
		Name:           "n",
		Nickname:       "nn",
		Description:    "desc",
		Private:        true,
		License:        "MIT",
		DefaultBranch:  "main",
		RepositoryType: types.ModelRepo,
		HTTPCloneURL:   "http",
		SSHCloneURL:    "ssh",
	}
	repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "ns", string(types.ModelRepo), "n").Return(dbrepo, nil)
	repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil)
	dbuser := database.User{
		ID:       123,
		RoleMask: "admin",
		Email:    "foo@bar.com",
	}
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(dbuser, nil)
	repo.mocks.stores.RepoMock().EXPECT().CleanRelationsByRepoID(ctx, dbrepo.ID).Return(nil)
	repo.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, dbrepo.ID).Return(nil, nil)

	repo.mocks.gitServer.EXPECT().DeleteRepo(ctx, "models_ns/n.git").Return(nil)

	repo.mocks.stores.RepoMock().EXPECT().DeleteRepo(ctx, *dbrepo).Return(nil)

	r1, err := repo.DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "user",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.ModelRepo,
	})
	require.Nil(t, err)
	require.Equal(t, dbrepo, r1)

}

func TestRepoComponent_PublicToUser(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	repo.mocks.userSvcClient.EXPECT().GetUserInfo(ctx, "user", "user").Return(&rpc.User{
		ID:    1,
		Roles: []string{"a", "b"},
		Orgs: []rpc.Organization{
			{UserID: 2},
			{UserID: 3},
		},
	}, nil)

	filter := &types.RepoFilter{}
	mrepos := []*database.Repository{
		{Name: "foo"},
	}
	repo.mocks.stores.RepoMock().EXPECT().PublicToUser(ctx, types.ModelRepo, []int64{1, 2, 3}, filter, 10, 1, false).Return(mrepos, 100, nil)

	repos, count, err := repo.PublicToUser(ctx, types.ModelRepo, "user", &types.RepoFilter{}, 10, 1)
	require.Equal(t, mrepos, repos)
	require.Equal(t, 100, count)
	require.Nil(t, err)
}

func mockUserRepoAdminPermission(ctx context.Context, stores *tests.MockStores, userName string) {
	stores.UserMock().EXPECT().FindByUsername(ctx, userName).Return(database.User{
		RoleMask: "admin",
	}, nil).Once()
	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(
		database.Namespace{NamespaceType: "user"}, nil,
	).Maybe()

}

func TestRepoComponent_RelatedRepos(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	froms := []*database.RepoRelation{{ToRepoID: 1}, {ToRepoID: 2}}
	repo.mocks.stores.RepoRelationMock().EXPECT().From(ctx, int64(123)).Return(froms, nil)
	tos := []*database.RepoRelation{{FromRepoID: 3}, {FromRepoID: 4}}
	repo.mocks.stores.RepoRelationMock().EXPECT().To(ctx, int64(123)).Return(tos, nil)

	var opts []interface{}
	opts = append(opts, database.Columns("id", "repository_type", "path", "user_id", "private", "name",
		"nickname", "description", "download_count", "updated_at"))

	repos := []*database.Repository{
		{Private: false, RepositoryType: types.ModelRepo, Path: "a/b"},
		{Private: false, RepositoryType: types.ModelRepo, Path: "a/c"},
		{Private: true, RepositoryType: types.DatasetRepo, Path: "b/e"},
		{Private: true, RepositoryType: types.DatasetRepo, Path: "user/f"},
	}
	repo.mocks.stores.RepoMock().EXPECT().FindByIds(ctx, []int64{1, 2, 3, 4}, opts...).Return(repos, nil)

	repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "b").Return(database.Namespace{
		NamespaceType: "user",
	}, nil)
	repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "user").Return(database.Namespace{
		NamespaceType: "user",
	}, nil)
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{RoleMask: "foo"}, nil)

	related, err := repo.RelatedRepos(ctx, 123, "user")
	require.Nil(t, err)
	require.Equal(t, map[types.RepositoryType][]*database.Repository{
		types.ModelRepo:   {repos[0], repos[1]},
		types.DatasetRepo: {repos[3]},
	}, related)

}

func TestRepoComponent_CreateFile(t *testing.T) {

	cases := []struct {
		useLFS     bool
		path       string
		testUpload bool
	}{
		{false, "test.go", false},
		{false, "README.md", false},
		{true, "test.go", false},
		{false, "test.go", true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			ctx := context.TODO()
			repo := initializeTestRepoComponent(ctx, t)

			mockedRepo := &database.Repository{ID: 123}
			repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(mockedRepo, nil)
			mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
			repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "un").Return(database.User{
				Email: "foo@bar.com",
			}, nil)
			repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil)
			if c.useLFS {
				repo.config.GitServer.Type = types.GitServerTypeGitaly
				ct := base64.RawStdEncoding.EncodeToString(
					[]byte(c.path + " filter=lfs diff=lfs merge=lfs -text"),
				)
				repo.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
					RepoType:  types.ModelRepo,
					Namespace: "ns",
					Name:      "n",
					Ref:       "main",
					Path:      GitAttributesFileName,
				}).Return(&types.File{
					Content: ct,
				}, nil)
				repo.mocks.s3Client.EXPECT().PutObject(
					mock.Anything, repo.config.S3.Bucket,
					"lfs/e3/b0/c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", mock.Anything, int64(0), minio.PutObjectOptions{}).Return(minio.UploadInfo{
					Size: 0,
				}, nil)
				repo.mocks.stores.LfsMetaObjectMock().EXPECT().UpdateOrCreate(mock.Anything, database.LfsMetaObject{
					Oid:          "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					Size:         0,
					RepositoryID: 123,
					Existing:     true,
				}).Return(nil, nil)
			}

			repo.mocks.stores.RepoMock().EXPECT().SetUpdateTimeByPath(mock.Anything, types.ModelRepo, "ns", "n", mock.Anything).Return(nil)
			if c.path == "README.md" {
				repo.mocks.components.tag.EXPECT().UpdateMetaTags(mock.Anything, getTagScopeByRepoType(types.ModelRepo), "ns", "n", "").Return(nil, nil)
			} else {
				repo.mocks.components.tag.EXPECT().UpdateLibraryTags(mock.Anything, getTagScopeByRepoType(types.ModelRepo), "ns", "n", "", c.path).Return(nil)
			}
			req := &types.CreateFileReq{
				RepoType:        types.ModelRepo,
				Namespace:       "ns",
				Name:            "n",
				CurrentUser:     "user",
				Username:        "un",
				Branch:          "main",
				FilePath:        c.path,
				OriginalContent: []byte{},
			}
			repo.mocks.gitServer.EXPECT().CreateRepoFile(req).Return(nil)

			if c.testUpload {
				// GetRepoFileContents return error, create
				repo.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: req.Namespace,
					Name:      req.Name,
					Ref:       req.Branch,
					Path:      req.FilePath,
					RepoType:  req.RepoType,
				}).Return(nil, errors.New("not allowed")).Once()
				err := repo.UploadFile(ctx, req)
				require.Nil(t, err)
			} else {

				resp, err := repo.CreateFile(ctx, req)
				require.Nil(t, err)
				require.Equal(t, &types.CreateFileResp{}, resp)
			}
		})
	}
}

func TestRepoComponent_UpdateFile(t *testing.T) {

	cases := []struct {
		useLFS     bool
		path       string
		testUpload bool
	}{
		{false, "test.go", false},
		{false, "README.md", false},
		{true, "test.go", false},
		{false, "test.go", true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			ctx := context.TODO()
			repo := initializeTestRepoComponent(ctx, t)

			mockedRepo := &database.Repository{ID: 123}
			repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(mockedRepo, nil)
			mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
			repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "un").Return(database.User{
				Email: "foo@bar.com",
			}, nil)
			repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil)
			if c.useLFS {
				repo.config.GitServer.Type = types.GitServerTypeGitaly
				ct := base64.RawStdEncoding.EncodeToString(
					[]byte(c.path + " filter=lfs diff=lfs merge=lfs -text"),
				)
				repo.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
					RepoType:  types.ModelRepo,
					Namespace: "ns",
					Name:      "n",
					Ref:       "main",
					Path:      GitAttributesFileName,
				}).Return(&types.File{
					Content: ct,
				}, nil)
				repo.mocks.s3Client.EXPECT().PutObject(
					mock.Anything, repo.config.S3.Bucket,
					"lfs/af/a7/518106309c22d325df6d2663249d158d2f36f1976269d6d4104d9198a108", mock.Anything, int64(4), minio.PutObjectOptions{}).Return(minio.UploadInfo{
					Size: 4,
				}, nil)
				repo.mocks.stores.LfsMetaObjectMock().EXPECT().UpdateOrCreate(mock.Anything, database.LfsMetaObject{
					Oid:          "afa7518106309c22d325df6d2663249d158d2f36f1976269d6d4104d9198a108",
					Size:         4,
					RepositoryID: 123,
					Existing:     true,
				}).Return(nil, nil)
			}

			repo.mocks.stores.RepoMock().EXPECT().SetUpdateTimeByPath(mock.Anything, types.ModelRepo, "ns", "n", mock.Anything).Return(nil)
			if c.path == "README.md" {
				repo.mocks.components.tag.EXPECT().UpdateMetaTags(mock.Anything, getTagScopeByRepoType(types.ModelRepo), "ns", "n", "").Return(nil, nil)
			} else {
				repo.mocks.components.tag.EXPECT().UpdateLibraryTags(mock.Anything, getTagScopeByRepoType(types.ModelRepo), "ns", "n", "", c.path).Return(nil)
			}
			req := &types.UpdateFileReq{
				RepoType:        types.ModelRepo,
				Namespace:       "ns",
				Name:            "n",
				CurrentUser:     "user",
				Username:        "un",
				Branch:          "main",
				FilePath:        c.path,
				OriginalContent: []byte{1, 0, 0, 1},
				Email:           "foo@bar.com",
			}
			repo.mocks.gitServer.EXPECT().UpdateRepoFile(req).Return(nil)

			if c.testUpload {
				// GetRepoFileContents success, update
				repo.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: req.Namespace,
					Name:      req.Name,
					Ref:       req.Branch,
					Path:      req.FilePath,
					RepoType:  req.RepoType,
				}).Return(&types.File{}, nil).Once()
				err := repo.UploadFile(ctx, &types.CreateFileReq{
					Username:        req.Username,
					Branch:          req.Branch,
					Namespace:       req.Namespace,
					Name:            req.Name,
					FilePath:        req.FilePath,
					RepoType:        req.RepoType,
					CurrentUser:     req.CurrentUser,
					OriginalContent: req.OriginalContent,
				})
				require.Nil(t, err)
			} else {
				resp, err := repo.UpdateFile(ctx, req)
				require.Nil(t, err)
				require.Equal(t, &types.UpdateFileResp{}, resp)
			}
		})
	}
}

func TestRepoComponent_DeleteFile(t *testing.T) {

	cases := []struct {
		path string
	}{
		{"test.go"},
		{"README.md"},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			ctx := context.TODO()
			repo := initializeTestRepoComponent(ctx, t)

			mockedRepo := &database.Repository{ID: 123}
			repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(mockedRepo, nil)
			mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
			repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "un").Return(database.User{
				Email: "foo@bar.com",
			}, nil)
			repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil)

			repo.mocks.stores.RepoMock().EXPECT().SetUpdateTimeByPath(mock.Anything, types.ModelRepo, "ns", "n", mock.Anything).Return(nil)
			if c.path == "README.md" {
				repo.mocks.components.tag.EXPECT().UpdateMetaTags(mock.Anything, getTagScopeByRepoType(types.ModelRepo), "ns", "n", "").Return(nil, nil)
			} else {
				repo.mocks.components.tag.EXPECT().UpdateLibraryTags(mock.Anything, getTagScopeByRepoType(types.ModelRepo), "ns", "n", "", c.path).Return(nil)
			}
			req := &types.DeleteFileReq{
				RepoType:        types.ModelRepo,
				Namespace:       "ns",
				Name:            "n",
				CurrentUser:     "user",
				Username:        "un",
				Branch:          "main",
				FilePath:        c.path,
				OriginalContent: []byte{1, 0, 0, 1},
			}
			repo.mocks.gitServer.EXPECT().DeleteRepoFile(req).Return(nil)

			resp, err := repo.DeleteFile(ctx, req)
			require.Nil(t, err)
			require.Equal(t, &types.DeleteFileResp{}, resp)
		})
	}
}

func TestRepoComponent_Commits(t *testing.T) {

	for _, user := range []string{"user", ""} {
		t.Run(user, func(t *testing.T) {
			ctx := context.TODO()
			repo := initializeTestRepoComponent(ctx, t)

			r := &database.Repository{Private: true}
			repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(r, nil)
			if user != "" {
				mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
				repo.mocks.gitServer.EXPECT().GetRepoCommits(ctx, gitserver.GetRepoCommitsReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       "main",
					Per:       10,
					Page:      1,
					RepoType:  types.ModelRepo,
				}).Return(nil, nil, nil)
			}

			a, b, err := repo.Commits(ctx, &types.GetCommitsReq{
				Namespace:   "ns",
				Name:        "n",
				RepoType:    types.ModelRepo,
				Per:         10,
				Page:        1,
				Ref:         "main",
				CurrentUser: user,
			})
			if user == "" {
				require.Equal(t, errorx.ErrUnauthorized, err)
				return
			}
			require.Nil(t, err)
			require.Nil(t, a)
			require.Nil(t, b)
		})
	}

}

func TestRepoComponent_FileRaw(t *testing.T) {

	cases := []struct {
		canRead  bool
		source   types.RepositorySource
		path     string
		mirrored bool
	}{
		{false, types.HuggingfaceSource, "test.go", false},
		{true, types.HuggingfaceSource, "test.go", false},
		{true, types.LocalSource, "README.md", false},
		{true, types.HuggingfaceSource, "README.md", false},
		{true, types.HuggingfaceSource, "README.md", true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			ctx := context.TODO()
			repo := initializeTestRepoComponent(ctx, t)

			r := &database.Repository{
				ID:      123,
				Private: true,
				Source:  c.source,
				Readme:  "readme1",
			}
			repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(r, nil)
			currentUser := ""
			if !c.canRead {
				_, err := repo.FileRaw(ctx, &types.GetFileReq{
					Namespace:   "ns",
					Name:        "n",
					RepoType:    types.ModelRepo,
					Ref:         "main",
					Path:        c.path,
					CurrentUser: currentUser,
				})
				require.True(t, errors.Is(err, errorx.ErrForbidden))
				return
			}

			currentUser = "user"
			mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
			rawContent := "readme1"
			if c.source != types.LocalSource && c.path == "README.md" {
				if c.mirrored {
					repo.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(nil, nil)
					repo.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
						Namespace: "ns",
						Name:      "n",
						Ref:       "main",
						Path:      c.path,
						RepoType:  types.ModelRepo,
					}).Return("readme2", nil)
					rawContent = "readme2"
				} else {
					repo.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(nil, sql.ErrNoRows)
				}
			} else {
				repo.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       "main",
					Path:      c.path,
					RepoType:  types.ModelRepo,
				}).Return("readme2", nil)
				rawContent = "readme2"
			}

			a, err := repo.FileRaw(ctx, &types.GetFileReq{
				Namespace:   "ns",
				Name:        "n",
				RepoType:    types.ModelRepo,
				Ref:         "main",
				Path:        c.path,
				CurrentUser: currentUser,
			})
			require.Nil(t, err)
			require.Equal(t, rawContent, a)
		})
	}

}

func TestRepoComponent_DownloadFile(t *testing.T) {
	for _, lfs := range []bool{false, true} {
		t.Run(fmt.Sprintf("is lfs: %v", lfs), func(t *testing.T) {
			ctx := context.TODO()
			repo := initializeTestRepoComponent(ctx, t)

			mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
			mockedRepo := &database.Repository{}
			repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
				mockedRepo, nil,
			)

			repo.mocks.stores.RepoMock().EXPECT().UpdateRepoFileDownloads(ctx, mockedRepo, mock.Anything, int64(1)).Return(nil)

			if lfs {
				reqParams := make(url.Values)
				reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", "zzz"))
				repo.mocks.s3Client.EXPECT().PresignedGetObject(
					ctx, repo.lfsBucket, "lfs/pa/th", types.OssFileExpire, reqParams,
				).Return(&url.URL{Path: "foobar"}, nil)
			} else {
				repo.mocks.gitServer.EXPECT().GetRepoFileReader(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       "main",
					Path:      "path",
					RepoType:  types.ModelRepo,
				}).Return(nil, 100, nil)
			}

			a, b, c, err := repo.DownloadFile(ctx, &types.GetFileReq{
				Namespace:   "ns",
				Name:        "n",
				Ref:         "main",
				Path:        "path",
				RepoType:    types.ModelRepo,
				Lfs:         lfs,
				SaveAs:      "zzz",
				CurrentUser: "user",
			}, "user")
			require.Nil(t, err)
			if lfs {
				require.Nil(t, a)
				require.Equal(t, int64(0), b)
				require.Equal(t, "foobar", c)
			} else {
				require.Nil(t, a)
				require.Equal(t, int64(100), b)
				require.Equal(t, "", c)
			}
		})
	}

}

func TestRepoComponent_InternalDownloadFile(t *testing.T) {
	for _, lfs := range []bool{true} {
		t.Run(fmt.Sprintf("is lfs: %v", lfs), func(t *testing.T) {
			ctx := context.TODO()
			repo := initializeTestRepoComponent(ctx, t)

			mockedRepo := &database.Repository{}
			repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
				mockedRepo, nil,
			)
			file := &types.File{Name: "zzz", LfsSHA256: "abcdefghi"}
			repo.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
				Namespace: "ns",
				Name:      "n",
				Ref:       "main",
				Path:      "path",
				RepoType:  types.ModelRepo,
			}).Return(file, nil)

			if lfs {
				reqParams := make(url.Values)
				reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", "zzz"))
				repo.mocks.s3Client.EXPECT().PresignedGetObject(
					ctx, repo.lfsBucket, "lfs/ab/cd/efghi", types.OssFileExpire, reqParams,
				).Return(&url.URL{Path: "foobar"}, nil)
			} else {
				repo.mocks.gitServer.EXPECT().GetRepoFileReader(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       "main",
					Path:      "path",
					RepoType:  types.ModelRepo,
				}).Return(nil, 100, nil)
			}

			a, b, c, err := repo.InternalDownloadFile(ctx, &types.GetFileReq{
				Namespace:   "ns",
				Name:        "n",
				Ref:         "main",
				Path:        "path",
				RepoType:    types.ModelRepo,
				Lfs:         lfs,
				SaveAs:      "zzz",
				CurrentUser: "user",
			})
			require.Nil(t, err)
			if lfs {
				require.Nil(t, a)
				require.Equal(t, int64(0), b)
				require.Equal(t, "foobar", c)
			} else {
				require.Nil(t, a)
				require.Equal(t, int64(100), b)
				require.Equal(t, "", c)
			}
		})
	}

}

func TestRepoComponent_SDKListFiles(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	mockedRepo := &database.Repository{Source: types.HuggingfaceSource, Private: false}
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		mockedRepo, nil,
	)

	files := []*types.File{{Name: "test.go"}}
	repo.mocks.gitServer.EXPECT().GetTree(
		mock.Anything, types.GetTreeRequest{Namespace: "ns", Name: "n", Ref: "main", RepoType: "model", Limit: 500, Recursive: true},
	).Return(&types.GetRepoFileTreeResp{Files: files, Cursor: ""}, nil)

	fs, err := repo.SDKListFiles(ctx, types.ModelRepo, "ns", "n", "main", "user")
	require.Nil(t, err)
	require.Equal(t, &types.SDKFiles{
		Tags:     []string{},
		ID:       "ns/n",
		Siblings: []types.SDKFile{{}},
	}, fs)

}

func TestRepoComponent_IsLFS(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	repo.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "p",
		RepoType:  types.ModelRepo,
	}).Return("readme", nil)

	a, b, err := repo.IsLfs(ctx, &types.GetFileReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "p",
		RepoType:  types.ModelRepo,
	})
	require.Nil(t, err)
	require.Equal(t, false, a)
	require.Equal(t, int64(6), b)

}

func TestRepoComponent_HeadDownloadFile(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	mockedRepo := &database.Repository{Source: types.HuggingfaceSource, Private: false}
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		mockedRepo, nil,
	)

	file := &types.File{Name: "zzz"}
	repo.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "path",
		RepoType:  types.ModelRepo,
	}).Return(file, nil)

	commit := &types.Commit{Message: "zzz"}
	repo.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		RepoType:  types.ModelRepo,
	}).Return(commit, nil)

	f, c, e := repo.HeadDownloadFile(ctx, &types.GetFileReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "path",
		RepoType:  types.ModelRepo,
	}, "user")
	require.Nil(t, e)
	require.Equal(t, file, f)
	require.Equal(t, commit, c)

}

func TestRepoComponent_SDKDownloadFile(t *testing.T) {
	for _, lfs := range []bool{false, true} {
		t.Run(fmt.Sprintf("is lfs: %v", lfs), func(t *testing.T) {
			ctx := context.TODO()
			repo := initializeTestRepoComponent(ctx, t)

			mockedRepo := &database.Repository{}
			repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
				mockedRepo, nil,
			)

			if lfs {

				repo.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       "main",
					Path:      "path",
					RepoType:  types.ModelRepo,
				}).Return(&types.File{LfsRelativePath: "qqq", LfsSHA256: "123456"}, nil)

				reqParams := make(url.Values)
				reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", "zzz"))
				repo.mocks.s3Client.EXPECT().PresignedGetObject(
					ctx, repo.lfsBucket, "lfs/12/34/56", types.OssFileExpire, reqParams,
				).Return(&url.URL{Path: "foobar"}, nil)
			} else {
				repo.mocks.gitServer.EXPECT().GetRepoFileReader(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       "main",
					Path:      "path",
					RepoType:  types.ModelRepo,
				}).Return(nil, 100, nil)
			}

			a, b, c, err := repo.SDKDownloadFile(ctx, &types.GetFileReq{
				Namespace:   "ns",
				Name:        "n",
				Ref:         "main",
				Path:        "path",
				RepoType:    types.ModelRepo,
				Lfs:         lfs,
				SaveAs:      "zzz",
				CurrentUser: "user",
			}, "user")
			require.Nil(t, err)
			if lfs {
				require.Nil(t, a)
				require.Equal(t, int64(0), b)
				require.Equal(t, "foobar", c)
			} else {
				require.Nil(t, a)
				require.Equal(t, int64(100), b)
				require.Equal(t, "", c)
			}
		})
	}

}

func TestRepoComponent_CreateMirror(t *testing.T) {
	cases := []struct {
		saas bool
	}{
		{false},
		{true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			ctx := context.TODO()
			repo := initializeTestRepoComponent(ctx, t)

			repo.config.Saas = c.saas

			mockedRepo := &database.Repository{ID: 123, Path: "ns/n", RepositoryType: types.ModelRepo}
			repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
				mockedRepo, nil,
			)
			mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")

			repo.mocks.stores.MirrorMock().EXPECT().IsExist(ctx, int64(123)).Return(false, nil)
			repo.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(321)).Return(&database.MirrorSource{}, nil)

			mirror := &database.Mirror{
				SourceUrl:      "su",
				MirrorSourceID: 321,
				Username:       "user",
				AccessToken:    "ak",
				RepositoryID:   123,
				Repository:     mockedRepo,
				LocalRepoPath:  "_model_ns_n",
				Priority:       types.ASAPMirrorPriority,
			}

			rmi := *mirror
			rmi.ID = 321
			rmi.Priority = types.HighMirrorPriority

			repo.mocks.stores.MirrorMock().EXPECT().Create(ctx, mirror).Return(&rmi, nil)
			repo.mocks.stores.MirrorMock().EXPECT().Update(ctx, mock.Anything).Return(nil)
			repo.mocks.stores.MirrorTaskMock().EXPECT().Create(ctx, mock.Anything).Return(database.MirrorTask{}, nil)

			rm, err := repo.CreateMirror(ctx, types.CreateMirrorReq{
				SourceUrl:      "su",
				Username:       "user",
				CurrentUser:    "user",
				AccessToken:    "ak",
				Namespace:      "ns",
				Name:           "n",
				RepoType:       types.ModelRepo,
				MirrorSourceID: 321,
			})
			require.Nil(t, err)
			require.Equal(t, rmi, *rm)
		})
	}
}

func TestRepoComponent_GetMirror(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(&database.Repository{
		ID: 123,
	}, nil)
	dm := &database.Mirror{ID: 11, SourceUrl: "test", Repository: &database.Repository{Path: "test/abc", RepositoryType: types.ModelRepo}}
	m := &types.Mirror{ID: 11, SourceUrl: "test", LocalRepoPath: "models/test/abc"}
	repo.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(dm, nil)
	mm, err := repo.GetMirror(ctx, types.GetMirrorReq{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
	})
	require.Nil(t, err)
	require.Equal(t, m, mm)
}

func TestRepoComponent_UpdateMirror(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(&database.Repository{
		ID: 123,
	}, nil)
	m := database.Mirror{
		ID:              123,
		Username:        "user",
		AccessToken:     "ak",
		PushUsername:    "user",
		PushAccessToken: "foo",
		LocalRepoPath:   "a_model_ns_n",
		MirrorSourceID:  111,
	}
	mi := m
	repo.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(&mi, nil)
	repo.mocks.stores.AccessTokenMock().EXPECT().GetUserGitToken(ctx, "user").Return(&database.AccessToken{Token: "foo"}, nil)
	repo.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(111)).Return(&database.MirrorSource{
		SourceName: "a",
	}, nil)
	repo.mocks.stores.MirrorMock().EXPECT().Update(ctx, &m).Return(nil)

	mm, err := repo.UpdateMirror(ctx, types.UpdateMirrorReq{
		Namespace:      "ns",
		CurrentUser:    "user",
		Username:       "user",
		AccessToken:    "ak",
		RepoType:       types.ModelRepo,
		Name:           "n",
		MirrorSourceID: 111,
	})
	require.Nil(t, err)
	require.Equal(t, m, *mm)
}

func TestRepoComponent_DeleteMirror(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(&database.Repository{
		ID: 123,
	}, nil)
	m := &database.Mirror{ID: 123}
	repo.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(m, nil)
	repo.mocks.stores.MirrorMock().EXPECT().Delete(ctx, m).Return(nil)
	err := repo.DeleteMirror(ctx, types.DeleteMirrorReq{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
	})
	require.Nil(t, err)
}

func TestRepoComponent_ListRuntimeFrameworkWithType(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	frames := []database.RuntimeFramework{
		{
			ID: 1, FrameName: "foo", FrameVersion: "v1", FrameImage: "i",
			Enabled: 1, ContainerPort: 321, Type: 12,
		},
	}
	repo.mocks.stores.RuntimeFrameworkMock().EXPECT().List(ctx, 1).Return(frames, nil)

	fs, err := repo.ListRuntimeFrameworkWithType(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(fs))
	require.Equal(t, types.RuntimeFramework{
		ID: 1, FrameName: "foo", FrameVersion: "v1", FrameImage: "i",
		Enabled: 1, ContainerPort: 321, Type: 12,
	}, fs[0])

}

func TestRepoComponent_ListRuntimeFramework(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(&database.Repository{
		ID:   123,
		Name: "test",
		Tags: []database.Tag{
			{
				Name:     "safetensors",
				Category: "framework",
			},
		},
		Metadata: database.Metadata{
			Architecture: "qwen",
		},
	}, nil)

	frames := []database.RuntimeFramework{}
	frames = append(frames, database.RuntimeFramework{
		ID: 1, FrameName: "foo", FrameVersion: "v1",
		FrameImage: "i",
		Enabled:    1, ContainerPort: 321, Type: 12,
		ModelFormat: "safetensors",
	})
	repo.mocks.stores.RuntimeFrameworkMock().EXPECT().ListByArchsNameAndType(ctx, "test", "safetensors", []string{"qwen"}, 1).Return(frames, nil)

	fs, err := repo.ListRuntimeFramework(ctx, types.ModelRepo, "ns", "n", 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(fs))
	require.Equal(t, types.RuntimeFramework{
		ID: 1, FrameName: "foo", FrameVersion: "v1", FrameImage: "i",
		Enabled: 1, ContainerPort: 321, Type: 12,
	}, fs[0])

}

func TestRepoComponent_CreateRuntimeFramework(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	frame := database.RuntimeFramework{
		FrameName:     "fm",
		FrameVersion:  "v1",
		FrameImage:    "img",
		Enabled:       2,
		ContainerPort: 321,
		Type:          2,
	}
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1, RoleMask: "admin"}, nil)
	repo.mocks.stores.RuntimeFrameworkMock().EXPECT().Add(ctx, frame).Return(nil, nil)

	fn, err := repo.CreateRuntimeFramework(ctx, &types.RuntimeFrameworkReq{
		FrameName:     "fm",
		FrameVersion:  "v1",
		FrameImage:    "img",
		Enabled:       2,
		ContainerPort: 321,
		Type:          2,
		CurrentUser:   "user",
	})
	require.Nil(t, err)
	require.Equal(t, types.RuntimeFramework{
		FrameName:     "fm",
		FrameVersion:  "v1",
		FrameImage:    "img",
		Enabled:       2,
		ContainerPort: 321,
		Type:          2,
	}, *fn)

}

func TestRepoComponent_UpdateRuntimeFramework(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	frame := database.RuntimeFramework{
		ID:            123,
		FrameName:     "fm",
		FrameVersion:  "v1",
		FrameImage:    "img",
		Enabled:       2,
		ContainerPort: 321,
		Type:          2,
	}
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1, RoleMask: "admin"}, nil)
	repo.mocks.stores.RuntimeFrameworkMock().EXPECT().Update(ctx, frame).Return(&frame, nil)

	fn, err := repo.UpdateRuntimeFramework(ctx, 123, &types.RuntimeFrameworkReq{
		FrameName:     "fm",
		FrameVersion:  "v1",
		FrameImage:    "img",
		Enabled:       2,
		ContainerPort: 321,
		Type:          2,
		CurrentUser:   "user",
	})
	require.Nil(t, err)
	require.Equal(t, types.RuntimeFramework{
		ID:            123,
		FrameName:     "fm",
		FrameVersion:  "v1",
		FrameImage:    "img",
		Enabled:       2,
		ContainerPort: 321,
		Type:          2,
	}, *fn)

}

func TestRepoComponent_DeleteRuntimeFramework(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	frame := database.RuntimeFramework{
		FrameName:     "fm",
		FrameVersion:  "v1",
		FrameImage:    "img",
		Enabled:       2,
		ContainerPort: 321,
		Type:          2,
	}
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1, RoleMask: "admin"}, nil)
	repo.mocks.stores.RuntimeFrameworkMock().EXPECT().FindByID(ctx, int64(123)).Return(&frame, nil)
	repo.mocks.stores.RuntimeFrameworkMock().EXPECT().Delete(ctx, frame).Return(nil)

	err := repo.DeleteRuntimeFramework(ctx, "user", 123)
	require.Nil(t, err)

}

func TestRepoComponent_ListDeploy(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(&database.Repository{
		ID: 123,
	}, nil)
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 123}, nil)

	deploys := []database.Deploy{
		{ID: 123, DeployName: "foo", RepoID: 123, GitBranch: "main"},
	}
	repo.mocks.stores.DeployTaskMock().EXPECT().ListDeploy(ctx, types.ModelRepo, int64(123), int64(123)).Return(deploys, nil)

	ds, err := repo.ListDeploy(ctx, types.ModelRepo, "ns", "n", "user")
	require.Nil(t, err)
	require.Equal(t, 1, len(ds))
	require.Equal(t, types.DeployRepo{
		DeployID:   123,
		DeployName: "foo",
		RepoID:     123,
		GitBranch:  "main",
		Status:     "Stopped",
	}, ds[0])
}

func TestRepoComponent_DeleteDeploy(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
	dr := types.DeployRepo{
		SpaceID:   0,
		DeployID:  3,
		Namespace: "ns",
		Name:      "n",
		ClusterID: "cluster",
	}
	repo.mocks.deployer.EXPECT().Purge(ctx, dr).Return(nil)
	repo.mocks.deployer.EXPECT().Exist(ctx, dr).Return(false, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(3)).Return(&database.Deploy{
		RepoID:        1,
		UserUUID:      "uuid",
		OrderDetailID: 11,
		ClusterID:     "cluster",
	}, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().DeleteDeploy(
		ctx, types.ModelRepo, int64(1), int64(0), int64(3),
	).Return(nil)
	ur := &database.UserResources{}
	repo.mocks.stores.UserResourcesMock().EXPECT().FindUserResourcesByOrderDetailId(ctx, "uuid", int64(11)).Return(ur, nil)
	repo.mocks.stores.UserResourcesMock().EXPECT().UpdateDeployId(ctx, ur).Return(nil)

	err := repo.DeleteDeploy(ctx, types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "user",
		DeployID:    3,
	})
	require.Nil(t, err)

}

func TestRepoComponent_DeployDetail(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")

	repo.mocks.stores.ClusterInfoMock().EXPECT().ByClusterID(ctx, "cluster").Return(database.ClusterInfo{
		Zone:     "z",
		Provider: "p",
	}, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(&database.Deploy{
		RepoID:        1,
		UserUUID:      "uuid",
		OrderDetailID: 11,
		ClusterID:     "cluster",
		SvcName:       "svc",
		Status:        deployStatus.Running,
	}, nil)

	repo.mocks.deployer.EXPECT().GetReplica(ctx, types.DeployRepo{
		Namespace: "ns",
		Name:      "n",
		ClusterID: "cluster",
		SvcName:   "svc",
	}).Return(1, 2, []types.Instance{{Name: "i1"}}, nil)

	repo.mocks.deployer.EXPECT().Status(ctx, types.DeployRepo{
		DeployID:  0,
		SpaceID:   0,
		ModelID:   0,
		Namespace: "ns",
		Name:      "n",
		SvcName:   "svc",
		ClusterID: "cluster",
	}, false).Return("svc", 23, nil, nil)

	dp, err := repo.DeployDetail(ctx, types.DeployActReq{
		RepoType:     types.ModelRepo,
		Namespace:    "ns",
		Name:         "n",
		CurrentUser:  "user",
		DeployID:     1,
		DeployType:   2,
		InstanceName: "i1",
	})
	require.Nil(t, err)
	require.Equal(t, types.DeployRepo{
		RepoID:         1,
		ActualReplica:  1,
		DesiredReplica: 2,
		Status:         "Running",
		ClusterID:      "cluster",
		Instances:      []types.Instance{{Name: "i1"}},
		Private:        true,
		SvcName:        "svc",
		Endpoint:       "endpoint/svc",
	}, *dp)

}

func TestRepoComponent_DeployInstanceLogs(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(123)).Return(&database.Deploy{
		ID:            123,
		RepoID:        1,
		UserUUID:      "uuid",
		OrderDetailID: 11,
		ClusterID:     "cluster",
		SvcName:       "svc",
		Status:        deployStatus.Running,
	}, nil)

	m := &deploy.MultiLogReader{}
	repo.mocks.deployer.EXPECT().InstanceLogs(ctx, types.DeployRepo{
		DeployID:     123,
		Namespace:    "ns",
		Name:         "n",
		ClusterID:    "cluster",
		SvcName:      "svc",
		InstanceName: "i1",
	}).Return(m, nil)

	mr, err := repo.DeployInstanceLogs(ctx, types.DeployActReq{
		RepoType:     types.ModelRepo,
		Namespace:    "ns",
		Name:         "n",
		CurrentUser:  "user",
		DeployID:     123,
		DeployType:   1,
		InstanceName: "i1",
	})
	require.Nil(t, err)
	require.Equal(t, m, mr)
}

func TestRepoComponent_AllowAccessByRepoID(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	repo.mocks.stores.RepoMock().EXPECT().FindById(ctx, int64(123)).Return(&database.Repository{
		Path:           "foo/bar",
		RepositoryType: types.ModelRepo,
	}, nil)
	mockedRepo := &database.Repository{}
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "foo", "bar").Return(
		mockedRepo, nil,
	)

	allow, err := repo.AllowAccessByRepoID(ctx, 123, "user")
	require.Nil(t, err)
	require.True(t, allow)
}

func TestRepoComponent_AllowAccessEndpoint(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		ID: 123,
	}, nil)

	allow, err := repo.AllowAccessEndpoint(ctx, "user", &database.Deploy{
		SecureLevel: types.EndpointPrivate,
		UserID:      123,
		RepoID:      456,
	})
	require.Nil(t, err)
	require.True(t, allow)
}

func TestRepoComponent_AllowAccessDeploy(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(123)).Return(&database.Deploy{
		UserID: 123,
	}, nil)
	mockedRepo := &database.Repository{}
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		mockedRepo, nil,
	)
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		ID: 123,
	}, nil)

	allow, err := repo.AllowAccessDeploy(ctx, types.DeployActReq{
		RepoType:     types.ModelRepo,
		Namespace:    "ns",
		Name:         "n",
		CurrentUser:  "user",
		DeployID:     123,
		DeployType:   1,
		InstanceName: "i1",
	})
	require.Nil(t, err)
	require.True(t, allow)
}

func TestRepoComponent_DeployStop(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dr := types.DeployRepo{DeployID: 3, Namespace: "ns", Name: "n"}
	repo.mocks.deployer.EXPECT().Stop(ctx, dr).Return(nil)
	repo.mocks.deployer.EXPECT().Exist(ctx, dr).Return(false, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().StopDeploy(
		ctx, types.ModelRepo, int64(0), int64(2), int64(3),
	).Return(nil)
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		ID: 2,
	}, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(3)).Return(&database.Deploy{
		UserID: 2,
	}, nil)

	err := repo.DeployStop(ctx, types.DeployActReq{
		RepoType:     types.ModelRepo,
		Namespace:    "ns",
		Name:         "n",
		CurrentUser:  "user",
		DeployID:     3,
		DeployType:   1,
		InstanceName: "i1",
	})
	require.Nil(t, err)

}

func TestRepoComponent_AllowReadAccessByDeployID(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(123)).Return(&database.Deploy{
		UserID: 123,
	}, nil)
	mockedRepo := &database.Repository{}
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		mockedRepo, nil,
	)
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		ID: 123,
	}, nil)

	allow, err := repo.AllowReadAccessByDeployID(ctx, types.ModelRepo, "ns", "n", "user", 123)
	require.Nil(t, err)
	require.True(t, allow)
}

func TestRepoComponent_DeployStatus(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(123)).Return(&database.Deploy{
		ID:        1,
		SpaceID:   2,
		ModelID:   3,
		SvcName:   "svc",
		ClusterID: "cluster",
	}, nil)
	repo.mocks.deployer.EXPECT().Status(ctx, types.DeployRepo{
		DeployID:  1,
		SpaceID:   2,
		ModelID:   3,
		Namespace: "ns",
		Name:      "n",
		SvcName:   "svc",
		ClusterID: "cluster",
	}, true).Return("svc", 2, []types.Instance{{Name: "i1"}}, nil)
	status, err := repo.DeployStatus(ctx, types.ModelRepo, "ns", "n", 123)
	require.Nil(t, err)
	require.Equal(t, "Stopped", status.Status)
	require.Equal(t, []types.Instance{{Name: "i1"}}, status.Details)

}

func TestRepoComponent_SyncMirror(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")

	mirror := &database.Mirror{
		ID:             321,
		SourceUrl:      "/models/ns/n.git",
		Username:       "user",
		RepositoryID:   123,
		SourceRepoPath: "ns/n",
	}

	repo.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(mirror, nil)
	mockedRepo := &database.Repository{ID: 123}
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		mockedRepo, nil,
	)

	repo.mocks.stores.MirrorMock().EXPECT().Update(ctx, mirror).Return(nil)
	repo.mocks.stores.MirrorTaskMock().EXPECT().CancelOtherTasksAndCreate(ctx, mock.Anything).Return(database.MirrorTask{}, nil)

	err := repo.SyncMirror(ctx, types.ModelRepo, "ns", "n", "user")
	require.Nil(t, err)
}

func TestRepoComponent_Branches(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockedRepo := &database.Repository{Source: types.HuggingfaceSource, Private: false}
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		mockedRepo, nil,
	).Once()
	expected := []types.Branch{{Name: "foo"}}
	req := &types.GetBranchesReq{
		Namespace: "ns",
		Name:      "n",
		Per:       10,
		Page:      1,
		RepoType:  types.ModelRepo,
	}
	greq := gitserver.GetBranchesReq{
		Namespace: "ns",
		Name:      "n",
		Per:       10,
		Page:      1,
		RepoType:  types.ModelRepo,
	}
	repo.mocks.gitServer.EXPECT().GetRepoBranches(ctx, greq).Return(expected, nil).Once()

	bs, err := repo.Branches(ctx, req)
	require.Nil(t, err)
	require.Equal(t, expected, bs)

	// remote repo, err, return empty results
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		mockedRepo, nil,
	).Once()
	repo.mocks.gitServer.EXPECT().GetRepoBranches(ctx, greq).Return(nil, errors.New("err")).Once()
	bs, err = repo.Branches(ctx, req)
	require.Nil(t, err)
	require.Equal(t, []types.Branch{}, bs)

	// local repo, err, return error
	mockedRepo.Source = types.LocalSource
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		mockedRepo, nil,
	).Once()
	repo.mocks.gitServer.EXPECT().GetRepoBranches(ctx, greq).Return(nil, errors.New("err")).Once()
	_, err = repo.Branches(ctx, req)
	require.NotNil(t, err)

}

func TestRepoComponent_Tags(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockedRepo := &database.Repository{Source: types.HuggingfaceSource, Private: false, ID: 123}
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		mockedRepo, nil,
	).Once()
	expected := []database.Tag{{Name: "foo"}}
	req := &types.GetTagsReq{
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.ModelRepo,
	}

	repo.mocks.stores.RepoMock().EXPECT().Tags(ctx, int64(123)).Return(expected, nil)

	ts, err := repo.Tags(ctx, req)
	require.Nil(t, err)
	require.Equal(t, expected, ts)

}

func TestRepoComponent_UpdateTags(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockedRepo := &database.Repository{Source: types.HuggingfaceSource, Private: false, ID: 123}
	repo.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		mockedRepo, nil,
	).Once()
	repo.mocks.components.tag.EXPECT().UpdateRepoTagsByCategory(
		ctx, types.ModelTagScope, int64(123), "cat", []string{"foo", "bar"},
	).Return(nil)
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)

	err := repo.UpdateTags(ctx, "ns", "n", types.ModelRepo, "cat", "user", []string{"foo", "bar"})
	require.Nil(t, err)

}

func TestRepoComponent_GetFilePreviewCode(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestRepoComponent(ctx, t)

	// Test case 1: file content is text
	txtContent := "Hello, World!"
	expected1 := types.FilePreviewCodeNormal
	result1 := c.getFilePreviewCode([]byte(txtContent))
	assert.Equal(t, expected1, result1)

	// Test case 2: file content is not text
	pngFileHeader := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a} // PNG file header
	expected2 := types.FilePreviewCodeNotText
	result2 := c.getFilePreviewCode(pngFileHeader)
	assert.Equal(t, expected2, result2)
}

// test for adjustMaxFileSize
func TestRepoComponent_AdjustMaxFileSize(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestRepoComponent(ctx, t)
	maxFileSize := int64(0)
	expected := int64(100 * 9000)
	result := c.adjustMaxFileSize(maxFileSize)
	assert.Equal(t, expected, result)

	expected = int64(100 * 9000)
	maxFileSize = expected + 1
	result = c.adjustMaxFileSize(maxFileSize)
	assert.Equal(t, expected, result)
}

func TestRepoComponent_checkCurrentUserPermission(t *testing.T) {

	t.Run("can read self-owned", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)

		ns := database.Namespace{}
		ns.NamespaceType = "user"
		ns.Path = "user_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.True(t, yes)
		require.NoError(t, err)
	})

	t.Run("can not read other's", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)

		ns := database.Namespace{}
		ns.NamespaceType = "user"
		ns.Path = "user_name_other"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.False(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.False(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.False(t, yes)
		require.NoError(t, err)
	})

	t.Run("can not read org's if not org member", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)

		ns := database.Namespace{}
		ns.NamespaceType = "organization"
		ns.Path = "org_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		//user not belongs to org
		repoComp.mocks.userSvcClient.EXPECT().GetMemberRole(mock.Anything, ns.Path, user.Username).Return(membership.RoleUnknown, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.False(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.False(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.False(t, yes)
		require.NoError(t, err)
	})

	t.Run("can read org's as org member", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)

		ns := database.Namespace{}
		ns.NamespaceType = "organization"
		ns.Path = "org_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		//user is read-only member of the org
		repoComp.mocks.userSvcClient.EXPECT().GetMemberRole(mock.Anything, ns.Path, user.Username).Return(membership.RoleRead, nil)

		//can read
		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.True(t, yes)
		require.NoError(t, err)
		//can't write
		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.False(t, yes)
		require.NoError(t, err)
		//can't admin
		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.False(t, yes)
		require.NoError(t, err)
	})

	t.Run("admin read org's", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)

		ns := database.Namespace{}
		ns.NamespaceType = "organization"
		ns.Path = "org_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name_admin"
		user.RoleMask = "admin"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.True(t, yes)
		require.NoError(t, err)
	})

	t.Run("admin read other's", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)

		ns := database.Namespace{}
		ns.NamespaceType = "user"
		ns.Path = "user_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name_admin"
		user.RoleMask = "admin"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.True(t, yes)
		require.NoError(t, err)
	})
}

func TestRepoComponent_LastCommit(t *testing.T) {
	t.Run("can read self-owned", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)

		repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&database.Repository{}, nil)

		ns := database.Namespace{}
		ns.NamespaceType = "user"
		ns.Path = "user_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		commit := &types.Commit{}
		repoComp.mocks.gitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(commit, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.True(t, yes)
		require.NoError(t, err)

		actualCommit, err := repoComp.LastCommit(context.Background(), &types.GetCommitsReq{})
		require.NoError(t, err)
		require.Equal(t, commit, actualCommit)

	})

	t.Run("forbidden anonymous user to read private repo", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)

		repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&database.Repository{
			// private repo don't allow read from other user
			Private: true,
		}, nil)

		actualCommit, err := repoComp.LastCommit(context.Background(), &types.GetCommitsReq{})
		require.Nil(t, actualCommit)
		require.ErrorIs(t, err, errorx.ErrForbidden)

	})
}

func TestRepoComponent_Tree(t *testing.T) {
	{
		t.Run("can read self-owned", func(t *testing.T) {
			ctx := context.TODO()
			repoComp := initializeTestRepoComponent(ctx, t)

			user := database.User{}
			user.Username = "user_name"
			repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

			ns := database.Namespace{}
			ns.NamespaceType = "user"
			ns.Path = "user_name"
			repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

			repo := &database.Repository{
				Private: true,
				User:    user,
				Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
				Source:  types.LocalSource,
			}
			repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, types.ModelRepo, ns.Path, repo.Name).Return(repo, nil)

			tree := []*types.File{}
			repoComp.mocks.gitServer.EXPECT().GetRepoFileTree(mock.Anything, mock.Anything).Return(tree, nil)

			actualTree, err := repoComp.Tree(context.Background(), &types.GetFileReq{
				Namespace:   ns.Path,
				Name:        repo.Name,
				Path:        "",
				RepoType:    types.ModelRepo,
				CurrentUser: user.Username,
			})
			require.Nil(t, err)
			require.Equal(t, tree, actualTree)

		})

		t.Run("forbidden anoymous user to read private repo", func(t *testing.T) {
			ctx := context.TODO()
			repoComp := initializeTestRepoComponent(ctx, t)

			repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&database.Repository{
				// private repo don't allow read from other user
				Private: true,
			}, nil)

			actualTree, err := repoComp.Tree(context.Background(), &types.GetFileReq{})
			require.Nil(t, actualTree)
			require.ErrorIs(t, err, errorx.ErrForbidden)
		})
	}

}

func TestRepoComponent_AllowReadAccess(t *testing.T) {
	t.Run("should return false if repo find return error", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)
		repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").Return(&database.Repository{}, errors.New("error"))
		allow, err := repoComp.AllowReadAccess(ctx, types.ModelRepo, "namespace", "name", "user_name")
		require.Error(t, fmt.Errorf("failed to find repo, error: %w", err))
		require.False(t, allow)
	})
}

func TestRepoComponent_AllowWriteAccess(t *testing.T) {
	t.Run("should return false if username is empty", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)
		repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").Return(&database.Repository{
			ID:      1,
			Name:    "name",
			Path:    "namespace/name",
			Private: false,
		}, nil)
		allow, err := repoComp.AllowWriteAccess(ctx, types.ModelRepo, "namespace", "name", "")
		require.Error(t, err, errorx.ErrUserNotFound)
		require.False(t, allow)
	})

	t.Run("should return false if repo find return error", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)
		repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").Return(&database.Repository{}, errors.New("error"))
		allow, err := repoComp.AllowWriteAccess(ctx, types.ModelRepo, "namespace", "name", "user_name")
		require.Error(t, err, fmt.Errorf("failed to find repo, error: %w", err))
		require.False(t, allow)
	})

	t.Run("should return false if user has no write access for public repo", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)
		repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").Return(&database.Repository{
			ID:      1,
			Name:    "name",
			Path:    "namespace/name",
			Private: false,
		}, nil)
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "namespace").Return(database.Namespace{
			ID:            1,
			Path:          "namespace",
			NamespaceType: database.UserNamespace,
		}, nil)
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user_name").Return(database.User{
			ID:       1,
			Username: "user_name",
			Email:    "user@example.com",
			RoleMask: "",
		}, nil)
		allow, err := repoComp.AllowAdminAccess(ctx, types.ModelRepo, "namespace", "name", "user_name")
		require.NoError(t, err)
		require.False(t, allow)
	})
}

func TestRepoComponent_AllowAdminAccess(t *testing.T) {
	t.Run("should return false if username is empty", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)
		repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").Return(&database.Repository{
			ID:      1,
			Name:    "name",
			Path:    "namespace/name",
			Private: false,
		}, nil)
		allow, err := repoComp.AllowAdminAccess(ctx, types.ModelRepo, "namespace", "name", "")
		require.Error(t, err, errorx.ErrUserNotFound)
		require.False(t, allow)
	})

	t.Run("should return false if repo find return error", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)
		repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").Return(&database.Repository{}, errors.New("error"))
		allow, err := repoComp.AllowAdminAccess(ctx, types.ModelRepo, "namespace", "name", "user_name")
		require.Error(t, err, fmt.Errorf("failed to find repo, error: %w", err))
		require.False(t, allow)
	})

	t.Run("should return false if user has no admin access for public repo", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)
		repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").Return(&database.Repository{
			ID:      1,
			Name:    "name",
			Path:    "namespace/name",
			Private: false,
		}, nil)
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "namespace").Return(database.Namespace{
			ID:            1,
			Path:          "namespace",
			NamespaceType: database.UserNamespace,
		}, nil)
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user_name").Return(database.User{
			ID:       1,
			Username: "user_name",
			Email:    "user@example.com",
			RoleMask: "",
		}, nil)
		allow, err := repoComp.AllowAdminAccess(ctx, types.ModelRepo, "namespace", "name", "user_name")
		require.NoError(t, err)
		require.False(t, allow)
	})
}

func TestRepoComponent_AllowReadAccessRepo(t *testing.T) {
	t.Run("should return true if repo is public", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)

		allow, err := repoComp.AllowReadAccessRepo(ctx, &database.Repository{
			ID:      1,
			Name:    "name",
			Path:    "namespace/name",
			Private: false,
		}, "user_name")
		require.NoError(t, err)
		require.True(t, allow)
	})

	t.Run("should return false if repo is private and username is empty", func(t *testing.T) {
		ctx := context.TODO()
		repoComp := initializeTestRepoComponent(ctx, t)

		allow, err := repoComp.AllowReadAccessRepo(ctx, &database.Repository{
			ID:      1,
			Name:    "name",
			Path:    "namespace/name",
			Private: true,
		}, "")
		require.Error(t, err, errorx.ErrUserNotFound)
		require.False(t, allow)
	})
}

func TestRepoComponent_TreeV2(t *testing.T) {
	{
		t.Run("can read self-owned", func(t *testing.T) {
			ctx := context.TODO()
			repoComp := initializeTestRepoComponent(ctx, t)

			user := database.User{}
			user.Username = "user_name"
			repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

			ns := database.Namespace{}
			ns.NamespaceType = "user"
			ns.Path = "user_name"
			repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

			repo := &database.Repository{
				Private: true,
				User:    user,
				Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
				Source:  types.LocalSource,
			}
			repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, types.ModelRepo, ns.Path, repo.Name).Return(repo, nil)

			tree := &types.GetRepoFileTreeResp{}
			req := &types.GetTreeRequest{
				Namespace:   ns.Path,
				Name:        repo.Name,
				Path:        "go",
				RepoType:    types.ModelRepo,
				CurrentUser: user.Username,
				Limit:       100,
				Cursor:      "cc",
			}
			repoComp.mocks.gitServer.EXPECT().GetTree(mock.Anything, *req).Return(tree, nil)

			actualTree, err := repoComp.TreeV2(context.Background(), req)
			require.Nil(t, err)
			require.Equal(t, tree, actualTree)

		})

		t.Run("forbidden anoymous user to read private repo", func(t *testing.T) {
			ctx := context.TODO()
			repoComp := initializeTestRepoComponent(ctx, t)

			repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&database.Repository{
				// private repo don't allow read from other user
				Private: true,
			}, nil)

			actualTree, err := repoComp.TreeV2(context.Background(), &types.GetTreeRequest{})
			require.Nil(t, actualTree)
			require.ErrorIs(t, err, errorx.ErrForbidden)

		})
	}

}

func TestRepoComponent_TreeV2Remote(t *testing.T) {
	ctx := context.TODO()
	repoComp := initializeTestRepoComponent(ctx, t)

	user := database.User{}
	user.Username = "user_name"
	repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

	ns := database.Namespace{}
	ns.NamespaceType = "user"
	ns.Path = "user_name"
	repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

	repo := &database.Repository{
		ID:      1,
		Private: true,
		User:    user,
		Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
		Source:  types.OpenCSGSource,
	}
	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, types.ModelRepo, ns.Path, repo.Name).Return(repo, nil)
	repoComp.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(1)).Return(nil, sql.ErrNoRows)
	repoComp.mocks.stores.FileMock().EXPECT().FindByParentPath(
		ctx, int64(1), "go", &types.OffsetPagination{Limit: 1, Offset: 0},
	).Return([]database.File{{Name: "f1"}}, nil)
	repoComp.mocks.stores.FileMock().EXPECT().FindByParentPath(
		ctx, int64(1), "go", &types.OffsetPagination{Limit: 1, Offset: 1},
	).Return([]database.File{{Name: "f2"}}, nil)
	repoComp.mocks.stores.FileMock().EXPECT().FindByParentPath(
		ctx, int64(1), "go", &types.OffsetPagination{Limit: 1, Offset: 2},
	).Return([]database.File{}, nil)

	req := &types.GetTreeRequest{
		Namespace:   ns.Path,
		Name:        repo.Name,
		Path:        "go",
		RepoType:    types.ModelRepo,
		CurrentUser: user.Username,
		Limit:       1,
	}

	files := []*types.File{}
	for {
		tree, err := repoComp.TreeV2(ctx, req)
		require.Nil(t, err)
		req.Cursor = tree.Cursor
		files = append(files, tree.Files...)
		if tree.Cursor == "" {
			break
		}
	}
	require.Equal(t, []*types.File{{Name: "f1"}, {Name: "f2"}}, files)
}

func TestRepoComponent_LogsTree(t *testing.T) {
	{
		t.Run("can read self-owned", func(t *testing.T) {
			ctx := context.TODO()
			repoComp := initializeTestRepoComponent(ctx, t)

			user := database.User{}
			user.Username = "user_name"
			repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

			ns := database.Namespace{}
			ns.NamespaceType = "user"
			ns.Path = "user_name"
			repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

			repo := &database.Repository{
				Private: true,
				User:    user,
				Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
				Source:  types.LocalSource,
			}
			repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, types.ModelRepo, ns.Path, repo.Name).Return(repo, nil)

			tree := &types.LogsTreeResp{}
			req := &types.GetLogsTreeRequest{
				Namespace:   ns.Path,
				Name:        repo.Name,
				Path:        "go",
				RepoType:    types.ModelRepo,
				CurrentUser: user.Username,
				Limit:       10,
				Offset:      5,
			}
			repoComp.mocks.gitServer.EXPECT().GetLogsTree(mock.Anything, *req).Return(tree, nil)

			actualTree, err := repoComp.LogsTree(context.Background(), req)
			require.Nil(t, err)
			require.Equal(t, tree, actualTree)

		})

		t.Run("forbidden anoymous user to read private repo", func(t *testing.T) {
			ctx := context.TODO()
			repoComp := initializeTestRepoComponent(ctx, t)

			repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&database.Repository{
				// private repo don't allow read from other user
				Private: true,
			}, nil)

			actualTree, err := repoComp.LogsTree(context.Background(), &types.GetLogsTreeRequest{})
			require.Nil(t, actualTree)
			require.ErrorIs(t, err, errorx.ErrForbidden)

		})
	}

}

func TestRepoComponent_LogsTreeRemote(t *testing.T) {
	ctx := context.TODO()
	repoComp := initializeTestRepoComponent(ctx, t)

	user := database.User{}
	user.Username = "user_name"
	repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

	ns := database.Namespace{}
	ns.NamespaceType = "user"
	ns.Path = "user_name"
	repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

	repo := &database.Repository{
		ID:      1,
		Private: true,
		User:    user,
		Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
		Source:  types.OpenCSGSource,
	}
	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, types.ModelRepo, ns.Path, repo.Name).Return(repo, nil)
	repoComp.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(1)).Return(nil, sql.ErrNoRows)
	repoComp.mocks.stores.FileMock().EXPECT().FindByParentPath(
		ctx, int64(1), "go", &types.OffsetPagination{Limit: 1, Offset: 0},
	).Return([]database.File{{LastCommitMessage: "m1"}}, nil)
	repoComp.mocks.stores.FileMock().EXPECT().FindByParentPath(
		ctx, int64(1), "go", &types.OffsetPagination{Limit: 1, Offset: 1},
	).Return([]database.File{{LastCommitMessage: "m2"}}, nil)
	repoComp.mocks.stores.FileMock().EXPECT().FindByParentPath(
		ctx, int64(1), "go", &types.OffsetPagination{Limit: 1, Offset: 2},
	).Return([]database.File{}, nil)

	req := &types.GetLogsTreeRequest{
		Namespace:   ns.Path,
		Name:        repo.Name,
		Path:        "go",
		RepoType:    types.ModelRepo,
		CurrentUser: user.Username,
		Limit:       1,
		Offset:      0,
	}

	commits := []*types.CommitForTree{}
	for {
		tree, err := repoComp.LogsTree(ctx, req)
		require.Nil(t, err)
		commits = append(commits, tree.Commits...)
		if len(tree.Commits) == 0 {
			break
		}
		req.Offset += 1
	}
	require.Equal(t, []*types.CommitForTree{{Message: "m1"}, {Message: "m2"}}, commits)
}

func TestRepoComponent_FixRepoSource(t *testing.T) {
	ctx := context.TODO()
	repoComp := initializeTestRepoComponent(ctx, t)

	user := database.User{}
	user.Username = "user_name"

	ns := database.Namespace{}
	ns.NamespaceType = "user"
	ns.Path = "user_name"

	repo := &database.Repository{
		ID:      1,
		Private: true,
		User:    user,
		Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
		Source:  types.OpenCSGSource,
		Mirror: database.Mirror{
			ID:        1,
			SourceUrl: "https://opencsg.com/abc/def.git",
		},
	}
	repoComp.mocks.stores.RepoMock().EXPECT().FindMirrorReposWithBatch(mock.Anything, 1000, 1).Return([]database.Repository{*repo}, nil)
	repoComp.mocks.stores.RepoMock().EXPECT().FindMirrorReposWithBatch(mock.Anything, 1000, 2).Return([]database.Repository{}, nil)
	repoComp.mocks.stores.RepoMock().EXPECT().BulkUpdateSourcePath(mock.Anything, mock.Anything).Return(nil)

	err := repoComp.FixRepoSource(ctx)
	require.Nil(t, err)
}

func TestRepoComponent_RemoteTree(t *testing.T) {
	ctx := context.TODO()
	repoComp := initializeTestRepoComponent(ctx, t)

	user := database.User{}
	user.Username = "user_name"
	repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

	ns := database.Namespace{}
	ns.NamespaceType = "user"
	ns.Path = "user_name"
	repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

	repo := &database.Repository{
		ID:      1,
		Private: true,
		User:    user,
		Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
		Source:  types.OpenCSGSource,
	}
	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, types.ModelRepo, ns.Path, repo.Name).Return(repo, nil)
	repoComp.mocks.stores.FileMock().EXPECT().FindByParentPath(
		ctx, int64(1), "go", &types.OffsetPagination{Limit: 1, Offset: 0},
	).Return([]database.File{{Name: "f1"}}, nil)
	repoComp.mocks.stores.FileMock().EXPECT().FindByParentPath(
		ctx, int64(1), "go", &types.OffsetPagination{Limit: 1, Offset: 1},
	).Return([]database.File{{Name: "f2"}}, nil)
	repoComp.mocks.stores.FileMock().EXPECT().FindByParentPath(
		ctx, int64(1), "go", &types.OffsetPagination{Limit: 1, Offset: 2},
	).Return([]database.File{}, nil)

	req := &types.GetTreeRequest{
		Namespace:   ns.Path,
		Name:        repo.Name,
		Path:        "go",
		RepoType:    types.ModelRepo,
		CurrentUser: user.Username,
		Limit:       1,
	}

	files := []*types.File{}
	for {
		tree, err := repoComp.RemoteTree(ctx, req)
		require.Nil(t, err)
		req.Cursor = tree.Cursor
		files = append(files, tree.Files...)
		if tree.Cursor == "" {
			break
		}
	}
	require.Equal(t, []*types.File{{Name: "f1"}, {Name: "f2"}}, files)
}

func TestRepoComponent_DiffBetweenTwoCommits(t *testing.T) {
	ctx := context.TODO()
	repoComp := initializeTestRepoComponent(ctx, t)

	user := database.User{}
	user.Username = "user_name"
	repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

	ns := database.Namespace{}
	ns.NamespaceType = "user"
	ns.Path = "user_name"
	repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

	repo := &database.Repository{
		ID:      1,
		Private: true,
		User:    user,
		Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
		Source:  types.OpenCSGSource,
	}
	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, types.ModelRepo, ns.Path, repo.Name).Return(repo, nil)
	commit := &types.Commit{ID: "zzz"}
	repoComp.mocks.gitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(commit, nil)
	repoComp.mocks.gitServer.EXPECT().GetDiffBetweenTwoCommits(mock.Anything, mock.Anything).Return(&types.GiteaCallbackPushReq{
		Commits: []types.GiteaCallbackPushReq_Commit{
			{
				Added:    []string{"go"},
				Modified: []string{"add"},
				Removed:  []string{"abc"},
			},
		},
	}, nil)

	diff, err := repoComp.DiffBetweenTwoCommits(ctx, types.GetDiffBetweenCommitsReq{
		Namespace:    ns.Path,
		Name:         repo.Name,
		RepoType:     types.ModelRepo,
		LeftCommitID: "aaa",
		CurrentUser:  user.Username,
	})
	require.Equal(t, nil, err)
	require.Equal(t, []types.GiteaCallbackPushReq_Commit{
		{
			Added:    []string{"go"},
			Modified: []string{"add"},
			Removed:  []string{"abc"},
		},
	}, diff)
}

func TestRepoComponent_Preupload(t *testing.T) {
	ctx := context.TODO()
	repoComp := initializeTestRepoComponent(ctx, t)
	repoComp.config.Git.MaxUnLfsFileSize = 100000

	user := database.User{}
	user.Username = "user_name"
	repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

	ns := database.Namespace{}
	ns.NamespaceType = "user"
	ns.Path = "user_name"
	repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

	repo := &database.Repository{
		ID:      1,
		Private: true,
		User:    user,
		Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
		Source:  types.OpenCSGSource,
	}

	req := types.PreuploadReq{
		Namespace:   ns.Path,
		Name:        repo.Name,
		RepoType:    types.ModelRepo,
		Revision:    "revision",
		CurrentUser: user.Username,
		Files: []types.PreuploadFile{
			{
				Path:   "a.go",
				Sample: "",
				Size:   123,
			},
			{
				Path:   "b.example",
				Sample: "",
				Size:   1234,
			},
			{
				Path:   "c.parquet",
				Sample: "",
				Size:   123,
			},
			{
				Path:   "c.txt",
				Sample: "",
				Size:   10000000000,
			},
			{
				Path:   "dir",
				Sample: "",
				Size:   100,
			},
		},
	}

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, types.ModelRepo, ns.Path, repo.Name).Return(repo, nil)
	repoComp.mocks.gitServer.EXPECT().GetTree(mock.Anything, mock.Anything).Return(&types.GetRepoFileTreeResp{
		Files: []*types.File{
			{
				Path: "a.go",
				SHA:  "sha",
			},
			{
				Path: "dir/a.go",
				SHA:  "sha",
			},
		},
		Cursor: "",
	}, nil)

	content := `
*.parquet filter=lfs diff=lfs merge=lfs -text
`
	encodedContent := base64.StdEncoding.EncodeToString([]byte(content))
	repoComp.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		RepoType:  req.RepoType,
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Revision,
		Path:      GitAttributesFileName,
	}).Return(&types.File{
		Content: encodedContent,
	}, nil)

	ignoreContent := `*.example`

	encodedIgnoreContent := base64.StdEncoding.EncodeToString([]byte(ignoreContent))
	repoComp.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		RepoType:  req.RepoType,
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Revision,
		Path:      GitIgnoreFileName,
	}).Return(&types.File{
		Content: encodedIgnoreContent,
	}, nil)

	res, err := repoComp.Preupload(ctx, req)
	require.Equal(t, nil, err)
	require.Equal(t, &types.PreuploadResp{
		Files: []types.PreuploadRespFile{
			{
				OID:          "sha",
				Path:         "a.go",
				ShouldIgnore: false,
				UploadMode:   types.UploadModeRegular,
			},
			{
				OID:          "",
				Path:         "b.example",
				ShouldIgnore: true,
				UploadMode:   types.UploadModeRegular,
			},
			{
				OID:          "",
				Path:         "c.parquet",
				ShouldIgnore: false,
				UploadMode:   types.UploadModeLFS,
			},
			{
				OID:          "",
				Path:         "c.txt",
				ShouldIgnore: false,
				UploadMode:   types.UploadModeLFS,
			},
			{
				OID:          "",
				Path:         "dir",
				ShouldIgnore: false,
				UploadMode:   types.UploadModeRegular,
				IsDir:        true,
			},
		},
	}, res)
}

func TestRepoComponent_CommitFiles(t *testing.T) {
	ctx := context.TODO()
	repoComp := initializeTestRepoComponent(ctx, t)

	user := database.User{}
	user.Username = "user_name"
	repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

	ns := database.Namespace{}
	ns.NamespaceType = "user"
	ns.Path = "user_name"
	repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

	repo := &database.Repository{
		ID:      1,
		Private: true,
		User:    user,
		Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
		Source:  types.OpenCSGSource,
	}

	req := types.CommitFilesReq{
		Namespace:   ns.Path,
		Name:        repo.Name,
		RepoType:    types.ModelRepo,
		Revision:    "main",
		CurrentUser: user.Username,
		Message:     "msg",
		Files: []types.CommitFileReq{
			{
				Path:    "a.go",
				Action:  types.CommitActionUpdate,
				Content: "content",
			},
		},
	}
	repoComp.mocks.gitServer.EXPECT().GetRepoAllFiles(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: ns.Path,
		Name:      repo.Name,
		Ref:       "main",
		RepoType:  types.ModelRepo,
	}).Return([]*types.File{
		{
			Path: "a.go",
			Size: 1024 * 1024 * 1024 * 1024,
		},
	}, nil)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, types.ModelRepo, ns.Path, repo.Name).Return(repo, nil)
	repoComp.mocks.gitServer.EXPECT().CommitFiles(mock.Anything, gitserver.CommitFilesReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		RepoType:  req.RepoType,
		Revision:  req.Revision,
		Username:  user.Username,
		Email:     user.Email,
		Message:   req.Message,
		Files: []gitserver.CommitFile{
			{
				Path:    "a.go",
				Content: "content=",
				Action:  gitserver.CommitActionUpdate,
			},
		},
	}).Return(nil)

	err := repoComp.CommitFiles(ctx, req)
	require.Equal(t, nil, err)
}

func TestRepoComponent_IsExists(t *testing.T) {
	ctx := context.Background()

	repoComp := initializeTestRepoComponent(ctx, t)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").
		Return(&database.Repository{ID: 1}, nil).Once()

	exists, err := repoComp.IsExists(ctx, types.ModelRepo, "namespace", "name")
	require.NoError(t, err)
	require.True(t, exists)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").
		Return(nil, errors.New("error")).Once()

	exists, err = repoComp.IsExists(ctx, types.ModelRepo, "namespace", "name")
	require.Error(t, err)
	require.False(t, exists)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").
		Return(nil, sql.ErrNoRows).Once()

	exists, err = repoComp.IsExists(ctx, types.ModelRepo, "namespace", "name")
	require.NotNil(t, err)
	require.False(t, exists)
}

func TestParseNDJson_LFSFileGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Git.MaxUnLfsFileSize = 10485760

	component := &repoComponentImpl{
		config: cfg,
	}

	// Test LFS file with specific values to verify the generated content
	requestBody := `{"key": "header", "value": {"summary": "LFS test"}}
{"key": "lfsFile", "value": {"path": "test.bin", "algo": "sha256", "oid": "abcdef123456", "size": 1024}}`

	req := httptest.NewRequest("POST", "/test", strings.NewReader(requestBody))
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	result, err := component.ParseNDJson(ctx)

	require.NoError(t, err)
	assert.Equal(t, "LFS test", result.Message)
	assert.Len(t, result.Files, 1)
	assert.Equal(t, "test.bin", result.Files[0].Path)
	assert.Equal(t, types.CommitActionCreate, result.Files[0].Action)

	// Verify the LFS pointer content is properly base64 encoded
	expectedBase64 := "dmVyc2lvbiBodHRwczovL2dpdC1sZnMuZ2l0aHViLmNvbS9zcGVjL3YxCm9pZCBzaGEyNTY6YWJjZGVmMTIzNDU2CnNpemUgMTAyNAo="
	assert.Equal(t, expectedBase64, result.Files[0].Content)
}

func TestParseNDJson_AllKeyTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Git.MaxUnLfsFileSize = 10485760

	component := &repoComponentImpl{
		config: cfg,
	}

	// Test with all supported key types in one request
	requestBody := `{"key": "header", "value": {"summary": "Complete test", "description": "Testing all key types"}}
{"key": "file", "value": {"path": "regular.txt", "content": "cmVndWxhciBmaWxl"}}
{"key": "lfsFile", "value": {"path": "large.bin", "algo": "sha256", "oid": "123abc", "size": 5000}}
{"key": "deletedFile", "value": {"path": "remove.txt"}}
{"key": "deletedFolder", "value": {"path": "old_dir/"}}`

	req := httptest.NewRequest("POST", "/test", strings.NewReader(requestBody))
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	result, err := component.ParseNDJson(ctx)

	require.NoError(t, err)
	assert.Equal(t, "Complete test", result.Message)
	assert.Len(t, result.Files, 4)

	// Check regular file
	assert.Equal(t, "regular.txt", result.Files[0].Path)
	assert.Equal(t, "cmVndWxhciBmaWxl", result.Files[0].Content)
	assert.Equal(t, types.CommitActionCreate, result.Files[0].Action)

	// Check LFS file
	assert.Equal(t, "large.bin", result.Files[1].Path)
	assert.Equal(t, types.CommitActionCreate, result.Files[1].Action)

	// Check deleted file
	assert.Equal(t, "remove.txt", result.Files[2].Path)
	assert.Equal(t, "", result.Files[2].Content)
	assert.Equal(t, types.CommitActionDelete, result.Files[2].Action)

	// Check deleted folder
	assert.Equal(t, "old_dir/", result.Files[3].Path)
	assert.Equal(t, "", result.Files[3].Content)
	assert.Equal(t, types.CommitActionDelete, result.Files[3].Action)
}

func TestGetRepoUrl(t *testing.T) {
	tests := []struct {
		name     string
		repoType types.RepositoryType
		repoPath string
		expected string
	}{
		{
			name:     "Model repository",
			repoType: types.ModelRepo,
			repoPath: "namespace/model",
			expected: "/models/namespace/model",
		},
		{
			name:     "Dataset repository",
			repoType: types.DatasetRepo,
			repoPath: "namespace/dataset",
			expected: "/datasets/namespace/dataset",
		},
		{
			name:     "Space repository",
			repoType: types.SpaceRepo,
			repoPath: "namespace/space",
			expected: "/spaces/namespace/space",
		},
		{
			name:     "Code repository",
			repoType: types.CodeRepo,
			repoPath: "team/code",
			expected: "/codes/team/code",
		},
		{
			name:     "Prompt repository",
			repoType: types.PromptRepo,
			repoPath: "namespace/prompt",
			expected: "/prompts/library/namespace/prompt",
		},
		{
			name:     "MCP Server repository",
			repoType: types.MCPServerRepo,
			repoPath: "namespace/mcpserver",
			expected: "/mcp/servers/namespace/mcpserver",
		},
		{
			name:     "Unknown repository type",
			repoType: types.UnknownRepo,
			repoPath: "namespace/repo",
			expected: "",
		},
		{
			name:     "Empty repo path",
			repoType: types.ModelRepo,
			repoPath: "",
			expected: "",
		},
		{
			name:     "Empty repo type string",
			repoType: "",
			repoPath: "user/repo",
			expected: "",
		},
		{
			name:     "Invalid repo type",
			repoType: "invalid",
			repoPath: "user/repo",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRepoUrl(tt.repoType, tt.repoPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRepoComponent_IsSyncing(t *testing.T) {
	ctx := context.Background()

	repoComp := initializeTestRepoComponent(ctx, t)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").
		Return(&database.Repository{ID: 1}, nil)

	repoComp.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(1)).Return(
		&database.Mirror{
			ID: 1,
			CurrentTask: &database.MirrorTask{
				ID:     1,
				Status: types.MirrorRepoSyncStart,
			},
		}, nil,
	).Once()

	syncing, err := repoComp.IsSyncing(ctx, types.ModelRepo, "namespace", "name")

	require.Nil(t, err)
	assert.True(t, syncing)

	repoComp.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(1)).Return(nil, sql.ErrNoRows).Once()

	syncing, err = repoComp.IsSyncing(ctx, types.ModelRepo, "namespace", "name")

	require.Nil(t, err)
	assert.False(t, syncing)
}

func TestRepoComponent_ChangePath(t *testing.T) {
	ctx := context.Background()

	repoComp := initializeTestRepoComponent(ctx, t)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").
		Return(&database.Repository{ID: 1}, nil)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "new", "path").
		Return(nil, sql.ErrNoRows)

	// repoComp.mocks.gitServer.EXPECT().CopyRepository(
	// 	ctx,
	// 	gitserver.CopyRepositoryReq{
	// 		RepoType:  types.ModelRepo,
	// 		Namespace: "namespace",
	// 		Name:      "name",
	// 		NewPath:   "@hashed_repos/6b/86/6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b.git",
	// 	}).Return(nil)

	err := repoComp.ChangePath(ctx, types.ChangePathReq{
		RepoType:  types.ModelRepo,
		Namespace: "namespace",
		Name:      "name",
		NewPath:   "new/path",
	})

	require.NotNil(t, err)
}

func TestRepoComponent_ChangePath_RepoHashed(t *testing.T) {
	ctx := context.Background()

	repoComp := initializeTestRepoComponent(ctx, t)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").
		Return(&database.Repository{ID: 1, Hashed: true}, nil)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "new", "path").
		Return(nil, sql.ErrNoRows)

	repoComp.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, database.Repository{
		ID:      1,
		Path:    "new/path",
		GitPath: "models_new/path",
		Hashed:  true,
	}).Return(nil, nil)

	err := repoComp.ChangePath(ctx, types.ChangePathReq{
		RepoType:  types.ModelRepo,
		Namespace: "namespace",
		Name:      "name",
		NewPath:   "new/path",
	})

	require.Nil(t, err)
}

func TestRepoComponent_ChangePath_NewNamespaceExists(t *testing.T) {
	ctx := context.Background()

	repoComp := initializeTestRepoComponent(ctx, t)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "name").
		Return(&database.Repository{ID: 1}, nil)

	repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "new", "path").
		Return(&database.Repository{ID: 2}, nil)

	err := repoComp.ChangePath(ctx, types.ChangePathReq{
		RepoType:  types.ModelRepo,
		Namespace: "namespace",
		Name:      "name",
		NewPath:   "new/path",
	})

	require.NotNil(t, err)
}

func TestRepoComponent_BatchMigrateRepoToHashedPath_AutoFalse(t *testing.T) {
	ctx := context.Background()

	repoComp := initializeTestRepoComponent(ctx, t)
	repoComp.config.Git.RepoDataMigrateEnable = true

	repoComp.mocks.stores.RepoMock().EXPECT().FindUnhashedRepos(ctx, 10, int64(0)).
		Return([]database.Repository{{
			ID:             1,
			Hashed:         false,
			Path:           "namespace/name",
			RepositoryType: types.ModelRepo,
		}}, nil)
	repoComp.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, database.Repository{
		ID:             1,
		Path:           "namespace/name",
		RepositoryType: types.ModelRepo,
		Hashed:         true,
	}).Return(&database.Repository{}, nil)

	repoComp.mocks.gitServer.EXPECT().CopyRepository(ctx, gitserver.CopyRepositoryReq{
		Namespace: "namespace",
		Name:      "name",
		RepoType:  types.ModelRepo,
		NewPath:   "@hashed_repos/6b/86/6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b.git",
	}).Return(nil)

	lastID, err := repoComp.BatchMigrateRepoToHashedPath(ctx, false, 10, 0)

	require.Nil(t, err)
	require.Equal(t, int64(1), lastID)
}

func TestRepoComponent_BatchMigrateRepoToHashedPath_AutoTrue(t *testing.T) {
	ctx := context.Background()

	repoComp := initializeTestRepoComponent(ctx, t)
	repoComp.config.Git.RepoDataMigrateEnable = true

	repoComp.mocks.stores.RepoMock().EXPECT().FindUnhashedRepos(ctx, 1, int64(0)).
		Return([]database.Repository{}, nil).Once()

	_, err := repoComp.BatchMigrateRepoToHashedPath(ctx, true, 1, 0)

	require.Nil(t, err)
}

func TestRepoComponent_GetMirrorTaskStatusAndSyncStatus(t *testing.T) {
	ctx := context.Background()

	repoComp := initializeTestRepoComponent(ctx, t)

	repo := &database.Repository{
		ID: 1,
		Mirror: database.Mirror{
			ID: 1,
			CurrentTask: &database.MirrorTask{
				ID:     1,
				Status: types.MirrorRepoSyncStart,
			},
		},
	}

	mirrorTaskStatus, syncStatus := repoComp.GetMirrorTaskStatusAndSyncStatus(repo)

	assert.Equal(t, types.MirrorRepoSyncStart, mirrorTaskStatus)
	assert.Equal(t, types.SyncStatusInProgress, syncStatus)

	repo1 := &database.Repository{
		ID: 1,
		Mirror: database.Mirror{
			ID: 0,
		},
		SyncStatus: types.SyncStatusInProgress,
	}

	mirrorTaskStatus, syncStatus = repoComp.GetMirrorTaskStatusAndSyncStatus(repo1)

	assert.Equal(t, types.MirrorTaskStatus(""), mirrorTaskStatus)
	assert.Equal(t, types.SyncStatusInProgress, syncStatus)

	repo2 := &database.Repository{
		ID: 1,
		Mirror: database.Mirror{
			ID:     1,
			Status: types.MirrorLfsSyncFinished,
		},
		SyncStatus: types.SyncStatusInProgress,
	}

	mirrorTaskStatus, syncStatus = repoComp.GetMirrorTaskStatusAndSyncStatus(repo2)

	assert.Equal(t, types.MirrorTaskStatus(""), mirrorTaskStatus)
	assert.Equal(t, types.SyncStatusCompleted, syncStatus)
}

func TestRepoComponent_UpdateRepo_PermissionChecks(t *testing.T) {
	ctx := context.Background()
	nickname := "new-nickname"
	description := "new-description"

	// Test cases
	testCases := []struct {
		name             string
		setupMocks       func(repo *testRepoWithMocks, req types.UpdateRepoReq)
		req              types.UpdateRepoReq
		expectError      bool
		expectedErrorMsg string
	}{
		{
			name: "Non-admin fails to change privacy of org repo",
			req: types.UpdateRepoReq{
				Username:  "test-user",
				Namespace: "org-ns",
				Name:      "test-repo",
				RepoType:  types.ModelRepo,
				Private:   tea.Bool(true),
			},
			setupMocks: func(repo *testRepoWithMocks, req types.UpdateRepoReq) {
				repo.mocks.stores.RepoMock().EXPECT().Find(ctx, req.Namespace, string(req.RepoType), req.Name).Return(&database.Repository{}, nil)
				repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, req.Namespace).Return(database.Namespace{NamespaceType: database.OrgNamespace}, nil)
				repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{Username: "test-user"}, nil)
				repo.mocks.userSvcClient.EXPECT().GetMemberRole(ctx, req.Namespace, req.Username).Return(membership.RoleWrite, nil)
			},
			expectError:      true,
			expectedErrorMsg: "only admins can change the privacy of an organization repository",
		},
		{
			name: "Non-admin with write access updates org repo successfully without changing privacy",
			req: types.UpdateRepoReq{
				Username:    "test-user",
				Namespace:   "org-ns",
				Name:        "test-repo",
				RepoType:    types.ModelRepo,
				Nickname:    &nickname,
				Description: &description,
			},
			setupMocks: func(repo *testRepoWithMocks, req types.UpdateRepoReq) {
				repo.mocks.stores.RepoMock().EXPECT().Find(ctx, req.Namespace, string(req.RepoType), req.Name).Return(&database.Repository{}, nil)
				repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, req.Namespace).Return(database.Namespace{NamespaceType: database.OrgNamespace}, nil)
				repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{Username: "test-user"}, nil)
				repo.mocks.userSvcClient.EXPECT().GetMemberRole(ctx, req.Namespace, req.Username).Return(membership.RoleWrite, nil)
				repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(&gitserver.CreateRepoResp{}, nil)
				repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.Anything).Return(&database.Repository{}, nil)
			},
			expectError: false,
		},
		{
			name: "Non-admin fails to update org repo without write access",
			req: types.UpdateRepoReq{
				Username:  "test-user",
				Namespace: "org-ns",
				Name:      "test-repo",
				RepoType:  types.ModelRepo,
			},
			setupMocks: func(repo *testRepoWithMocks, req types.UpdateRepoReq) {
				repo.mocks.stores.RepoMock().EXPECT().Find(ctx, req.Namespace, string(req.RepoType), req.Name).Return(&database.Repository{}, nil)
				repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, req.Namespace).Return(database.Namespace{NamespaceType: database.OrgNamespace}, nil)
				repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{Username: "test-user"}, nil)
				repo.mocks.userSvcClient.EXPECT().GetMemberRole(ctx, req.Namespace, req.Username).Return(membership.RoleRead, nil)
			},
			expectError:      true,
			expectedErrorMsg: "users do not have permission to update repo in this organization",
		},
		{
			name: "Non-admin fails to update another user's repo",
			req: types.UpdateRepoReq{
				Username:  "test-user",
				Namespace: "another-user",
				Name:      "test-repo",
				RepoType:  types.ModelRepo,
			},
			setupMocks: func(repo *testRepoWithMocks, req types.UpdateRepoReq) {
				repo.mocks.stores.RepoMock().EXPECT().Find(ctx, req.Namespace, string(req.RepoType), req.Name).Return(&database.Repository{}, nil)
				repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, req.Namespace).Return(database.Namespace{Path: "another-user", NamespaceType: database.UserNamespace}, nil)
				repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{Username: "test-user"}, nil)
			},
			expectError:      true,
			expectedErrorMsg: "users do not have permission to update repo in this namespace",
		},
		{
			name: "Non-admin updates their own repo to public successfully",
			req: types.UpdateRepoReq{
				Username:  "test-user",
				Namespace: "test-user",
				Name:      "test-repo",
				RepoType:  types.ModelRepo,
				Private:   tea.Bool(false),
			},
			setupMocks: func(repo *testRepoWithMocks, req types.UpdateRepoReq) {
				repo.mocks.stores.RepoMock().EXPECT().Find(ctx, req.Namespace, string(req.RepoType), req.Name).Return(&database.Repository{}, nil)
				repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, req.Namespace).Return(database.Namespace{Path: "test-user", NamespaceType: database.UserNamespace}, nil)
				repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{Username: "test-user"}, nil)
				// Mock allowPublic to return true
				// As allowPublic is a private method, we can't mock it directly.
				// We assume it returns true for this test case.
				repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(&gitserver.CreateRepoResp{}, nil)
				repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.Anything).Return(&database.Repository{}, nil)
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := initializeTestRepoComponent(ctx, t)
			tc.setupMocks(repo, tc.req)

			_, err := repo.UpdateRepo(ctx, tc.req)

			if tc.expectError {
				require.Error(t, err)
				if tc.expectedErrorMsg != "" {
					require.Contains(t, err.Error(), tc.expectedErrorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRepoComponent_SendAssetManagementMsg(t *testing.T) {
	config := &config.Config{}
	config.Notification.NotificationRetryCount = 3
	mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)
	repoComp := &repoComponentImpl{
		config:                config,
		notificationSvcClient: mockNotificationRpc,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	mockNotificationRpc.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
		defer wg.Done()
		return req.Scenario == types.MessageScenarioAssetManagement
	})).Return(nil).Once()

	err := repoComp.SendAssetManagementMsg(context.Background(), types.RepoNotificationReq{
		RepoType:  types.ModelRepo,
		RepoPath:  "ns/n",
		Operation: types.OperationCreate,
		UserUUID:  "user0",
	})
	require.Nil(t, err)
	wg.Wait()
}

func TestRepoComponent_GetRepos(t *testing.T) {
	ctx := context.Background()

	repoComp := initializeTestRepoComponent(ctx, t)

	repoComp.mocks.stores.RepoMock().EXPECT().GetReposBySearch(ctx, "search", types.ModelRepo, 1, 10).
		Return([]*database.Repository{{ID: 1, Path: "ns/name"}}, 1, nil).Once()
	paths, err := repoComp.GetRepos(ctx, "search", "u", types.ModelRepo)
	require.NoError(t, err)
	require.Equal(t, 1, len(paths))
	require.Equal(t, "ns/name", paths[0])
}
