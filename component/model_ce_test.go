//go:build !ee && !saas

package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestModelComponent_Deploy(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Model{
		RepositoryID: int64(123),
		Repository: &database.Repository{
			ID:   1,
			Path: "foo",
		},
	}, nil)
	mc.mocks.stores.DeployTaskMock().EXPECT().GetServerlessDeployByRepID(ctx, int64(1)).Return(
		nil, nil,
	)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(11)).Return(
		&database.RuntimeFramework{}, nil,
	)
	mc.mocks.components.repo.EXPECT().IsAdminRole(database.User{
		RoleMask: "admin",
	}).Return(true)
	mc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(123)).Return(
		&database.SpaceResource{
			ID:        123,
			Resources: `{"memory": "foo"}`,
		}, nil,
	)

	mc.mocks.deployer.EXPECT().CheckResourceAvailable(ctx, "cluster", int64(0), &types.HardWare{
		Memory: "foo",
	}).Return(true, nil)
	mc.mocks.deployer.EXPECT().Deploy(ctx, types.DeployRepo{
		DeployName: "dp",
		Path:       "foo",
		Hardware:   "{\"memory\": \"foo\"}",
		Annotation: "{\"hub-res-name\":\"ns/n\",\"hub-res-type\":\"model\"}",
		ClusterID:  "cluster",
		RepoID:     1,
		SKU:        "123",
		Type:       types.ServerlessType,
	}).Return(111, nil)

	id, err := mc.Deploy(ctx, types.DeployActReq{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "user",
		DeployType:  types.ServerlessType,
	}, types.ModelRunReq{
		RuntimeFrameworkID: 11,
		ResourceID:         123,
		ClusterID:          "cluster",
		DeployName:         "dp",
	})
	require.Nil(t, err)
	require.Equal(t, int64(111), id)

}

func TestModelComponent_containerImg(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	cases := []struct {
		hd  types.HardWare
		img string
	}{
		{hd: types.HardWare{}, img: "cpu"},
		{hd: types.HardWare{Gpu: types.GPU{}}, img: "cpu"},
		{hd: types.HardWare{Gpu: types.GPU{Num: "1"}}, img: "gpu"},
	}

	for _, c := range cases {
		v := mc.containerImg(&database.RuntimeFramework{
			FrameImage:    "gpu",
			FrameCpuImage: "cpu",
		}, c.hd)
		require.Equal(t, c.img, v)
	}
}
