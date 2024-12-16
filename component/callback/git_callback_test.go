package callback

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
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

func TestGitCallbackComponent_UpdateRepoInfos(t *testing.T) {
	ctx := context.TODO()
	gc := initializeTestGitCallbackComponent(context.TODO(), t)

	// modified mock
	gc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		&database.Repository{ID: 1, Path: "foo/bar"}, nil,
	)
	gc.mocks.runtimeArchComponent.EXPECT().GetArchitectureFromConfig(ctx, "ns", "n").Return("foo", nil)
	gc.mocks.stores.TagMock().EXPECT().GetTagsByScopeAndCategories(
		ctx, database.ModelTagScope, []string{"runtime_framework", "resource"},
	).Return([]*database.Tag{{Name: "t1"}}, nil)
	gc.mocks.runtimeArchComponent.EXPECT().AddResourceTag(
		ctx, []*database.Tag{{Name: "t1"}}, "bar", int64(1),
	).Return(nil)
	gc.mocks.stores.RuntimeArchMock().EXPECT().ListByRArchNameAndModel(ctx, "foo", "bar").Return(
		[]database.RuntimeArchitecture{{ID: 11, RuntimeFrameworkID: 111}}, nil,
	)
	gc.mocks.stores.RuntimeFrameworkMock().EXPECT().ListByIDs(ctx, []int64{111}).Return(
		[]database.RuntimeFramework{{ID: 12, FrameName: "fm"}}, nil,
	)
	gc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().GetByRepoIDs(ctx, int64(1)).Return(
		[]database.RepositoriesRuntimeFramework{{RuntimeFrameworkID: 13}}, nil,
	)
	gc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().Delete(ctx, int64(13), int64(1), 0).Return(nil)
	gc.mocks.runtimeArchComponent.EXPECT().RemoveRuntimeFrameworkTag(
		ctx, []*database.Tag{{Name: "t1"}}, int64(1), int64(13),
	).Return()
	gc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().Add(ctx, int64(12), int64(1), 0).Return(nil)
	gc.mocks.runtimeArchComponent.EXPECT().AddRuntimeFrameworkTag(
		ctx, []*database.Tag{{Name: "t1"}}, int64(1), int64(12),
	).Return(nil)
	// removed mock
	gc.mocks.tagComponent.EXPECT().UpdateLibraryTags(
		ctx, database.ModelTagScope, "ns", "n", "bar.go", "",
	).Return(nil)
	gc.mocks.tagComponent.EXPECT().ClearMetaTags(ctx, types.ModelRepo, "ns", "n").Return(nil)
	// added mock
	gc.mocks.tagComponent.EXPECT().UpdateLibraryTags(
		ctx, database.ModelTagScope, "ns", "n", "", "foo.go",
	).Return(nil)
	gc.mocks.gitServer.EXPECT().GetRepoFileRaw(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "refs/heads/main",
		Path:      "README.md",
		RepoType:  types.ModelRepo,
	}).Return("", nil)
	gc.mocks.tagComponent.EXPECT().UpdateMetaTags(
		ctx, database.ModelTagScope, "ns", "n", "",
	).Return(nil, nil)

	err := gc.UpdateRepoInfos(ctx, &types.GiteaCallbackPushReq{
		Ref: "refs/heads/main",
		Repository: types.GiteaCallbackPushReq_Repository{
			FullName: "models_ns/n",
		},
		Commits: []types.GiteaCallbackPushReq_Commit{
			{
				Modified: []string{component.ConfigFileName},
				Removed:  []string{"bar.go", types.ReadmeFileName},
				Added:    []string{"foo.go", types.ReadmeFileName},
			},
		},
	})
	require.Nil(t, err)
}
