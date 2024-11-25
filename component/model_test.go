package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	gsmock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func NewTestModelComponent(stores *tests.MockStores, git gitserver.GitServer) (ModelComponent, error) {
	c := &modelComponentImpl{}
	c.ms = stores.Model
	c.rs = stores.Repo
	c.SS = stores.SpaceResource
	c.us = stores.User
	c.ts = stores.Tag
	c.ds = stores.Dataset
	c.repoComponentImpl = &repoComponentImpl{
		git:  git,
		user: stores.User,
		repo: stores.Repo,
	}
	return c, nil
}

func TestModelComponent_SetRelationDatasetsAndPrompts(t *testing.T) {
	ctx := context.TODO()

	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	model, err := NewTestModelComponent(stores, gitServer)
	require.Nil(t, err)

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		Email:    "foo@bar.com",
		RoleMask: "foo",
	}, nil).Once()
	err = model.SetRelationDatasets(ctx, types.RelationDatasets{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
	})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "only admin")

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		Email:    "foo@bar.com",
		RoleMask: "foo-admin",
	}, nil).Once()

	stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(&database.Repository{}, nil).Once()
	// ---
	// foo: "foo"
	// bar: "bar"
	gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       types.MainBranch,
		Path:      REPOCARD_FILENAME,
		RepoType:  types.ModelRepo,
	}).Return(&types.File{
		Content: "LS0tCiBmb286ICJmb28iCiBiYXI6ICJiYXIi",
	}, nil).Once()

	// ---
	// bar: bar
	// datasets:
	//     - a
	//     - b
	// foo: foo

	// ---
	gitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Branch:    types.MainBranch,
		Message:   "update dataset tags",
		FilePath:  REPOCARD_FILENAME,
		RepoType:  types.ModelRepo,
		Namespace: "ns",
		Name:      "n",
		Username:  "foo",
		Email:     "foo@bar.com",
		Content:   "LS0tCmJhcjogYmFyCmRhdGFzZXRzOgogICAgLSBhCiAgICAtIGIKZm9vOiBmb28KCi0tLQ==",
	}).Return(nil).Once()

	err = model.SetRelationDatasets(ctx, types.RelationDatasets{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
		Datasets:    []string{"a", "b"},
	})
	require.Nil(t, err)

}
