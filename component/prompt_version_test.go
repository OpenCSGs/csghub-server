package component

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type fakePromptVersionStore struct {
	versions []database.PromptVersion
	nextID   int64
}

func (s *fakePromptVersionStore) Create(_ context.Context, input database.PromptVersion) (*database.PromptVersion, error) {
	for i := range s.versions {
		if s.versions[i].PromptID == input.PromptID && s.versions[i].FilePath == input.FilePath && s.versions[i].Version == input.Version {
			return nil, errorx.ErrDatabaseDuplicateKey
		}
	}
	s.nextID++
	input.ID = s.nextID
	s.versions = append(s.versions, input)
	return &input, nil
}

func (s *fakePromptVersionStore) ByPromptIDAndFilePath(_ context.Context, promptID int64, filePath string) ([]database.PromptVersion, error) {
	var result []database.PromptVersion
	for i := range s.versions {
		if s.versions[i].PromptID == promptID && s.versions[i].FilePath == filePath {
			result = append(result, s.versions[i])
		}
	}
	return result, nil
}

func (s *fakePromptVersionStore) ByPromptIDFilePathAndVersion(_ context.Context, promptID int64, filePath, version string) (*database.PromptVersion, error) {
	for i := range s.versions {
		if s.versions[i].PromptID == promptID && s.versions[i].FilePath == filePath && s.versions[i].Version == version {
			result := s.versions[i]
			return &result, nil
		}
	}
	return nil, errorx.ErrDatabaseNoRows
}

func (s *fakePromptVersionStore) UpdateHash(_ context.Context, id int64, hash string) (*database.PromptVersion, error) {
	for i := range s.versions {
		if s.versions[i].ID == id {
			s.versions[i].Hash = hash
			result := s.versions[i]
			return &result, nil
		}
	}
	return nil, errorx.ErrDatabaseNoRows
}

func TestNormalizePromptVersionFilePath(t *testing.T) {
	t.Parallel()

	valid, err := normalizePromptVersionFilePath("/folder/prompt.jsonl")
	require.NoError(t, err)
	require.Equal(t, "folder/prompt.jsonl", valid)

	for _, input := range []string{"", "../prompt.jsonl", "/../prompt.jsonl", "prompt.json"} {
		_, err := normalizePromptVersionFilePath(input)
		require.Error(t, err, input)
	}
}

func TestPromptComponent_CreatePromptVersion(t *testing.T) {
	ctx := context.Background()
	pc := initializeTestPromptComponent(ctx, t)
	versionStore := &fakePromptVersionStore{}
	pc.promptVersionStore = versionStore
	repo := &database.Repository{DefaultBranch: "develop"}
	prompt := &database.Prompt{ID: 7, Repository: repo}

	pc.mocks.stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "repo").Return(prompt, nil).Once()
	pc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "writer", repo).Return(&types.UserRepoPermission{CanWrite: true}, nil).Once()
	pc.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: "ns", Name: "repo", Ref: "develop", RepoType: types.PromptRepo,
	}).Return(&types.Commit{ID: "commit-1"}, nil).Once()
	pc.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns", Name: "repo", Ref: "commit-1", Path: "folder/prompt.jsonl", RepoType: types.PromptRepo,
	}).Return(&types.File{Content: "eyJ0aXRsZSI6InQiLCJjb250ZW50IjoiYyIsImxhbmd1YWdlIjoiemgifQ=="}, nil).Once()

	result, err := pc.CreatePromptVersion(ctx, types.PromptVersionReq{
		Namespace: "ns", Name: "repo", CurrentUser: "writer", FilePath: "/folder/prompt.jsonl",
	}, &types.CreatePromptVersionReq{Version: " v1 ", Changelog: "initial"})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.ID)
	require.Equal(t, "v1", result.Version)
	require.Equal(t, "commit-1", result.Commit)
	require.Equal(t, "folder/prompt.jsonl", result.FilePath)
}

