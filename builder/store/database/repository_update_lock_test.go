package database

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"opencsg.com/csghub-server/common/types"
)

func TestCanonicalNamespaceLockOrderUsesCaseInsensitivePrimaryKey(t *testing.T) {
	paths := []string{"Beta", "alpha", "ALPHA", "beta"}

	ordered := canonicalNamespaceLockOrder(paths)

	require.Equal(t, []string{"ALPHA", "alpha", "Beta", "beta"}, ordered)
	require.Equal(t, []string{"Beta", "alpha", "ALPHA", "beta"}, paths, "input must not be mutated")
}

func TestRepoStoreUpdateRepoLocksChangedNamespacesInStableOrder(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	bunDB.RegisterModel((*RepositoryTag)(nil))
	db := &DB{Operator: Operator{Core: bunDB}, BunDB: bunDB}
	store := NewRepoStoreWithDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "repositories" AS "repository" WHERE \(id = 42\).*deleted_at.*NULL$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "path", "repository_type"}).
			AddRow(42, "z-old/repo", types.ModelRepo))
	mock.ExpectQuery(`SELECT .* FROM "namespaces" AS "namespace".*LOWER\(path\) = LOWER\('a-new'\).*FOR KEY SHARE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "path", "deleted_at"}).AddRow(1, "a-new", nil))
	mock.ExpectQuery(`SELECT .* FROM "namespaces" AS "namespace".*LOWER\(path\) = LOWER\('z-old'\).*FOR KEY SHARE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "path", "deleted_at"}).AddRow(2, "z-old", nil))
	mock.ExpectQuery(`SELECT .* FROM "repositories" AS "repository" WHERE \(id = 42\).*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "path", "repository_type"}).
			AddRow(42, "z-old/repo", types.ModelRepo))
	mock.ExpectQuery(`SELECT .* FROM "sync_versions"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(`UPDATE "repositories"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	_, err = store.UpdateRepo(context.Background(), Repository{
		ID: 42, Path: "a-new/repo", Name: "repo", RepositoryType: types.ModelRepo,
	})
	require.NoError(t, err)
	require.NoError(t, bunDB.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepoStoreUpdateRepoRejectsConcurrentPathChange(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	bunDB.RegisterModel((*RepositoryTag)(nil))
	db := &DB{Operator: Operator{Core: bunDB}, BunDB: bunDB}
	store := NewRepoStoreWithDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "repositories" AS "repository" WHERE \(id = 42\).*deleted_at.*NULL$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "path", "repository_type"}).AddRow(42, "old/repo", types.ModelRepo))
	mock.ExpectQuery(`SELECT .* FROM "namespaces" AS "namespace".*LOWER\(path\) = LOWER\('new'\).*FOR KEY SHARE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "path", "deleted_at"}).AddRow(1, "new", nil))
	mock.ExpectQuery(`SELECT .* FROM "namespaces" AS "namespace".*LOWER\(path\) = LOWER\('old'\).*FOR KEY SHARE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "path", "deleted_at"}).AddRow(2, "old", nil))
	mock.ExpectQuery(`SELECT .* FROM "repositories" AS "repository" WHERE \(id = 42\).*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "path", "repository_type"}).AddRow(42, "other/repo", types.ModelRepo))
	mock.ExpectRollback()
	mock.ExpectClose()

	_, err = store.UpdateRepo(context.Background(), Repository{ID: 42, Path: "new/repo", RepositoryType: types.ModelRepo})
	require.ErrorContains(t, err, `repository path changed concurrently from "old/repo" to "other/repo"`)
	require.NoError(t, bunDB.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}
