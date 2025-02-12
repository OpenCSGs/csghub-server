package component

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestRuntimeArchComponent_ListByRuntimeFrameworkID(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRuntimeArchComponent(ctx, t)

	data := []database.RuntimeArchitecture{
		{ID: 123, ArchitectureName: "arch"},
	}
	rc.mocks.stores.RuntimeArchMock().EXPECT().ListByRuntimeFrameworkID(ctx, int64(1)).Return(
		data, nil,
	)
	resp, err := rc.ListByRuntimeFrameworkID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, data, resp)

}

func TestRuntimeArchComponent_SetArchitectures(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRuntimeArchComponent(ctx, t)

	rc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindByID(ctx, int64(1)).Return(nil, nil)
	rc.mocks.stores.RuntimeArchMock().EXPECT().Add(ctx, database.RuntimeArchitecture{
		RuntimeFrameworkID: 1,
		ArchitectureName:   "foo",
	}).Return(nil)
	rc.mocks.stores.RuntimeArchMock().EXPECT().Add(ctx, database.RuntimeArchitecture{
		RuntimeFrameworkID: 1,
		ArchitectureName:   "bar",
	}).Return(errors.New(""))

	failed, err := rc.SetArchitectures(ctx, int64(1), []string{"foo", "bar"})
	require.Nil(t, err)
	require.Equal(t, []string{"bar"}, failed)

}

func TestRuntimeArchComponent_DeleteArchitectures(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRuntimeArchComponent(ctx, t)

	rc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindByID(ctx, int64(1)).Return(nil, nil)
	rc.mocks.stores.RuntimeArchMock().EXPECT().DeleteByRuntimeIDAndArchName(ctx, int64(1), "foo").Return(nil)
	rc.mocks.stores.RuntimeArchMock().EXPECT().DeleteByRuntimeIDAndArchName(ctx, int64(1), "bar").Return(errors.New(""))

	failed, err := rc.DeleteArchitectures(ctx, int64(1), []string{"foo", "bar"})
	require.Nil(t, err)
	require.Equal(t, []string{"bar"}, failed)

}

func TestRuntimeArchComponent_ScanArchitectures(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRuntimeArchComponent(ctx, t)

	rc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.RuntimeFramework{
			Type: 11,
		}, nil,
	)

	// scan exists mocks
	rc.mocks.stores.RepoMock().EXPECT().GetRepoWithRuntimeByID(ctx, int64(1), []string{"foo"}).Return([]database.Repository{
		{Path: "foo/bar"},
	}, nil)
	rc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "foo",
		Name:      "bar",
		Path:      ConfigFileName,
		RepoType:  types.ModelRepo,
	}).Return(`{"architectures": ["foo","bar"]}`, nil)

	// scan new mocks
	rc.mocks.stores.RepoMock().EXPECT().GetRepoWithoutRuntimeByID(ctx, int64(1), []string{"foo"}, 1000, 0).Return([]database.Repository{
		{Path: "foo/bar"},
	}, nil)
	//page 2
	rc.mocks.stores.RepoMock().EXPECT().GetRepoWithoutRuntimeByID(ctx, int64(1), []string{"foo"}, 1000, 1).Return(nil, nil)
	filter := &types.TagFilter{
		Categories: []string{"runtime_framework", "resource"},
		Scopes:     []types.TagScope{types.ModelTagScope},
	}
	rc.mocks.stores.TagMock().EXPECT().AllTags(ctx, filter).Return([]*database.Tag{}, nil)
	rc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().Add(ctx, int64(1), int64(0), 0).Return(nil)
	rc.mocks.stores.ResourceModelMock().EXPECT().CheckModelNameNotInRFRepo(ctx, "bar", int64(0)).Return(
		&database.ResourceModel{}, nil,
	)
	rc.mocks.stores.ResourceModelMock().EXPECT().FindByModelName(ctx, "bar").Return(
		[]*database.ResourceModel{
			{ResourceName: "r1"},
			{ResourceName: "r2"},
		}, nil,
	)
	var archMap = make(map[string]string)
	archMap["foo"] = "bar"
	models := []string{"foo"}
	err := rc.scanExistModels(ctx, types.ScanReq{
		FrameID:   1,
		FrameType: 0,
		ArchMap:   archMap,
		Models:    models,
		Task:      types.TextGeneration,
	})
	require.Nil(t, err)
	err = rc.scanNewModels(ctx, types.ScanReq{
		FrameID:   1,
		FrameType: 0,
		ArchMap:   archMap,
		Models:    models,
		Task:      types.TextGeneration,
	})
	require.Nil(t, err)

}