func TestPromptComponent_CreatePromptVersionEmptyVersion(t *testing.T) {
	pc := initializeTestPromptComponent(context.Background(), t)
	pc.promptVersionStore = &fakePromptVersionStore{}

	_, err := pc.CreatePromptVersion(context.Background(), types.PromptVersionReq{FilePath: "prompt.jsonl"}, &types.CreatePromptVersionReq{Version: "  "})
	require.ErrorIs(t, err, errorx.ErrReqParamInvalid)
}

func TestPromptComponent_CreatePromptVersionDuplicate(t *testing.T) {
	ctx := context.Background()
	pc := initializeTestPromptComponent(ctx, t)
	pc.promptVersionStore = &fakePromptVersionStore{versions: []database.PromptVersion{{
		PromptID: 7, FilePath: "prompt.jsonl", Version: "v1", Hash: "old-commit",
	}}}
	repo := &database.Repository{DefaultBranch: "develop"}
	prompt := &database.Prompt{ID: 7, Repository: repo}

	pc.mocks.stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "repo").Return(prompt, nil).Once()
	pc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "writer", repo).Return(&types.UserRepoPermission{CanWrite: true}, nil).Once()
	pc.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: "ns", Name: "repo", Ref: "develop", RepoType: types.PromptRepo,
	}).Return(&types.Commit{ID: "commit-1"}, nil).Once()
	pc.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns", Name: "repo", Ref: "commit-1", Path: "prompt.jsonl", RepoType: types.PromptRepo,
	}).Return(&types.File{Content: "eyJ0aXRsZSI6InQiLCJjb250ZW50IjoiYyIsImxhbmd1YWdlIjoiemgifQ=="}, nil).Once()

	_, err := pc.CreatePromptVersion(ctx, types.PromptVersionReq{
		Namespace: "ns", Name: "repo", CurrentUser: "writer", FilePath: "prompt.jsonl",
	}, &types.CreatePromptVersionReq{Version: "v1"})
	require.ErrorIs(t, err, errorx.ErrDatabaseDuplicateKey)
}

func TestPromptComponent_CreatePromptVersionInvalidFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		fileErr error
	}{
		{name: "file not found", fileErr: errorx.ErrGitFileNotFound},
		{name: "invalid prompt content", content: base64.StdEncoding.EncodeToString([]byte("- invalid"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pc := initializeTestPromptComponent(ctx, t)
			pc.promptVersionStore = &fakePromptVersionStore{}
			repo := &database.Repository{DefaultBranch: "develop"}
			prompt := &database.Prompt{ID: 7, Repository: repo}

			pc.mocks.stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "repo").Return(prompt, nil).Once()
			pc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "writer", repo).Return(&types.UserRepoPermission{CanWrite: true}, nil).Once()
			pc.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
				Namespace: "ns", Name: "repo", Ref: "develop", RepoType: types.PromptRepo,
			}).Return(&types.Commit{ID: "commit-1"}, nil).Once()
			pc.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
				Namespace: "ns", Name: "repo", Ref: "commit-1", Path: "prompt.jsonl", RepoType: types.PromptRepo,
			}).Return(&types.File{Content: tt.content}, tt.fileErr).Once()

			_, err := pc.CreatePromptVersion(ctx, types.PromptVersionReq{
				Namespace: "ns", Name: "repo", CurrentUser: "writer", FilePath: "prompt.jsonl",
			}, &types.CreatePromptVersionReq{Version: "v1"})
			require.Error(t, err)
			if tt.fileErr != nil {
				require.ErrorIs(t, err, tt.fileErr)
			}
		})
	}
}

