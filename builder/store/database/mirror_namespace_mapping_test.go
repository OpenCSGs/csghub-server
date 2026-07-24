package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
)

func TestMirrorNamespaceMappingStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorNamespaceMappingStoreWithDB(db)
	_, err := store.Create(ctx, &database.MirrorNamespaceMapping{
		SourceNamespace: "Foo",
		TargetNamespace: "TargetTeam",
	})
	require.Nil(t, err)

	mnm := &database.MirrorNamespaceMapping{}
	err = db.Core.NewSelect().Model(mnm).Order("id DESC").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "Foo", mnm.SourceNamespace)
	require.Equal(t, "TargetTeam", mnm.TargetNamespace)
	_, err = store.Create(ctx, &database.MirrorNamespaceMapping{
		SourceNamespace: "fOO",
		TargetNamespace: "AnotherTeam",
	})
	require.ErrorIs(t, err, errorx.ErrSourceNamespaceMappingExists)
	require.False(t, errors.Is(err, errorx.ErrDatabaseDuplicateKey))

	mnm, err = store.Get(ctx, mnm.ID)
	require.Nil(t, err)
	require.Equal(t, "Foo", mnm.SourceNamespace)
	require.Equal(t, "TargetTeam", mnm.TargetNamespace)

	mnm, err = store.FindBySourceNamespace(ctx, "FOO")
	require.Nil(t, err)
	require.Equal(t, "Foo", mnm.SourceNamespace)
	require.Equal(t, "TargetTeam", mnm.TargetNamespace)
	mnm1, err := store.Update(ctx, &database.MirrorNamespaceMapping{
		ID:              mnm.ID,
		SourceNamespace: "foo",
	})
	require.NoError(t, err)
	require.Equal(t, "Foo", mnm1.SourceNamespace)
	require.Equal(t, "TargetTeam", mnm1.TargetNamespace)

	mnm.SourceNamespace = "fOO"
	mnm1, err = store.Update(ctx, mnm)
	require.Nil(t, err)
	require.Equal(t, mnm.ID, mnm1.ID)
	require.Equal(t, "Foo", mnm1.SourceNamespace)

	mnm.SourceNamespace = "Bar"
	_, err = store.Update(ctx, mnm)
	require.ErrorIs(t, err, errorx.ErrSourceNamespaceMappingNotFound)
	mnm, err = store.Get(ctx, mnm.ID)
	require.NoError(t, err)
	require.Equal(t, "Foo", mnm.SourceNamespace)

	_, err = store.Update(ctx, &database.MirrorNamespaceMapping{ID: mnm.ID})
	require.ErrorIs(t, err, errorx.ErrSourceNamespaceMappingNotFound)

	_, err = store.Update(ctx, &database.MirrorNamespaceMapping{
		ID:              -1,
		SourceNamespace: "Foo",
	})
	require.ErrorIs(t, err, errorx.ErrSourceNamespaceMappingNotFound)

	mnm = &database.MirrorNamespaceMapping{}
	err = db.Core.NewSelect().Model(mnm).Order("id DESC").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "Foo", mnm.SourceNamespace)

	mnms, err := store.Index(ctx)
	require.Nil(t, err)
	require.Equal(t, 29, len(mnms))

	err = store.Delete(ctx, mnm)
	require.Nil(t, err)
	_, err = store.Get(ctx, mnm.ID)
	require.NotNil(t, err)

}

// TestMirrorNamespaceMappingStore_CreateSerializesCaseVariants verifies concurrent case variants cannot create duplicate mappings.
func TestMirrorNamespaceMappingStore_CreateSerializesCaseVariants(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewMirrorNamespaceMappingStoreWithDB(db)

	type createResult struct {
		mapping *database.MirrorNamespaceMapping
		err     error
	}
	start := make(chan struct{})
	results := make(chan createResult, 2)
	for _, sourceNamespace := range []string{"ConcurrentSource", "concurrentsource"} {
		sourceNamespace := sourceNamespace
		go func() {
			<-start
			mapping, err := store.Create(ctx, &database.MirrorNamespaceMapping{
				SourceNamespace: sourceNamespace,
				TargetNamespace: "TargetTeam",
			})
			results <- createResult{mapping: mapping, err: err}
		}()
	}
	close(start)

	successes := 0
	conflicts := 0
	for range 2 {
		result := <-results
		switch {
		case result.err == nil:
			successes++
			require.NotNil(t, result.mapping)
		case errors.Is(result.err, errorx.ErrSourceNamespaceMappingExists):
			conflicts++
		default:
			require.NoError(t, result.err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)

	count, err := db.Core.NewSelect().
		Model((*database.MirrorNamespaceMapping)(nil)).
		Where("LOWER(source_namespace) = LOWER(?)", "ConcurrentSource").
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
