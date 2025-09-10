package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestMirrorNamespaceMappingStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorNamespaceMappingStoreWithDB(db)
	_, err := store.Create(ctx, &database.MirrorNamespaceMapping{
		SourceNamespace: "foo",
		TargetNamespace: "bar",
	})
	require.Nil(t, err)

	mnm := &database.MirrorNamespaceMapping{}
	err = db.Core.NewSelect().Model(mnm).Order("id DESC").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo", mnm.SourceNamespace)
	require.Equal(t, "bar", mnm.TargetNamespace)

	mnm, err = store.Get(ctx, mnm.ID)
	require.Nil(t, err)
	require.Equal(t, "foo", mnm.SourceNamespace)
	require.Equal(t, "bar", mnm.TargetNamespace)

	mnm, err = store.FindBySourceNamespace(ctx, "foo")
	require.Nil(t, err)
	require.Equal(t, "foo", mnm.SourceNamespace)
	require.Equal(t, "bar", mnm.TargetNamespace)

	mnm.SourceNamespace = "bar"
	mnm1, err := store.Update(ctx, mnm)
	require.Nil(t, err)
	require.Equal(t, mnm.ID, mnm1.ID)
	require.Equal(t, "bar", mnm1.SourceNamespace)

	mnm = &database.MirrorNamespaceMapping{}
	err = db.Core.NewSelect().Model(mnm).Order("id DESC").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "bar", mnm.SourceNamespace)

	mnms, err := store.Index(ctx)
	require.Nil(t, err)
	require.Equal(t, 29, len(mnms))

	err = store.Delete(ctx, mnm)
	require.Nil(t, err)
	_, err = store.Get(ctx, mnm.ID)
	require.NotNil(t, err)

}
