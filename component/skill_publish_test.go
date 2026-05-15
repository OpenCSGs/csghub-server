package component

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgitserver "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type skillPublishTestDeps struct {
	component    *skillPublishComponentImpl
	repo         *mockcomponent.MockRepoComponent
	git          *mockgitserver.MockGitServer
	skillStore   *mockdatabase.MockSkillStore
	versionStore *mockdatabase.MockSkillVersionStore
}

func newSkillPublishTestDeps(t *testing.T) *skillPublishTestDeps {
	deps := &skillPublishTestDeps{
		repo:         mockcomponent.NewMockRepoComponent(t),
		git:          mockgitserver.NewMockGitServer(t),
		skillStore:   mockdatabase.NewMockSkillStore(t),
		versionStore: mockdatabase.NewMockSkillVersionStore(t),
	}
	deps.component = &skillPublishComponentImpl{
		repoComponent:     deps.repo,
		gitServer:         deps.git,
		skillStore:        deps.skillStore,
		skillVersionStore: deps.versionStore,
	}
	return deps
}

func TestSkillPublishComponent_Publish(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		deps := newSkillPublishTestDeps(t)
		repo := &database.Repository{ID: 1, Path: "u/r", Name: "r", DefaultBranch: "dev"}
		skill := &database.Skill{ID: 11, RepositoryID: repo.ID, Repository: repo}
		req := &types.PublishSkillVersionReq{
			Namespace: "u",
			Name:      "r",
			Username:  "u",
			Version:   " v1.0.0 ",
			Changelog: "Initial release",
			License:   "MIT",
		}

		deps.skillStore.EXPECT().FindByPath(ctx, "u", "r").Return(skill, nil)
		deps.repo.EXPECT().GetUserRepoPermission(ctx, "u", repo).Return(&types.UserRepoPermission{CanWrite: true}, nil)
		deps.git.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
			Namespace: "u",
			Name:      "r",
			Ref:       "dev",
			RepoType:  types.SkillRepo,
		}).Return(&types.Commit{ID: "abc123"}, nil)
		deps.versionStore.EXPECT().BySkillIDAndVersion(ctx, int64(11), "v1.0.0").Return(nil, sql.ErrNoRows)
		deps.versionStore.EXPECT().Create(ctx, mock.MatchedBy(func(version database.SkillVersion) bool {
			return version.SkillID == 11 &&
				version.Version == "v1.0.0" &&
				version.Hash == "abc123" &&
				version.Changelog == "Initial release" &&
				version.License == "MIT"
		})).Return(&database.SkillVersion{
			ID:      22,
			SkillID: 11,
			Version: "v1.0.0",
			Hash:    "abc123",
		}, nil)

		resp, err := deps.component.Publish(ctx, req)

		require.NoError(t, err)
		require.Equal(t, &types.PublishSkillVersionResp{
			Ok:        true,
			SkillID:   "11",
			VersionID: "22",
			Version:   "v1.0.0",
			Commit:    "abc123",
		}, resp)
	})

	t.Run("forbidden without write permission", func(t *testing.T) {
		ctx := context.Background()
		deps := newSkillPublishTestDeps(t)
		repo := &database.Repository{ID: 1, Path: "u/r", Name: "r"}
		skill := &database.Skill{ID: 11, RepositoryID: repo.ID, Repository: repo}

		deps.skillStore.EXPECT().FindByPath(ctx, "u", "r").Return(skill, nil)
		deps.repo.EXPECT().GetUserRepoPermission(ctx, "reader", repo).Return(&types.UserRepoPermission{CanWrite: false}, nil)

		resp, err := deps.component.Publish(ctx, &types.PublishSkillVersionReq{
			Namespace: "u",
			Name:      "r",
			Username:  "reader",
			Version:   "v1.0.0",
		})

		require.Nil(t, resp)
		require.ErrorIs(t, err, errorx.ErrForbidden)
	})

	t.Run("duplicate version", func(t *testing.T) {
		ctx := context.Background()
		deps := newSkillPublishTestDeps(t)
		repo := &database.Repository{ID: 1, Path: "u/r", Name: "r"}
		skill := &database.Skill{ID: 11, RepositoryID: repo.ID, Repository: repo}

		deps.skillStore.EXPECT().FindByPath(ctx, "u", "r").Return(skill, nil)
		deps.repo.EXPECT().GetUserRepoPermission(ctx, "u", repo).Return(&types.UserRepoPermission{CanWrite: true}, nil)
		deps.git.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
			Namespace: "u",
			Name:      "r",
			Ref:       types.MainBranch,
			RepoType:  types.SkillRepo,
		}).Return(&types.Commit{ID: "abc123"}, nil)
		deps.versionStore.EXPECT().BySkillIDAndVersion(ctx, int64(11), "v1.0.0").Return(&database.SkillVersion{
			ID:      22,
			SkillID: 11,
			Version: "v1.0.0",
			Hash:    "old",
		}, nil)

		resp, err := deps.component.Publish(ctx, &types.PublishSkillVersionReq{
			Namespace: "u",
			Name:      "r",
			Username:  "u",
			Version:   "v1.0.0",
		})

		require.Nil(t, resp)
		require.ErrorIs(t, err, errorx.ErrDatabaseDuplicateKey)
	})
}

func TestResolvePublishVersion(t *testing.T) {
	version, err := resolvePublishVersion(" v1.0.0 ")
	require.NoError(t, err)
	require.Equal(t, "v1.0.0", version)

	version, err = resolvePublishVersion(" ")
	require.Empty(t, version)
	require.True(t, errors.Is(err, errorx.ErrReqParamInvalid))
}
