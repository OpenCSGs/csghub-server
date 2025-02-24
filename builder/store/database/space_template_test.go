package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestSpaceTemplateStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceTemplateStoreWithDB(db)

	_, err := store.Create(ctx, database.SpaceTemplate{
		Type:     "docker",
		Name:     "t1",
		ShowName: "testname",
		Enable:   true,
	})
	require.Nil(t, err)

	st := &database.SpaceTemplate{}
	err = db.Core.NewSelect().Model(st).Where("name=?", "t1").Scan(ctx, st)
	require.Nil(t, err)
	require.Equal(t, "docker", st.Type)

	st, err = store.FindByID(ctx, st.ID)
	require.Nil(t, err)
	require.Equal(t, "docker", st.Type)

	_, err = store.Create(ctx, database.SpaceTemplate{
		Type:     "gradio",
		Name:     "t2",
		ShowName: "testname2",
		Enable:   false,
	})
	require.Nil(t, err)

	sss, err := store.Index(ctx)
	require.Nil(t, err)
	require.Equal(t, 3, len(sss))
	require.Equal(t, "docker", sss[0].Type)

	st.Name = "tt2"
	_, err = store.Update(ctx, *st)
	require.Nil(t, err)
	st, err = store.FindByID(ctx, st.ID)
	require.Nil(t, err)
	require.Equal(t, "tt2", st.Name)

	err = store.Delete(ctx, st.ID)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, st.ID)
	require.NotNil(t, err)

	sss, err = store.FindAllByType(ctx, "docker")
	require.Nil(t, err)
	require.Equal(t, 1, len(sss))

	st, err = store.FindByName(ctx, "docker", "ChatUI")
	require.Nil(t, err)
	require.Equal(t, "ChatUI", st.Name)
}