func TestPromptComponent_CreatePromptVersionForbidden(t *testing.T) {
	ctx := context.Background()
	pc := initializeTestPromptComponent(ctx, t)
	pc.promptVersionStore = &fakePromptVersionStore{}
	repo := &database.Repository{}
	prompt := &database.Prompt{ID: 7, Repository: repo}

	pc.mocks.stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "repo").Return(prompt, nil).Once()
	pc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "reader", repo).Return(&types.UserRepoPermission{CanRead: true}, nil).Once()

	_, err := pc.CreatePromptVersion(ctx, types.PromptVersionReq{
		Namespace: "ns", Name: "repo", CurrentUser: "reader", FilePath: "prompt.jsonl",
	}, &types.CreatePromptVersionReq{Version: "v1"})
	require.True(t, errors.Is(err, errorx.ErrForbidden))
}

func TestPromptComponent_GetPromptVersion(t *testing.T) {
	ctx := context.Background()
	pc := initializeTestPromptComponent(ctx, t)
	pc.promptVersionStore = &fakePromptVersionStore{versions: []database.PromptVersion{{
		ID: 1, PromptID: 7, FilePath: "prompt.jsonl", Version: "v1", Hash: "commit-1",
	}}}
	repo := &database.Repository{}
	prompt := &database.Prompt{ID: 7, Repository: repo}

	pc.mocks.stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "repo").Return(prompt, nil).Once()
	pc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "reader", repo).Return(&types.UserRepoPermission{CanRead: true}, nil).Twice()
	pc.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns", Name: "repo", Ref: "commit-1", Path: "prompt.jsonl", RepoType: types.PromptRepo,
	}).Return(&types.File{Content: "eyJ0aXRsZSI6InYiLCJjb250ZW50IjoiYyIsImxhbmd1YWdlIjoiemgifQ=="}, nil).Once()

	result, err := pc.GetPromptVersion(ctx, types.PromptVersionReq{
		Namespace: "ns", Name: "repo", CurrentUser: "reader", FilePath: "prompt.jsonl", Version: "v1",
	})
	require.NoError(t, err)
	require.Equal(t, "v", result.Prompt.Title)
	require.Equal(t, "commit-1", result.Commit)
}

func TestPromptComponent_UpdatePromptVersionUsesDefaultBranch(t *testing.T) {
	ctx := context.Background()
	pc := initializeTestPromptComponent(ctx, t)
	versionStore := &fakePromptVersionStore{versions: []database.PromptVersion{{
		ID: 1, PromptID: 7, FilePath: "prompt.jsonl", Version: "v1", Hash: "commit-1",
	}}}
	pc.promptVersionStore = versionStore
	repo := &database.Repository{DefaultBranch: "develop"}
	prompt := &database.Prompt{ID: 7, Repository: repo}
	body := &types.UpdatePromptReq{Prompt: types.Prompt{Title: "t", Content: "c", Language: "zh"}}

	pc.mocks.stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "repo").Return(prompt, nil).Once()
	pc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "writer", repo).Return(&types.UserRepoPermission{CanWrite: true}, nil).Once()
	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "writer").Return(database.User{Email: "writer@example.com"}, nil).Once()
	pc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns", Name: "repo", Ref: "develop", Path: "prompt.jsonl", RepoType: types.PromptRepo,
	}).Return("{}", nil).Once()
	pc.mocks.components.repo.EXPECT().UpdateFile(ctx, mock.MatchedBy(func(req *types.UpdateFileReq) bool {
		return req.Branch == "develop" && req.FilePath == "prompt.jsonl" && req.CurrentUser == "writer"
	})).Return(&types.UpdateFileResp{}, nil).Once()
	pc.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: "ns", Name: "repo", Ref: "develop", RepoType: types.PromptRepo,
	}).Return(&types.Commit{ID: "commit-2"}, nil).Once()

	result, err := pc.UpdatePromptVersion(ctx, types.PromptVersionReq{
		Namespace: "ns", Name: "repo", CurrentUser: "writer", FilePath: "prompt.jsonl", Version: "v1",
	}, body)
	require.NoError(t, err)
	require.Equal(t, "commit-2", result.Commit)
	require.Equal(t, "commit-2", versionStore.versions[0].Hash)
}
