package callback

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestGitCallbackComponent_SetRepoVisibility(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitCallbackComponent(ctx, t)

	require.False(t, gc.setRepoVisibility)
	gc.SetRepoVisibility(true)
	require.True(t, gc.setRepoVisibility)
}

func TestGitCallbackComponent_WatchSpaceChange(t *testing.T) {
	ctx := mock.Anything
	gc := initializeTestGitCallbackComponent(context.TODO(), t)

	gc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "b", "c").Return(
		&database.Space{HasAppFile: true}, nil,
	)
	gc.mocks.spaceComponent.EXPECT().FixHasEntryFile(ctx, &database.Space{
		HasAppFile: true,
	}).Return(nil)
	gc.mocks.spaceComponent.EXPECT().Deploy(ctx, "b", "c", "b").Return(100, nil)

	err := gc.WatchSpaceChange(context.TODO(), &types.GiteaCallbackPushReq{
		Ref: "main",
		Repository: types.GiteaCallbackPushReq_Repository{
			FullName: "spaces_b/c/d",
		},
		Commits: []types.GiteaCallbackPushReq_Commit{
			{Modified: []string{types.ReadmeFileName, "main.py"}},
		},
	})
	require.Nil(t, err)
}

func TestGitCallbackComponent_WatchRepoRelation(t *testing.T) {
	ctx := mock.Anything
	gc := initializeTestGitCallbackComponent(context.TODO(), t)

	gc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "b",
		Name:      "c",
		Ref:       "refs/heads/main",
		Path:      "README.md",
		RepoType:  types.SpaceRepo,
	}).Return("", nil)
	gc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.SpaceRepo, "b", "c").Return(
		&database.Repository{ID: 1}, nil,
	)
	gc.mocks.stores.RepoRelationMock().EXPECT().Override(ctx, int64(1)).Return(nil)

	err := gc.WatchRepoRelation(context.TODO(), &types.GiteaCallbackPushReq{
		Ref: "refs/heads/main",
		Repository: types.GiteaCallbackPushReq_Repository{
			FullName: "spaces_b/c/d",
		},
		Commits: []types.GiteaCallbackPushReq_Commit{
			{Modified: []string{types.ReadmeFileName}},
		},
	})
	require.Nil(t, err)
}

func TestGitCallbackComponent_GenSyncVersion(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitCallbackComponent(context.TODO(), t)

	req := &types.GiteaCallbackPushReq{}
	gc.mocks.syncVersionGenerator.EXPECT().GenSyncVersion(req).Return(nil).Once()

	err := gc.GenSyncVersion(ctx, req)
	require.Nil(t, err)
	req.Repository.Private = true
	err = gc.GenSyncVersion(ctx, req)
	require.Nil(t, err)
}

func TestGitCallbackComponent_SetRepoUpdateTime(t *testing.T) {
	for _, mirror := range []bool{false, true} {
		t.Run(fmt.Sprintf("mirror %v", mirror), func(t *testing.T) {
			dt := time.Date(2022, 2, 2, 2, 0, 0, 0, time.UTC)
			ctx := mock.Anything
			gc := initializeTestGitCallbackComponent(context.TODO(), t)

			gc.mocks.stores.RepoMock().EXPECT().IsMirrorRepo(
				ctx, types.ModelRepo, "ns", "n",
			).Return(mirror, nil)

			if mirror {
				gc.mocks.stores.RepoMock().EXPECT().SetUpdateTimeByPath(
					ctx, types.ModelRepo, "ns", "n", dt,
				).Return(nil)
				gc.mocks.stores.MirrorMock().EXPECT().FindByRepoPath(
					ctx, types.ModelRepo, "ns", "n",
				).Return(&database.Mirror{}, nil)
				gc.mocks.stores.MirrorMock().EXPECT().Update(
					ctx, mock.Anything,
				).RunAndReturn(func(ctx context.Context, m *database.Mirror) error {
					require.GreaterOrEqual(t, m.LastUpdatedAt, time.Now().Add(-5*time.Second))
					return nil
				})
			} else {
				gc.mocks.stores.RepoMock().EXPECT().SetUpdateTimeByPath(
					ctx, types.ModelRepo, "ns", "n", mock.Anything,
				).RunAndReturn(func(ctx context.Context, rt types.RepositoryType, s1, s2 string, tt time.Time) error {
					require.GreaterOrEqual(t, tt, time.Now().Add(-5*time.Second))
					return nil
				})
			}

			err := gc.SetRepoUpdateTime(context.TODO(), &types.GiteaCallbackPushReq{
				Repository: types.GiteaCallbackPushReq_Repository{
					FullName: "models_ns/n",
				},
				HeadCommit: types.GiteaCallbackPushReq_HeadCommit{
					Timestamp: dt.Format(time.RFC3339),
				},
			})
			require.Nil(t, err)
		})
	}
}

func TestGitCallbackComponentImpl_UpdateRepoInfos(t *testing.T) {
	ctx := mock.Anything

	req := &types.GiteaCallbackPushReq{
		Ref: "refs/heads/main",
		Repository: types.GiteaCallbackPushReq_Repository{
			FullName: "models_namespace/repo",
		},
		Commits: []types.GiteaCallbackPushReq_Commit{
			{
				Added:    []string{"README.md", "new_file.txt"},
				Removed:  []string{"old_file.txt"},
				Modified: []string{"config.json"},
			},
		},
	}
	repo := &database.Repository{ID: 1, Path: "namespace/repo"}

	t.Run("should update repo infos successfully", func(t *testing.T) {
		gc := initializeTestGitCallbackComponent(context.Background(), t)
		// Expectations for modifyFiles
		gc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "repo").Return(repo, nil)
		modelInfo := &types.ModelInfo{}
		gc.mocks.runtimeArchComponent.EXPECT().UpdateModelMetadata(ctx, repo).Return(modelInfo, nil)
		gc.mocks.runtimeArchComponent.EXPECT().UpdateRuntimeFrameworkTag(ctx, modelInfo, repo).Return(nil)

		// Expectations for removeFiles
		gc.mocks.tagComponent.EXPECT().UpdateLibraryTags(ctx, types.ModelTagScope, "namespace", "repo", "old_file.txt", "").Return(nil)

		// Expectations for addFiles
		gc.mocks.tagComponent.EXPECT().UpdateLibraryTags(ctx, types.ModelTagScope, "namespace", "repo", "", "new_file.txt").Return(nil)

		getFileRawReq := gitserver.GetRepoInfoByPathReq{
			Namespace: "namespace",
			Name:      "repo",
			Ref:       "refs/heads/main",
			Path:      "README.md",
			RepoType:  types.ModelRepo,
		}
		gc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, getFileRawReq).Return("---\nlicense: apache-2.0\nlanguage:\n  - zh\n---\nreadme content", nil).Once()
		gc.mocks.tagComponent.EXPECT().UpdateMetaTags(ctx, types.ModelTagScope, "namespace", "repo", "---\nlicense: apache-2.0\nlanguage:\n  - zh\n---\nreadme content").Return(nil, nil)
		gc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "namespace", "repo").Return(repo, nil)
		gc.mocks.runtimeArchComponent.EXPECT().UpdateModelMetadata(ctx, repo).Return(modelInfo, nil)
		gc.mocks.runtimeArchComponent.EXPECT().UpdateRuntimeFrameworkTag(ctx, modelInfo, repo).Return(nil)

		err := gc.UpdateRepoInfos(context.Background(), req)
		assert.NoError(t, err)
	})
}
