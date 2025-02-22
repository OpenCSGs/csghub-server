package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestSpaceTemplateComponent_Index(t *testing.T) {
	ctx := context.TODO()
	st := initializeTestSpaceTemplateComponent(ctx, t)

	st.mocks.stores.SpaceTemplateMock().EXPECT().Index(ctx).Return([]database.SpaceTemplate{
		{ID: 1, Name: "s", Type: "1"},
	}, nil)

	data, err := st.Index(ctx)
	require.Nil(t, err)
	require.Equal(t, []database.SpaceTemplate{{ID: 1, Name: "s", Type: "1"}}, data)
}

func TestSpaceTemplateComponent_Create(t *testing.T) {
	ctx := context.TODO()
	st := initializeTestSpaceTemplateComponent(ctx, t)

	req := &types.SpaceTemplateReq{
		Type: "docker",
		Name: "test",
	}
	st.mocks.stores.SpaceTemplateMock().EXPECT().Create(ctx, database.SpaceTemplate{
		Type: "docker",
		Name: "test",
	}).Return(&database.SpaceTemplate{
		Type: "docker",
		Name: "test",
	}, nil)

	res, err := st.Create(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &database.SpaceTemplate{Name: "test", Type: "docker"}, res)
}

func TestSpaceTemplateComponent_Update(t *testing.T) {
	ctx := context.TODO()
	st := initializeTestSpaceTemplateComponent(ctx, t)

	newname := "newname"
	req := &types.UpdateSpaceTemplateReq{
		ID:   int64(1),
		Name: &newname,
	}
	st.mocks.stores.SpaceTemplateMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceTemplate{
		Type: "docker",
		Name: "test",
	}, nil)

	st.mocks.stores.SpaceTemplateMock().EXPECT().Update(ctx, database.SpaceTemplate{
		Type: "docker",
		Name: newname,
	}).Return(&database.SpaceTemplate{
		Type: "docker",
		Name: newname,
	}, nil)

	res, err := st.Update(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &database.SpaceTemplate{Name: newname, Type: "docker"}, res)
}

func TestSpaceTemplateComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	st := initializeTestSpaceTemplateComponent(ctx, t)

	st.mocks.stores.SpaceTemplateMock().EXPECT().Delete(ctx, int64(1)).Return(nil)

	err := st.Delete(ctx, int64(1))
	require.Nil(t, err)
}

func TestSpaceTemplateComponent_FindAllByType(t *testing.T) {
	ctx := context.TODO()
	st := initializeTestSpaceTemplateComponent(ctx, t)

	st.mocks.stores.SpaceTemplateMock().EXPECT().FindAllByType(ctx, "docker").Return([]database.SpaceTemplate{
		{ID: 1, Name: "s", Type: "docker"},
	}, nil)

	data, err := st.FindAllByType(ctx, "docker")
	require.Nil(t, err)
	require.Equal(t, []database.SpaceTemplate{{ID: 1, Name: "s", Type: "docker"}}, data)
}

func TestSpaceTemplateComponent_FindByName(t *testing.T) {
	ctx := context.TODO()
	st := initializeTestSpaceTemplateComponent(ctx, t)

	st.mocks.stores.SpaceTemplateMock().EXPECT().FindByName(ctx, "docker", "t1").Return(&database.SpaceTemplate{
		ID: 1, Name: "t1", Type: "docker",
	}, nil)

	data, err := st.FindByName(ctx, "docker", "t1")
	require.Nil(t, err)
	require.Equal(t, &database.SpaceTemplate{ID: 1, Name: "t1", Type: "docker"}, data)
}
