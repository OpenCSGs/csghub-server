//go:build !saas

package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// testRepoVisibilityWithWrite verifies CE and EE visibility changes use repository write permission for every repository type.
func testRepoVisibilityWithWrite(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name          string
		repoType      types.RepositoryType
		namespaceType database.NamespaceType
		private       bool
		canWrite      bool
	}{
		{name: "organization model becomes public", repoType: types.ModelRepo, namespaceType: database.OrgNamespace, canWrite: true},
		{name: "personal model becomes public", repoType: types.ModelRepo, namespaceType: database.UserNamespace, canWrite: true},
		{name: "personal dataset becomes private", repoType: types.DatasetRepo, namespaceType: database.UserNamespace, private: true, canWrite: true},
		{name: "personal space becomes public", repoType: types.SpaceRepo, namespaceType: database.UserNamespace, canWrite: true},
		{name: "personal code becomes public", repoType: types.CodeRepo, namespaceType: database.UserNamespace, canWrite: true},
		{name: "personal prompt becomes public", repoType: types.PromptRepo, namespaceType: database.UserNamespace, canWrite: true},
		{name: "personal MCP server becomes public", repoType: types.MCPServerRepo, namespaceType: database.UserNamespace, canWrite: true},
		{name: "personal template becomes public", repoType: types.TemplateRepo, namespaceType: database.UserNamespace, canWrite: true},
		{name: "personal skill becomes public", repoType: types.SkillRepo, namespaceType: database.UserNamespace, canWrite: true},
		{name: "write permission is required", repoType: types.ModelRepo, namespaceType: database.UserNamespace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := initializeTestRepoComponent(ctx, t)
			dbRepo := &database.Repository{ID: 1, Path: "ns/repo", Private: !tc.private, RepositoryType: tc.repoType}
			repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "ns", string(tc.repoType), "repo").Return(dbRepo, nil)
			repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
				Path: "ns", NamespaceType: tc.namespaceType,
			}, nil)
			repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "writer").Return(database.User{
				Username: "writer", UUID: "writer-uuid",
			}, nil).Twice()
			repoAuthorizerMock(repo).EXPECT().Check(ctx, mock.MatchedBy(func(req rebac.CheckRequest) bool {
				return req.Relation == rebac.RepositoryCanWrite && req.Object == rebac.RepositoryObject(dbRepo.ID)
			})).Return(rebac.Decision{Allowed: tc.canWrite}, nil).Once()
			if tc.canWrite {
				repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.MatchedBy(func(req gitserver.UpdateRepoReq) bool {
					return req.Private == tc.private
				})).Return(&gitserver.CreateRepoResp{}, nil)
				repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.MatchedBy(func(updated database.Repository) bool {
					return updated.Private == tc.private
				})).Return(&database.Repository{Private: tc.private}, nil)
			}

			result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
				Username: "writer", Namespace: "ns", Name: "repo", RepoType: tc.repoType, Private: &tc.private,
			})
			if !tc.canWrite {
				require.ErrorIs(t, err, errorx.ErrForbidden)
				require.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.private, result.Private)
		})
	}
}