func TestRuntimeArchComponent_IsSupportedModelResource(t *testing.T) {

	cases := []struct {
		image   string
		support bool
	}{
		{"foo", false},
		{"bar", true},
		{"foo/bar", true},
		{"bar/foo", false},
		{"foo-bar", true},
		{"foo-model", true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {

			ctx := context.TODO()
			rc := initializeTestRuntimeArchComponent(ctx, t)

			rc.mocks.stores.ResourceModelMock().EXPECT().CheckModelNameNotInRFRepo(ctx, "model", int64(1)).Return(
				&database.ResourceModel{EngineName: "a"}, nil,
			)

			r, err := rc.IsSupportedModelResource(ctx, "meta-model", &database.RuntimeFramework{
				FrameImage: c.image,
			}, 1)
			require.Nil(t, err, nil)
			require.Equal(t, c.support, r)
		})
	}
}

func TestRuntimeArchComponent_GetArchitectureFromConfig(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRuntimeArchComponent(ctx, t)

	rc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "foo",
		Name:      "bar",
		Ref:       "main",
		Path:      ConfigFileName,
		RepoType:  types.ModelRepo,
	}).Return(`{"architectures": ["foo","bar"]}`, nil)

	arch, err := rc.GetArchitecture(ctx, types.TextGeneration, &database.Repository{
		Path:          "foo/bar",
		DefaultBranch: "main",
	})
	require.Nil(t, err)
	require.Equal(t, "foo", arch)

}

func TestRuntimeArchComponent_RemoveRuntimeFrameworkTag(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRuntimeArchComponent(ctx, t)

	rc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindByID(ctx, int64(2)).Return(
		&database.RuntimeFramework{
			FrameImage: "img",
		}, nil,
	)
	rc.mocks.stores.TagMock().EXPECT().RemoveRepoTags(ctx, int64(1), []int64{1}).Return(nil)

	rc.RemoveRuntimeFrameworkTag(ctx, []*database.Tag{
		{Name: "img", ID: 1},
	}, int64(1), int64(2))
}

func TestRuntimeArchComponent_AddRuntimeFrameworkTag(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRuntimeArchComponent(ctx, t)

	rc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindByID(ctx, int64(2)).Return(
		&database.RuntimeFramework{
			FrameImage: "img",
		}, nil,
	)
	rc.mocks.stores.TagMock().EXPECT().UpsertRepoTags(ctx, int64(1), []int64{}, []int64{1}).Return(nil)

	err := rc.AddRuntimeFrameworkTag(ctx, []*database.Tag{
		{Name: "img", ID: 1},
	}, int64(1), int64(2))
	require.Nil(t, err)
}

func TestRuntimeArchComponent_AddResourceTag(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRuntimeArchComponent(ctx, t)

	rc.mocks.stores.ResourceModelMock().EXPECT().FindByModelName(ctx, "model").Return(
		[]*database.ResourceModel{
			{ResourceName: "r1"},
			{ResourceName: "r2"},
		}, nil,
	)
	rc.mocks.stores.TagMock().EXPECT().UpsertRepoTags(ctx, int64(1), []int64{}, []int64{1}).Return(nil)

	err := rc.AddResourceTag(ctx, []*database.Tag{
		{Name: "r1", ID: 1},
	}, "model", int64(1))
	require.Nil(t, err)
}
