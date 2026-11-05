package database_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database/migrations"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// TestRepoStore_UpdateDescriptionIfEmpty verifies that existing descriptions are preserved.
func TestRepoStore_UpdateDescriptionIfEmpty(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		UserID:         123,
		Path:           "description-owner/description-repo",
		GitPath:        "models_description-owner/description-repo",
		Name:           "description-repo",
		RepositoryType: types.ModelRepo,
	}))
	require.NoError(t, err)

	descriptionStore := store.(database.RepoDescriptionStore)
	updated, err := descriptionStore.UpdateDescriptionIfEmpty(ctx, repo.ID, "generated description")
	require.NoError(t, err)
	require.True(t, updated)

	updated, err = descriptionStore.UpdateDescriptionIfEmpty(ctx, repo.ID, "replacement description")
	require.NoError(t, err)
	require.False(t, updated)

	got, err := store.FindById(ctx, repo.ID)
	require.NoError(t, err)
	require.Equal(t, "generated description", got.Description)
}

func TestRepoStore_UpdateRepoPreservesXnetMigrationIntent(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))

	for _, intent := range []database.XnetMigrationIntent{database.XnetMigrationActive, database.XnetMigrationPaused} {
		t.Run(string(intent), func(t *testing.T) {
			repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
				UserID: 991, Path: "intent-owner/" + string(intent), GitPath: "models_intent-owner/" + string(intent),
				Name: string(intent), RepositoryType: types.ModelRepo,
			}))
			require.NoError(t, err)
			_, err = db.Core.NewUpdate().Model((*database.Repository)(nil)).
				Set("xnet_migration_intent = ?", intent).Where("id = ?", repo.ID).Exec(ctx)
			require.NoError(t, err)

			repo.XnetMigrationIntent = database.XnetMigrationInactive
			repo.Nickname = "unrelated update"
			_, err = store.UpdateRepo(ctx, *repo)
			require.NoError(t, err)

			var reloaded database.Repository
			require.NoError(t, db.Core.NewSelect().Model(&reloaded).Where("id = ?", repo.ID).Scan(ctx))
			require.Equal(t, intent, reloaded.XnetMigrationIntent)
			require.Equal(t, "unrelated update", reloaded.Nickname)
		})
	}
}

type updateNamespaceLockHook struct {
	path   string
	locked chan struct{}
	once   sync.Once
}

type moveOutDeleteHookContextKey struct{}
type moveOutUpdateHookContextKey struct{}
type mixedCaseUpdateHookContextKey struct{}
type mixedCaseDeleteHookContextKey struct{}

type moveOutDeleteHook struct {
	namespaceLocked chan struct{}
	resume          chan struct{}
	updateRead      chan struct{}
	updateAttempt   chan struct{}
	once            sync.Once
	updateOnce      sync.Once
	attemptOnce     sync.Once
}

type mixedCaseNamespaceOrderHook struct {
	firstPath     string
	blockedPath   string
	firstLocked   chan struct{}
	deleteAttempt chan struct{}
	resume        chan struct{}
	firstOnce     sync.Once
	attemptOnce   sync.Once
}

func (h *mixedCaseNamespaceOrderHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if ctx.Value(mixedCaseDeleteHookContextKey{}) == true && strings.Contains(event.Query, `FROM "namespaces"`) && strings.Contains(event.Query, h.blockedPath) {
		h.attemptOnce.Do(func() { close(h.deleteAttempt) })
	}
	return ctx
}

func (h *mixedCaseNamespaceOrderHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	if event.Err == nil && ctx.Value(mixedCaseUpdateHookContextKey{}) == true && strings.Contains(strings.ToUpper(event.Query), "FOR KEY SHARE") && strings.Contains(event.Query, h.firstPath) {
		h.firstOnce.Do(func() { close(h.firstLocked) })
		<-h.resume
	}
}

func (h *moveOutDeleteHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if ctx.Value(moveOutUpdateHookContextKey{}) == true && strings.Contains(event.Query, `FROM "namespaces"`) {
		h.attemptOnce.Do(func() { close(h.updateAttempt) })
	}
	if ctx.Value(moveOutDeleteHookContextKey{}) != true || !strings.Contains(event.Query, `FROM "repositories"`) {
		return ctx
	}
	select {
	case <-h.namespaceLocked:
		<-h.resume
	default:
	}
	return ctx
}

func (h *moveOutDeleteHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	query := strings.ToUpper(event.Query)
	if event.Err == nil && ctx.Value(moveOutUpdateHookContextKey{}) == true && strings.Contains(event.Query, `FROM "repositories"`) && !strings.Contains(query, "FOR UPDATE") {
		h.updateOnce.Do(func() { close(h.updateRead) })
	}
	if event.Err == nil && ctx.Value(moveOutDeleteHookContextKey{}) == true && strings.Contains(query, "FOR UPDATE") && strings.Contains(event.Query, `FROM "namespaces"`) {
		h.once.Do(func() { close(h.namespaceLocked) })
	}
}

func (h *updateNamespaceLockHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

func (h *updateNamespaceLockHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	query := strings.ToUpper(event.Query)
	if event.Err == nil && strings.Contains(query, "FOR KEY SHARE") && strings.Contains(event.Query, h.path) {
		h.once.Do(func() { close(h.locked) })
	}
}

// TestRepositoryGitalyPath verifies mirror workers derive both legacy and hashed storage paths from loaded metadata.
func TestRepositoryGitalyPath(t *testing.T) {
	t.Run("legacy path", func(t *testing.T) {
		repo := database.Repository{
			RepositoryType: types.ModelRepo,
			Path:           "NameSpace/RepoName",
		}

		require.Equal(t, "models_namespace/reponame.git", repo.GitalyPath())
	})

	t.Run("hashed path", func(t *testing.T) {
		repo := database.Repository{
			ID:             123,
			RepositoryType: types.ModelRepo,
			Path:           "namespace/repo",
			Hashed:         true,
		}
		digest := database.SHA256("123")

		require.Equal(
			t,
			fmt.Sprintf("%s/%s/%s/%s.git", database.HashedRepoPathPrefix, digest[:2], digest[2:4], digest),
			repo.GitalyPath(),
		)
	})
}

func TestRepoStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	seedTupleNamespace(t, db, "u")
	seedTupleNamespace(t, db, "bars_u")
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))

	_, err := store.CreateRepo(ctx, database.Repository{
		Name:           "repo1",
		UserID:         123,
		GitPath:        "models_u/bar",
		Path:           "u/bar",
		RepositoryType: types.ModelRepo,
	})
	require.Nil(t, err)

	rp := &database.Repository{}
	err = db.Core.NewSelect().Model(rp).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	rp, err = store.FindById(ctx, rp.ID)
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	rps, err := store.FindByIds(ctx, []int64{rp.ID})
	require.Nil(t, err)
	require.Equal(t, 1, len(rps))
	require.Equal(t, "repo1", rps[0].Name)

	rps, err = store.All(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(rps))
	require.Equal(t, "repo1", rps[0].Name)

	rp, err = store.Find(ctx, "u", "model", "bAr")
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	exist, err := store.Exists(ctx, "model", "u", "bar")
	require.Nil(t, err)
	require.True(t, exist)

	rp, err = store.FindByPath(ctx, "model", "u", "bAr")
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	rp, err = store.FindByGitPath(ctx, "models_u/bAr")
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	rps, err = store.FindByGitPaths(ctx, []string{"models_u/bAr"})
	require.Nil(t, err)
	require.Equal(t, 1, len(rps))
	require.Equal(t, "repo1", rps[0].Name)

	rpsp, err := store.ByUser(ctx, 123, 100, 0)
	require.Nil(t, err)
	require.Equal(t, 1, len(rpsp))
	require.Equal(t, "repo1", rpsp[0].Name)
	rpsp, err = store.ByUser(ctx, 125, 100, 0)
	require.Nil(t, err)
	require.Equal(t, 0, len(rpsp))

	rpn := *rp
	rpn.Name = "repo1-new"
	_, err = store.UpdateRepo(ctx, rpn)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(rp).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "repo1-new", rp.Name)

	_, err = store.UpdateOrCreateRepo(ctx, database.Repository{
		Name:           "MyName",
		Nickname:       "Original Name",
		UserID:         231,
		Path:           "bars_u/MyName",
		GitPath:        "codes_bars_u/MyName",
		RepositoryType: types.CodeRepo,
	})
	require.Nil(t, err)
	rp = &database.Repository{}
	err = db.Core.NewSelect().Model(rp).Where("user_id=?", 231).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "MyName", rp.Name)

	rp, err = store.UpdateOrCreateRepo(ctx, database.Repository{
		Name:           "myname",
		Nickname:       "Updated Name",
		UserID:         231,
		Path:           "BARS_U/myname",
		GitPath:        "codes_BARS_U/myname",
		RepositoryType: types.CodeRepo,
	})
	require.Nil(t, err)
	require.Equal(t, "BARS_U/myname", rp.Path)
	require.Equal(t, "myname", rp.Name)
	require.Equal(t, "codes_BARS_U/myname", rp.GitPath)
	require.Equal(t, "Updated Name", rp.Nickname)

	cnt, err := store.CountByRepoType(ctx, types.CodeRepo)
	require.Nil(t, err)
	require.Equal(t, 1, cnt)

}

// TestRepoStore_UpdateRepoSerializesMoveWithOrganizationDeletion includes a concurrent move in deletion cleanup.
func TestRepoStore_UpdateRepoSerializesMoveWithOrganizationDeletion(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	setOrgStoreTestDB(t, db)
	ctx := context.Background()
	user := createOrganizationUnitMemberTestUser(t, ctx, db, "repository-move-delete-user")
	orgPath := "repository-move-delete-target"
	orgUUID := uuid.New()
	orgStore := database.NewOrgStoreWithJobClients(false, &testRepositoryDeletionJobClient{}, nil, tests.NewFixtureRebacTupleJobClient(db))
	repoStore := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	require.NoError(t, orgStore.Create(ctx,
		&database.Organization{Name: orgPath, Nickname: orgPath, UUID: orgUUID, UserID: user.ID},
		&database.Namespace{Path: orgPath, UUID: orgUUID.String(), UserID: user.ID},
	))
	_, err := db.Core.NewInsert().Model(&database.Namespace{
		Path: "repository-move-delete-source", UUID: uuid.NewString(), UserID: user.ID, NamespaceType: database.UserNamespace,
	}).Exec(ctx)
	require.NoError(t, err)
	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		Name: "repo", Path: "repository-move-delete-source/repo",
		GitPath: "models_repository-move-delete-source/repo", RepositoryType: types.ModelRepo, UserID: user.ID,
	})
	require.NoError(t, err)

	locked := make(chan struct{})
	db.BunDB.AddQueryHook(&updateNamespaceLockHook{path: orgPath, locked: locked})
	blocker, err := db.BunDB.DB.Conn(ctx)
	require.NoError(t, err)
	defer blocker.Close()
	_, err = blocker.ExecContext(ctx, "BEGIN")
	require.NoError(t, err)
	_, err = blocker.ExecContext(ctx, "LOCK TABLE sync_versions IN ACCESS EXCLUSIVE MODE")
	require.NoError(t, err)
	released := false
	defer func() {
		if !released {
			_, _ = blocker.ExecContext(ctx, "ROLLBACK")
		}
	}()

	repo.Path = orgPath + "/repo"
	updateResult := make(chan error, 1)
	go func() {
		_, updateErr := repoStore.UpdateRepo(ctx, *repo)
		updateResult <- updateErr
	}()
	select {
	case <-locked:
	case updateErr := <-updateResult:
		require.NoError(t, updateErr)
		require.FailNow(t, "repository update finished before locking the target namespace")
	case <-time.After(5 * time.Second):
		require.FailNow(t, "repository update did not lock the target namespace")
	}

	deleteResult := make(chan error, 1)
	go func() {
		_, deleteErr := orgStore.Delete(ctx, orgPath)
		deleteResult <- deleteErr
	}()
	select {
	case deleteErr := <-deleteResult:
		require.NoError(t, deleteErr)
		require.FailNow(t, "organization deletion completed while repository move held the namespace lock")
	case <-time.After(500 * time.Millisecond):
	}

	_, err = blocker.ExecContext(ctx, "COMMIT")
	require.NoError(t, err)
	released = true
	require.NoError(t, <-updateResult)
	require.NoError(t, <-deleteResult)
	exists, err := db.Core.NewSelect().Model((*database.Repository)(nil)).Where("id = ?", repo.ID).Exists(ctx)
	require.NoError(t, err)
	require.False(t, exists)

	deletedPath := "repository-move-already-deleted-target"
	deletedUUID := uuid.New()
	require.NoError(t, orgStore.Create(ctx,
		&database.Organization{Name: deletedPath, Nickname: deletedPath, UUID: deletedUUID, UserID: user.ID},
		&database.Namespace{Path: deletedPath, UUID: deletedUUID.String(), UserID: user.ID},
	))
	repoAfterDelete, err := repoStore.CreateRepo(ctx, database.Repository{
		Name: "repo-after-delete", Path: "repository-move-delete-source/repo-after-delete",
		GitPath: "models_repository-move-delete-source/repo-after-delete", RepositoryType: types.ModelRepo, UserID: user.ID,
	})
	require.NoError(t, err)
	_, err = orgStore.Delete(ctx, deletedPath)
	require.NoError(t, err)
	repoAfterDelete.Path = deletedPath + "/repo-after-delete"
	_, err = repoStore.UpdateRepo(ctx, *repoAfterDelete)
	require.ErrorContains(t, err, "repository namespace \""+deletedPath+"\" is deleted")
	require.NoError(t, db.Core.NewSelect().Model(repoAfterDelete).Where("id = ?", repoAfterDelete.ID).Scan(ctx))
	require.Equal(t, "repository-move-delete-source/repo-after-delete", repoAfterDelete.Path)
}

// TestRepoStore_UpdateRepoMoveOutUsesNamespaceBeforeRepositoryLockOrder preserves lock ordering during a move out.
func TestRepoStore_UpdateRepoMoveOutUsesNamespaceBeforeRepositoryLockOrder(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	setOrgStoreTestDB(t, db)
	ctx := context.Background()
	user := createOrganizationUnitMemberTestUser(t, ctx, db, "repository-move-out-user")
	sourcePath := "repository-move-out-source"
	targetPath := "repository-move-out-target"
	orgUUID := uuid.New()
	orgStore := database.NewOrgStoreWithJobClients(false, &testRepositoryDeletionJobClient{}, nil, tests.NewFixtureRebacTupleJobClient(db))
	repoStore := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	require.NoError(t, orgStore.Create(ctx,
		&database.Organization{Name: sourcePath, Nickname: sourcePath, UUID: orgUUID, UserID: user.ID},
		&database.Namespace{Path: sourcePath, UUID: orgUUID.String(), UserID: user.ID},
	))
	_, err := db.Core.NewInsert().Model(&database.Namespace{
		Path: targetPath, UUID: uuid.NewString(), UserID: user.ID,
	}).Exec(ctx)
	require.NoError(t, err)
	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		Name: "repo", Path: sourcePath + "/repo", GitPath: "models_" + sourcePath + "/repo",
		RepositoryType: types.ModelRepo, UserID: user.ID,
	})
	require.NoError(t, err)

	hook := &moveOutDeleteHook{
		namespaceLocked: make(chan struct{}), resume: make(chan struct{}),
		updateRead: make(chan struct{}), updateAttempt: make(chan struct{}),
	}
	db.BunDB.AddQueryHook(hook)
	deleteResult := make(chan error, 1)
	go func() {
		deleteCtx := context.WithValue(ctx, moveOutDeleteHookContextKey{}, true)
		_, deleteErr := orgStore.Delete(deleteCtx, sourcePath)
		deleteResult <- deleteErr
	}()
	select {
	case <-hook.namespaceLocked:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "organization deletion did not lock the source namespace")
	}

	repo.Path = targetPath + "/repo"
	updateResult := make(chan error, 1)
	go func() {
		updateCtx := context.WithValue(ctx, moveOutUpdateHookContextKey{}, true)
		_, updateErr := repoStore.UpdateRepo(updateCtx, *repo)
		updateResult <- updateErr
	}()
	select {
	case <-hook.updateRead:
	case updateErr := <-updateResult:
		require.Error(t, updateErr)
		require.FailNow(t, "repository move finished before reaching namespace locking")
	case <-time.After(5 * time.Second):
		require.FailNow(t, "repository move did not read its current path")
	}
	select {
	case <-hook.updateAttempt:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "repository move did not attempt the source namespace lock")
	}
	close(hook.resume)

	require.NoError(t, <-deleteResult)
	updateErr := <-updateResult
	require.Error(t, updateErr)
	require.NotContains(t, strings.ToLower(updateErr.Error()), "deadlock")
	exists, err := db.Core.NewSelect().Model((*database.Repository)(nil)).Where("id = ?", repo.ID).Exists(ctx)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestRepoStore_MixedCaseCrossNamespaceMoveSerializesWithRootDeletion(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	creator := createOrganizationUnitMemberTestUser(t, ctx, db, "mixed-case-root-delete-user")
	unitStore := database.NewOrganizationUnitStoreWithDBAndJobClients(db, &testRepositoryDeletionJobClient{}, nil, tests.NewFixtureRebacTupleJobClient(db))
	repoStore := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	rootUUID := uuid.New()
	root, err := unitStore.CreateRoot(ctx, database.CreateRootOrganizationInput{
		Organization: &database.Organization{Name: "mixedCaseMiddle", UUID: rootUUID, UserID: creator.ID, IsRoot: true, IsHierarchical: true},
		Namespace:    &database.Namespace{Path: "mixedCaseMiddle", UUID: rootUUID.String()}, CreatorUserID: creator.ID,
	})
	require.NoError(t, err)
	source := createChild(t, ctx, unitStore, root, "alphaMixedCase", nil, 1)
	target := createChild(t, ctx, unitStore, root, "ZooMixedCase", nil, 2)
	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		Name: "repo", Path: source.Name + "/repo", GitPath: "models_" + source.Name + "/repo",
		RepositoryType: types.ModelRepo, UserID: creator.ID,
	})
	require.NoError(t, err)

	hook := &mixedCaseNamespaceOrderHook{
		firstPath: source.Name, blockedPath: source.Name, firstLocked: make(chan struct{}),
		deleteAttempt: make(chan struct{}), resume: make(chan struct{}),
	}
	db.BunDB.AddQueryHook(hook)
	repo.Path = target.Name + "/repo"
	updateResult := make(chan error, 1)
	go func() {
		updateCtx := context.WithValue(ctx, mixedCaseUpdateHookContextKey{}, true)
		_, updateErr := repoStore.UpdateRepo(updateCtx, *repo)
		updateResult <- updateErr
	}()
	select {
	case <-hook.firstLocked:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "repository move did not acquire its first canonical namespace lock")
	}

	deleteResult := make(chan error, 1)
	go func() {
		deleteCtx := context.WithValue(ctx, mixedCaseDeleteHookContextKey{}, true)
		_, deleteErr := unitStore.DeleteRoot(deleteCtx, database.DeleteRootOrganizationInput{OrganizationUUID: root.UUID.String()})
		deleteResult <- deleteErr
	}()
	select {
	case <-hook.deleteAttempt:
	case deleteErr := <-deleteResult:
		require.NoError(t, deleteErr)
		require.FailNow(t, "root deletion completed before contending on the first canonical namespace")
	case <-time.After(5 * time.Second):
		require.FailNow(t, "root deletion did not use the canonical namespace order")
	}
	close(hook.resume)

	require.NoError(t, <-updateResult)
	require.NoError(t, <-deleteResult)
	exists, err := db.Core.NewSelect().Model((*database.Repository)(nil)).Where("id = ?", repo.ID).Exists(ctx)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestRepoStore_UpdateRepoFileDownloads(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))

	repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path:    "u/bar",
		Name:    "repo1",
		UserID:  123,
		GitPath: "foos_u/bar",
	}))
	require.Nil(t, err)

	dt := time.Date(2022, 12, 11, 0, 0, 0, 0, time.UTC)
	dw := &database.RepositoryDownload{}
	// create
	err = store.UpdateRepoFileDownloads(ctx, repo, dt, 111)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(dw).Where("repository_id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 111, int(dw.ClickDownloadCount))
	err = db.Core.NewSelect().Model(repo).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 111, int(repo.DownloadCount))

	// update
	err = store.UpdateRepoFileDownloads(ctx, repo, dt, 5)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(dw).Where("repository_id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 116, int(dw.ClickDownloadCount))
	err = db.Core.NewSelect().Model(repo).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 116, int(repo.DownloadCount))

}

func TestRepoStore_UpdateRepoCloneDownloads(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))

	repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path:    "u/bar",
		Name:    "repo1",
		UserID:  123,
		GitPath: "foos_u/bar",
	}))
	require.Nil(t, err)

	dt := time.Date(2022, 12, 11, 0, 0, 0, 0, time.UTC)
	dw := &database.RepositoryDownload{}
	// create
	err = store.UpdateRepoCloneDownloads(ctx, repo, dt, 111)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(dw).Where("repository_id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 111, int(dw.CloneCount))
	err = db.Core.NewSelect().Model(repo).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 111, int(repo.DownloadCount))

	// update
	err = store.UpdateRepoCloneDownloads(ctx, repo, dt, 5)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(dw).Where("repository_id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	// clone count will be override, not add to previous
	require.Equal(t, 5, int(dw.CloneCount))
	err = db.Core.NewSelect().Model(repo).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 5, int(repo.DownloadCount))

}

func TestRepoStore_Tags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path:   "fixtures/repo1",
		Name:   "repo1",
		UserID: 123,
	}))
	require.Nil(t, err)

	tag := &database.Tag{
		Name:     "tg",
		Category: "foo",
	}
	_, err = db.Core.NewInsert().Model(tag).Exec(ctx, tag)
	require.Nil(t, err)
	tag2 := &database.Tag{
		Name:     "tg2",
		Category: "bar",
	}
	_, err = db.Core.NewInsert().Model(tag2).Exec(ctx, tag2)
	require.Nil(t, err)

	rtags := []database.RepositoryTag{
		{TagID: tag.ID, RepositoryID: repo.ID, Count: 1},
		{TagID: tag2.ID, RepositoryID: repo.ID, Count: 1},
	}
	err = store.BatchCreateRepoTags(ctx, rtags)
	require.Nil(t, err)

	tags, err := store.Tags(ctx, repo.ID)
	require.Nil(t, err)
	require.Equal(t, 2, len(tags))
	require.Equal(t, tags[0].Name, "tg")
	require.Equal(t, tags[1].Name, "tg2")

	tags, err = store.TagsWithCategory(ctx, repo.ID, "foo")
	require.Nil(t, err)
	require.Equal(t, 1, len(tags))
	require.Equal(t, tags[0].Name, "tg")

	ids, err := store.TagIDs(ctx, repo.ID, "foo")
	require.Nil(t, err)
	require.ElementsMatch(t, []int64{tag.ID}, ids)
}

func TestRepoStore_SetUpdateTimeByPath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name:           "repo1",
		UserID:         123,
		GitPath:        "models_u/bar",
		RepositoryType: types.ModelRepo,
		Path:           "u/bar",
	}))
	require.Nil(t, err)

	dt := time.Date(2022, 12, 6, 1, 2, 0, 0, time.UTC)
	err = store.SetUpdateTimeByPath(ctx, types.ModelRepo, "u", "bar", dt)
	require.Nil(t, err)

	err = db.Core.NewSelect().Model(repo).WherePK().Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, dt, repo.UpdatedAt)

}

func TestRepoStore_PublicToUserSimple(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	redisClient := tests.InitTestRedis()
	defer redisClient.Close()
	config := &config.Config{}
	config.Search.RepoSearchCacheTTL = 300
	config.Search.RepoSearchLimit = 10
	config.Database.Driver = "pg"
	config.Database.SearchConfiguration = "opencsgchinese"
	rs := database.NewRepoStoreWithCacheAndRebacTupleJobClient(config, db, cache.NewCacheWithClient(ctx, redisClient), tests.NewFixtureRebacTupleJobClient(db))
	userName := "user_name_" + uuid.NewString()
	repoName := "repo_name_" + uuid.NewString()
	repo, err := rs.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName, repoName),
		GitPath:        fmt.Sprintf("datasets_%s/%s", userName, repoName),
		Name:           repoName,
		DefaultBranch:  "main",
		Nickname:       "ww",
		Description:    "ww",
		Private:        false,
		RepositoryType: types.DatasetRepo,
	}))
	require.Empty(t, err)
	require.NotNil(t, repo)
	ds := database.NewDatasetStoreWithDB(db)
	_, err = ds.Create(ctx, database.Dataset{
		RepositoryID: repo.ID,
	})
	require.Empty(t, err)

	ts := database.NewTagStoreWithDB(db)
	var tags []*database.Tag
	tags = append(tags, &database.Tag{
		Name:     "tag_" + uuid.NewString(),
		Category: "evaluation",
		Group:    "",
		Scope:    types.DatasetTagScope,
		BuiltIn:  true,
		ShowName: "",
	})
	tags = append(tags, &database.Tag{
		Name:     "tag_" + uuid.NewString(),
		Category: "runtime_framework",
		Group:    "",
		Scope:    types.DatasetTagScope,
		BuiltIn:  true,
		ShowName: "",
	})
	t1, err := ts.FindOrCreate(ctx, *tags[0])
	require.Nil(t, err)
	t2, err := ts.FindOrCreate(ctx, *tags[1])
	require.Nil(t, err)
	err = ts.UpsertRepoTags(ctx, repo.ID, []int64{}, []int64{t1.ID, t2.ID})
	require.Nil(t, err)
	var tagsfilter []types.TagReq
	tagsfilter = append(tagsfilter, types.TagReq{
		Name:     tags[0].Name,
		Category: tags[0].Category,
	})
	filter := &types.RepoFilter{
		Tags: tagsfilter,
		Sort: "recently_update",
	}
	// case 1: one tag
	repos, _, err := rs.PublicToUserWithAccess(ctx, repo.RepositoryType, database.NewRepositoryAccessScope(database.RepositoryAccessAdmin, nil), filter, 20, 1)
	require.Nil(t, err)
	require.NotNil(t, repos)

	tagsfilter = append(tagsfilter, types.TagReq{
		Name:     tags[1].Name,
		Category: tags[1].Category,
	})
	filter = &types.RepoFilter{
		Tags: tagsfilter,
		Sort: "recently_update",
	}
	// case 2: two tag
	repos, _, err = rs.PublicToUserWithAccess(ctx, repo.RepositoryType, database.NewRepositoryAccessScope(database.RepositoryAccessAdmin, nil), filter, 20, 1)
	require.Nil(t, err)
	require.NotNil(t, repos)
}

func defineTestCases(hasZhparser bool) []repoSearchCase {
	commonCases := []repoSearchCase{
		{
			admin: false, repoType: types.CodeRepo,
			search:   "billionaire",
			expected: []string{"ChineseBlue", "ChineseMedicalBooksCollection"},
		},
		// test cache
		{
			admin: false, repoType: types.CodeRepo,
			search:   "billionaire",
			expected: []string{"ChineseBlue", "ChineseMedicalBooksCollection"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "stable diff",
			expected: []string{"Stable"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "Qwen2.5-Coder-32B-Instruct",
			expected: []string{"Qwen2.5"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "mason/chatglm3-6b-32k",
			expected: []string{"ChatGlm3"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "Qwen/Qwen2-1.5B",
			expected: []string{"Qwen2-1.5B", "Qwen2-1.5B-Instruct-GGUF"},
		},
	}

	if !hasZhparser {
		return commonCases
	}

	zhparserCases := []repoSearchCase{
		{
			admin: false, repoType: types.CodeRepo,
			search:   "中医经典",
			expected: []string{"ChineseMedicalBooksCollection"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "中医 经典",
			expected: []string{"ChineseMedicalBooksCollection"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "生物 医学",
			expected: []string{"ChineseBlue"},
		},
	}

	return append(zhparserCases, commonCases...)
}

// setupTestRepositories prepares searchable repositories with their namespace owners.
func setupTestRepositories(t *testing.T, ctx context.Context, store database.RepoStore, db *database.DB) ([]*database.Repository, error) {
	t.Helper()
	repos := []*database.Repository{
		{
			Name:           "ChineseMedicalBooksCollection",
			Path:           "billionaire/ChineseMedicalBooksCollection",
			Nickname:       "历代中医文献数据库",
			Description:    "历代中医文献数据库,内含约700本中医经典书籍。全部txt文件",
			UserID:         123,
			RepositoryType: types.CodeRepo,
			DownloadCount:  10,
			Private:        false,
			Tags:           []database.Tag{{Name: "foo"}},
		},
		{
			Name:           "ChineseBlue",
			Path:           "billionaire/ChineseBlue",
			Nickname:       "ChineseBlue中文生物医学文本",
			Description:    "ChinesseBLUE基准测试由不同的生物医学文本挖掘任务组成。这些任务涵盖了不同的文本类型(生物医学网络数据和临床记录)、数据集大小和难度级别，更重要的是，突出了常见的生物医学文本挖掘挑战。",
			UserID:         123,
			RepositoryType: types.CodeRepo,
			DownloadCount:  10,
			Private:        false,
		},
		{
			Name:           "AAIG_CUP",
			Path:           "MagicAI/AAIG_CUP",
			Nickname:       "电商推荐“抱大腿”攻击识别数据集",
			Description:    "随着互联网的发展，网购成为越来越多人的选择，平台流量竞争也越发激烈。为了保证平台的公平性，如何准确、高效地识别这类型的恶意流量攻击，实时过滤恶意的点击数据是推荐系统中迫切需要解决的问题。",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  19,
			Private:        false,
			Tags: []database.Tag{
				{Name: "relation-extraction"},
				{Name: "zero-shot"},
				{Name: "other"},
			},
		},
		{
			Name:           "Stable",
			Path:           "opencsg/stable-diffusion-3-medium",
			Nickname:       "opencsg",
			Description:    "great model",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  1,
			Private:        false,
		},
		{
			Name:           "Qwen2.5",
			Path:           "xzgan001/Qwen2.5-Coder-32B-Instruct",
			Nickname:       "Qwen2.5-Coder-32B-Instruct",
			Description:    "Qwen",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  1,
			Private:        false,
		},
		{
			Name:           "ChatGlm3",
			Path:           "mason/chatglm3-6b-32k",
			Nickname:       "chatglm3",
			Description:    "chatglm3 is a great model",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  1,
			Private:        false,
		},
		{
			Name:           "Qwen2-1.5B-Instruct-GGUF",
			Path:           "AIWizards/Qwen2-1.5B-Instruct-GGUF",
			Nickname:       "Qwen2-GGUF",
			Description:    "weighted/imatrix quants seem not to be available (by me) at this time. If they do not show up a week or so after the static ones, I have probably not planned for them. Feel free to request them by opening a Community Discussion.",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  1,
			Private:        false,
		},
		{
			Name:           "Qwen2-1.5B",
			Path:           "Qwen/Qwen2-1.5B",
			Nickname:       "Qwen2-1.5B",
			Description:    "qwen2 is a great model",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  1,
			Private:        false,
		},
	}

	for _, repo := range repos {
		repo.GitPath = repo.Path
		rn, err := store.CreateRepo(ctx, withTupleNamespace(t, db, *repo))
		if err != nil {
			return nil, err
		}
		code := database.Code{
			RepositoryID: rn.ID,
		}
		_, err = db.Core.NewInsert().Model(&code).Exec(ctx)
		if err != nil {
			return nil, err
		}
		for _, tag := range repo.Tags {
			_, err = db.Core.NewInsert().Model(&tag).Exec(ctx, &tag)
			if err != nil {
				return nil, err
			}
			rtags := []database.RepositoryTag{
				{TagID: tag.ID, RepositoryID: rn.ID, Count: 1},
			}
			err = store.BatchCreateRepoTags(ctx, rtags)
			if err != nil {
				return nil, err
			}
		}
	}
	return repos, nil
}

type repoSearchCase struct {
	admin    bool
	repoType types.RepositoryType
	source   string
	search   string
	tags     []types.TagReq
	sort     string
	expected []string
}

func TestRepoStore_PublicToUserSearch_Sqlite(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	redisClient := tests.InitTestRedis()
	defer redisClient.Close()

	config := &config.Config{}
	config.Search.RepoSearchCacheTTL = 300
	config.Search.RepoSearchLimit = 10
	config.Database.Driver = "sqlite"
	store := database.NewRepoStoreWithCacheAndRebacTupleJobClient(config, db, cache.NewCacheWithClient(ctx, redisClient), tests.NewFixtureRebacTupleJobClient(db))

	_, err := setupTestRepositories(t, ctx, store, db)
	require.Nil(t, err)
	cases := []repoSearchCase{
		{
			admin: false, repoType: types.CodeRepo,
			search:   "billionaire",
			expected: []string{"ChineseBlue", "ChineseMedicalBooksCollection"},
		},
		// test cache
		{
			admin: false, repoType: types.CodeRepo,
			search:   "billionaire",
			expected: []string{"ChineseBlue", "ChineseMedicalBooksCollection"},
		},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			scope := database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil)
			if c.admin {
				scope = database.NewRepositoryAccessScope(database.RepositoryAccessAdmin, nil)
			}
			rs, count, err := store.PublicToUserWithAccess(ctx, c.repoType, scope, &types.RepoFilter{
				Tags:   c.tags,
				Sort:   c.sort,
				Search: c.search,
				Source: c.source,
			}, 10, 1)
			require.Nil(t, err)
			names := []string{}
			for _, r := range rs {
				names = append(names, r.Name)
			}
			require.ElementsMatch(t, c.expected, names)
			require.Equal(t, len(c.expected), count)
		})
	}
}

func TestRepoStore_PublicToUserWithAccessFiltersPrivateRepositories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	config := &config.Config{}
	config.Database.Driver = "sqlite"
	config.Search.RepoSearchLimit = 20
	store := database.NewRepoStoreWithCacheAndRebacTupleJobClient(config, db, nil, tests.NewFixtureRebacTupleJobClient(db))

	_, err := setupTestRepositories(t, ctx, store, db)
	require.NoError(t, err)
	privateRepo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name:           "private-repo",
		Path:           "other/private-repo",
		GitPath:        "other/private-repo",
		Nickname:       "private-repo",
		UserID:         123,
		RepositoryType: types.CodeRepo,
		Private:        true,
	}))
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&database.Code{RepositoryID: privateRepo.ID}).Exec(ctx)
	require.NoError(t, err)

	scope := database.NewRepositoryAccessScope(database.RepositoryAccessReadable, []int64{privateRepo.ID})
	repos, _, err := store.PublicToUserWithAccess(ctx, types.CodeRepo, scope, &types.RepoFilter{Sort: "recently_update"}, 20, 1)
	require.NoError(t, err)
	var foundPrivate bool
	for _, repo := range repos {
		if repo.ID == privateRepo.ID {
			foundPrivate = true
		}
	}
	require.True(t, foundPrivate)

	publicOnly, _, err := store.PublicToUserWithAccess(ctx, types.CodeRepo,
		database.NewRepositoryAccessScope(database.RepositoryAccessReadable, nil),
		&types.RepoFilter{Sort: "recently_update"}, 20, 1)
	require.NoError(t, err)
	for _, repo := range publicOnly {
		require.NotEqual(t, privateRepo.ID, repo.ID)
	}
}

func TestRepoStore_PublicToUserSearch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	redisClient := tests.InitTestRedis()
	defer redisClient.Close()

	config := &config.Config{}
	config.Search.RepoSearchCacheTTL = 300
	config.Search.RepoSearchLimit = 10
	config.Database.Driver = "pg"
	config.Database.SearchConfiguration = "opencsgchinese"
	store := database.NewRepoStoreWithCacheAndRebacTupleJobClient(config, db, cache.NewCacheWithClient(ctx, redisClient), tests.NewFixtureRebacTupleJobClient(db))

	_, err := setupTestRepositories(t, ctx, store, db)
	require.Nil(t, err)
	hasZhparser, err := tests.CheckZhparser(ctx, db.BunDB, "pg")
	require.NoError(t, err)
	cases := defineTestCases(hasZhparser)

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			scope := database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil)
			if c.admin {
				scope = database.NewRepositoryAccessScope(database.RepositoryAccessAdmin, nil)
			}
			rs, count, err := store.PublicToUserWithAccess(ctx, c.repoType, scope, &types.RepoFilter{
				Tags:   c.tags,
				Sort:   c.sort,
				Search: c.search,
				Source: c.source,
			}, 10, 1)
			require.Nil(t, err)
			names := []string{}
			for _, r := range rs {
				names = append(names, r.Name)
			}
			require.ElementsMatch(t, c.expected, names)
			require.Equal(t, len(c.expected), count)
		})
	}
}

func TestRepoStore_PublicToUser(t *testing.T) {
	cases := []struct {
		admin    bool
		repoType types.RepositoryType
		source   string
		search   string
		tags     []types.TagReq
		sort     string
		expected []string
	}{
		{
			admin: false, repoType: types.CodeRepo,
			sort:     "recently_update",
			expected: []string{"rp6", "rp5", "rp4", "rp1"},
		},
		{
			admin: false, repoType: types.CodeRepo, source: string(types.HuggingfaceSource),
			expected: []string{},
		},
		{
			admin: true, repoType: types.CodeRepo,
			sort:     "recently_update",
			expected: []string{"rp6", "rp5", "rp4", "rp3", "rp2", "rp1"},
		},
		{
			admin: false, repoType: types.CodeRepo, search: "rp4",
			expected: []string{"rp4", "rp6"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			sort:     "most_download",
			expected: []string{"rp6", "rp5", "rp4", "rp1"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			sort:     "most_favorite",
			expected: []string{"rp6", "rp5", "rp4", "rp1"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			sort:     "most_star",
			expected: []string{"rp6", "rp5", "rp4", "rp1"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			sort:     "trending",
			expected: []string{"rp6", "rp5", "rp4", "rp1"},
		},
		{
			admin: false, repoType: types.CodeRepo, tags: []types.TagReq{{Name: "foo"}},
			expected: []string{"rp4"},
		},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			ctx := context.TODO()
			redisClient := tests.InitTestRedis()
			defer redisClient.Close()

			config := &config.Config{}
			config.Search.RepoSearchCacheTTL = 300
			config.Search.RepoSearchLimit = 10
			config.Database.Driver = "pg"
			config.Database.SearchConfiguration = "opencsgchinese"
			store := database.NewRepoStoreWithCacheAndRebacTupleJobClient(config, db, cache.NewCacheWithClient(ctx, redisClient), tests.NewFixtureRebacTupleJobClient(db))
			recomStore := database.NewRecomStoreWithDB(db)

			repos := []*database.Repository{
				{
					Name: "rp1", Path: "user123/rp1", UserID: 123, RepositoryType: types.CodeRepo,
					DownloadCount: 11,
				},
				{
					Name: "rp2", Path: "user123/rp2", UserID: 123, RepositoryType: types.CodeRepo,
					Private: true, Source: types.HuggingfaceSource,
					DownloadCount: 12,
				},
				{
					Name: "rp3", Path: "user456/rp3", UserID: 456, RepositoryType: types.CodeRepo,
					Private:       true,
					DownloadCount: 15,
				},
				{
					Name: "rp4", Path: "user456/rp4", UserID: 456, RepositoryType: types.CodeRepo,
					Private: false, Tags: []database.Tag{{Name: "foo"}},
					DownloadCount: 13,
				},
				{
					Name: "rp5", Path: "user789/rp5", UserID: 789, RepositoryType: types.CodeRepo,
					Private: false, Tags: []database.Tag{{Name: "bar"}},
					DownloadCount: 14,
				},
				{
					Name: "rp6", Path: "user789/rp6", UserID: 789, RepositoryType: types.CodeRepo,
					Private: false, Description: "rp4desc",
					DownloadCount: 16,
				},
			}

			fixedUpdatedAt := time.Now()
			for _, repo := range repos {
				repo.GitPath = repo.Path
				// Use the same update time to verify ID provides deterministic tie-breaking.
				repo.UpdatedAt = fixedUpdatedAt
				rn, err := store.CreateRepo(ctx, withTupleNamespace(t, db, *repo))
				require.Nil(t, err)

				// Create corresponding record in codes table for CodeRepo
				if repo.RepositoryType == types.CodeRepo {
					code := database.Code{
						RepositoryID: rn.ID,
					}
					_, err = db.Core.NewInsert().Model(&code).Exec(ctx)
					require.Nil(t, err)
				}

				for _, tag := range repo.Tags {
					_, err = db.Core.NewInsert().Model(&tag).Exec(ctx, &tag)
					require.Nil(t, err)
					rtags := []database.RepositoryTag{
						{TagID: tag.ID, RepositoryID: rn.ID, Count: 1},
					}
					err = store.BatchCreateRepoTags(ctx, rtags)
					require.Nil(t, err)
				}

				// Extract number from repo name (e.g., "rp4" -> 4) and multiply by 100
				if strings.HasPrefix(repo.Name, "rp") {
					if numStr := strings.TrimPrefix(repo.Name, "rp"); numStr != "" {
						if num, err := strconv.Atoi(numStr); err == nil {
							// insert recom repo score
							err := recomStore.UpsertScore(ctx, []*database.RecomRepoScore{
								{
									RepositoryID: rn.ID,
									WeightName:   database.RecomWeightTotal,
									Score:        float64(num * 100),
								},
							})
							require.Nil(t, err)
						}
					}
				}
			}

			scope := database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil)
			if c.admin {
				scope = database.NewRepositoryAccessScope(database.RepositoryAccessAdmin, nil)
			}
			rs, count, err := store.PublicToUserWithAccess(ctx, c.repoType, scope, &types.RepoFilter{
				Tags:   c.tags,
				Sort:   c.sort,
				Search: c.search,
				Source: c.source,
			}, 10, 1)
			require.Nil(t, err)
			names := []string{}
			for _, r := range rs {
				names = append(names, r.Name)
			}
			require.Equal(t, len(c.expected), count)
			require.ElementsMatch(t, c.expected, names)

		})
	}
}

func TestRepoStore_PublicToUserComplianceStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	modelStore := database.NewModelStoreWithDB(db)
	recomStore := database.NewRecomStoreWithDB(db)
	repositories := []database.Repository{
		{
			Name:                 "pending-model",
			Path:                 "compliance/pending-model",
			GitPath:              "models_compliance/pending-model",
			RepositoryType:       types.ModelRepo,
			ComplianceStatus:     types.ComplianceStatusPendingReview,
			CommercialPermission: types.CommercialPermissionCustomTerms,
		},
		{
			Name:                 "compliant-model",
			Path:                 "compliance/compliant-model",
			GitPath:              "models_compliance/compliant-model",
			RepositoryType:       types.ModelRepo,
			ComplianceStatus:     types.ComplianceStatusCompliant,
			CommercialPermission: types.CommercialPermissionAllowed,
		},
	}

	for index, repository := range repositories {
		repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, repository))
		require.NoError(t, err)
		_, err = modelStore.Create(ctx, database.Model{RepositoryID: repo.ID})
		require.NoError(t, err)
		err = recomStore.UpsertScore(ctx, []*database.RecomRepoScore{{
			RepositoryID: repo.ID,
			WeightName:   database.RecomWeightTotal,
			Score:        float64(index + 1),
		}})
		require.NoError(t, err)
	}

	status := types.ComplianceStatusPendingReview
	repos, count, err := store.PublicToUserWithAccess(
		ctx,
		types.ModelRepo,
		database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil),
		&types.RepoFilter{Sort: "trending", ComplianceStatus: &status},
		10,
		1,
	)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "pending-model", repos[0].Name)
}

func TestRepoStore_PublicToUserOwnerFilter(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	repos := []database.Repository{
		{Name: "match", Path: "owner/match", GitPath: "owner/match", UserID: 123, RepositoryType: types.CodeRepo},
		{Name: "other", Path: "other/match", GitPath: "other/match", UserID: 123, RepositoryType: types.CodeRepo},
		{Name: "underscore-match", Path: "own_er/match", GitPath: "own_er/match", UserID: 123, RepositoryType: types.CodeRepo},
		{Name: "underscore-other", Path: "ownerX/match", GitPath: "ownerX/match", UserID: 123, RepositoryType: types.CodeRepo},
	}
	for _, repo := range repos {
		created, err := store.CreateRepo(ctx, withTupleNamespace(t, db, repo))
		require.Nil(t, err)
		_, err = db.Core.NewInsert().Model(&database.Code{RepositoryID: created.ID}).Exec(ctx)
		require.Nil(t, err)
	}

	rs, count, err := store.PublicToUserWithAccess(ctx, types.CodeRepo, database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil), &types.RepoFilter{
		Owner: "owner",
		Sort:  "recently_update",
	}, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	require.Len(t, rs, 1)
	require.Equal(t, "owner/match", rs[0].Path)

	rs, count, err = store.PublicToUserWithAccess(ctx, types.CodeRepo, database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil), &types.RepoFilter{
		Owner: "own_er",
		Sort:  "recently_update",
	}, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	require.Len(t, rs, 1)
	require.Equal(t, "own_er/match", rs[0].Path)
}

func TestRepoStore_PublicToUserRangeFilters(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	modelStore := database.NewModelStoreWithDB(db)
	datasetStore := database.NewDatasetStoreWithDB(db)

	smallModel, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name:           "small-model",
		Path:           "test/small-model",
		GitPath:        "models_test/small-model",
		UserID:         123,
		RepositoryType: types.ModelRepo,
	}))
	require.Nil(t, err)
	_, err = modelStore.Create(ctx, database.Model{RepositoryID: smallModel.ID})
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.Metadata{
		RepositoryID:     smallModel.ID,
		ModelParams:      1.5,
		ModelParamsValid: true,
	}).Exec(ctx)
	require.Nil(t, err)

	largeModel, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name:           "large-model",
		Path:           "test/large-model",
		GitPath:        "models_test/large-model",
		UserID:         123,
		RepositoryType: types.ModelRepo,
	}))
	require.Nil(t, err)
	_, err = modelStore.Create(ctx, database.Model{RepositoryID: largeModel.ID})
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.Metadata{
		RepositoryID:     largeModel.ID,
		ModelParams:      7,
		ModelParamsValid: true,
	}).Exec(ctx)
	require.Nil(t, err)

	validZeroModel, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name:           "valid-zero-model",
		Path:           "test/valid-zero-model",
		GitPath:        "models_test/valid-zero-model",
		UserID:         123,
		RepositoryType: types.ModelRepo,
	}))
	require.Nil(t, err)
	_, err = modelStore.Create(ctx, database.Model{RepositoryID: validZeroModel.ID})
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.Metadata{
		RepositoryID:     validZeroModel.ID,
		ModelParams:      0,
		ModelParamsValid: true,
	}).Exec(ctx)
	require.Nil(t, err)

	invalidZeroModel, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name:           "invalid-zero-model",
		Path:           "test/invalid-zero-model",
		GitPath:        "models_test/invalid-zero-model",
		UserID:         123,
		RepositoryType: types.ModelRepo,
	}))
	require.Nil(t, err)
	_, err = modelStore.Create(ctx, database.Model{RepositoryID: invalidZeroModel.ID})
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.Metadata{
		RepositoryID:     invalidZeroModel.ID,
		ModelParams:      0,
		ModelParamsValid: false,
	}).Exec(ctx)
	require.Nil(t, err)

	noMetadataModel, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name:           "no-metadata-model",
		Path:           "test/no-metadata-model",
		GitPath:        "models_test/no-metadata-model",
		UserID:         123,
		RepositoryType: types.ModelRepo,
	}))
	require.Nil(t, err)
	_, err = modelStore.Create(ctx, database.Model{RepositoryID: noMetadataModel.ID})
	require.Nil(t, err)

	zero := 0.0
	repos, count, err := store.PublicToUserWithAccess(ctx, types.ModelRepo, database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil), &types.RepoFilter{
		Sort:           "recently_update",
		ModelParamsMin: &zero,
		ModelParamsMax: &zero,
	}, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	require.Len(t, repos, 1)
	require.Equal(t, "valid-zero-model", repos[0].Name)

	modelParamsMin := 2.0
	modelParamsMax := 8.0
	repos, count, err = store.PublicToUserWithAccess(ctx, types.ModelRepo, database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil), &types.RepoFilter{
		Sort:           "recently_update",
		ModelParamsMin: &modelParamsMin,
		ModelParamsMax: &modelParamsMax,
	}, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "large-model", repos[0].Name)

	smallDataset, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name:           "small-dataset",
		Path:           "test/small-dataset",
		GitPath:        "datasets_test/small-dataset",
		UserID:         123,
		RepositoryType: types.DatasetRepo,
		DefaultBranch:  "main",
	}))
	require.Nil(t, err)
	_, err = datasetStore.Create(ctx, database.Dataset{RepositoryID: smallDataset.ID})
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.RepositoryStatistics{
		RepositoryID: smallDataset.ID,
		Branch:       "main",
		TotalSize:    100,
	}).Exec(ctx)
	require.Nil(t, err)

	largeDataset, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name:           "large-dataset",
		Path:           "test/large-dataset",
		GitPath:        "datasets_test/large-dataset",
		UserID:         123,
		RepositoryType: types.DatasetRepo,
		DefaultBranch:  "main",
	}))
	require.Nil(t, err)
	_, err = datasetStore.Create(ctx, database.Dataset{RepositoryID: largeDataset.ID})
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.RepositoryStatistics{
		RepositoryID: largeDataset.ID,
		Branch:       "main",
		TotalSize:    12_032,
	}).Exec(ctx)
	require.Nil(t, err)

	nonDefaultBranchDataset, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name:           "non-default-branch-dataset",
		Path:           "test/non-default-branch-dataset",
		GitPath:        "datasets_test/non-default-branch-dataset",
		UserID:         123,
		RepositoryType: types.DatasetRepo,
		DefaultBranch:  "main",
	}))
	require.Nil(t, err)
	_, err = datasetStore.Create(ctx, database.Dataset{RepositoryID: nonDefaultBranchDataset.ID})
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.RepositoryStatistics{
		RepositoryID: nonDefaultBranchDataset.ID,
		Branch:       "dev",
		TotalSize:    12_032,
	}).Exec(ctx)
	require.Nil(t, err)

	repoSizeMin := int64(1000)
	repoSizeMax := int64(20000)
	repos, count, err = store.PublicToUserWithAccess(ctx, types.DatasetRepo, database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil), &types.RepoFilter{
		Sort:        "recently_update",
		RepoSizeMin: &repoSizeMin,
		RepoSizeMax: &repoSizeMax,
	}, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "large-dataset", repos[0].Name)
}

func TestRepoStore_IsMirrorRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	rn, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath:        "codes_ns/n",
		RepositoryType: types.CodeRepo,
		Path:           "ns/n",
	}))
	require.Nil(t, err)
	mi := &database.Mirror{RepositoryID: rn.ID}
	_, err = db.Core.NewInsert().Model(mi).Exec(ctx)
	require.Nil(t, err)
	m, err := store.IsMirrorRepo(ctx, types.CodeRepo, "ns", "n")
	require.Nil(t, err)
	require.True(t, m)
	m, err = store.IsMirrorRepo(ctx, types.CodeRepo, "ns", "n2")
	require.NotNil(t, err)
	require.False(t, m)

}

func TestRepoStore_WithMirror(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	rn, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path:    "ns/n",
		GitPath: "codes_ns/n",
	}))
	require.Nil(t, err)
	mi := &database.Mirror{RepositoryID: rn.ID}
	_, err = db.Core.NewInsert().Model(mi).Exec(ctx)
	require.Nil(t, err)
	_, err = store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath: "codes_ns2/n",
		Path:    "fixtures/zzz",
	}))
	require.Nil(t, err)
	rs, count, err := store.WithMirror(ctx, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "codes_ns/n", rs[0].GitPath)

}

func TestRepoStore_ListRepoByDeployType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	rn1, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		ID:             1,
		UserID:         123,
		Path:           "ns/n",
		GitPath:        "models_ns/n",
		RepositoryType: types.ModelRepo,
		Private:        true,
	}))
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.Metadata{
		RepositoryID: rn1.ID,
		Architecture: "Qwen2ForCausalLM",
		ClassName:    "qwen2",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.Metadata{
		RepositoryID: 2,
		Architecture: "Qwen3ForCausalLM",
		ClassName:    "qwen3",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.Metadata{
		RepositoryID: 3,
		Architecture: "Qwen4ForCausalLM",
		ClassName:    "qwen4",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RuntimeFramework{
		ID:          1,
		FrameName:   "vllm",
		Type:        1,
		ModelFormat: "safetensors",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RuntimeFramework{
		ID:          2,
		FrameName:   "llama.cpp",
		Type:        1,
		ModelFormat: "gguf",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RuntimeArchitecture{
		ArchitectureName:   "Qwen2ForCausalLM",
		RuntimeFrameworkID: 1,
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RuntimeArchitecture{
		ArchitectureName:   "qwen3",
		RuntimeFrameworkID: 1,
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		ID:             2,
		UserID:         123,
		Path:           "ns/n2",
		GitPath:        "models_ns/n2",
		RepositoryType: types.ModelRepo,
		Private:        false,
	}))
	require.Nil(t, err)
	_, err = store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		ID:             3,
		UserID:         123,
		Path:           "ns/n3",
		GitPath:        "models_ns/n3",
		RepositoryType: types.ModelRepo,
		Private:        false,
	}))
	require.Nil(t, err)
	rs, count, err := store.ListRepoByDeployType(ctx, types.ModelRepo, 123, "", "", 1, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, "ns/n", rs[0].Path)

}

func TestRepoStore_CleanRelationsByRepoID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	rn, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath:        "codes_ns/n",
		Path:           "ns/n",
		RepositoryType: types.CodeRepo,
	}))
	require.Nil(t, err)

	rn1, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath:        "codes_ns/n1",
		Path:           "ns/n1",
		RepositoryType: types.CodeRepo,
	}))
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RepositoriesRuntimeFramework{
		RepoID: rn.ID,
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.UserLike{
		RepoID: rn.ID,
	}).Exec(ctx)
	require.Nil(t, err)
	count, err := db.Core.NewSelect().Model(&database.RepositoriesRuntimeFramework{}).Where("repo_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	count, err = db.Core.NewSelect().Model(&database.UserLike{}).Where("repo_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	var mirror database.Mirror
	err = db.Core.NewInsert().Model(&database.Mirror{RepositoryID: rn.ID}).Scan(ctx, &mirror)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.MirrorTask{MirrorID: mirror.ID}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.LfsMetaObject{RepositoryID: rn.ID, Oid: "oid"}).Exec(ctx)
	require.Nil(t, err)

	count, err = db.Core.NewSelect().Model(&database.Mirror{}).Where("repository_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	count, err = db.Core.NewSelect().Model(&database.MirrorTask{}).Where("mirror_id=?", mirror.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	count, err = db.Core.NewSelect().Model(&database.LfsMetaObject{}).Where("repository_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	var mirror1 database.Mirror
	err = db.Core.NewInsert().Model(&database.Mirror{RepositoryID: rn1.ID}).Scan(ctx, &mirror1)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.MirrorTask{MirrorID: mirror1.ID}).Exec(ctx)
	require.Nil(t, err)

	count, err = db.Core.NewSelect().Model(&database.Mirror{}).Where("repository_id=?", rn1.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	count, err = db.Core.NewSelect().Model(&database.MirrorTask{}).Where("mirror_id=?", mirror1.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	err = store.CleanRelationsByRepoID(ctx, rn.ID)
	require.Nil(t, err)

	count, err = db.Core.NewSelect().Model(&database.LfsMetaObject{}).Where("repository_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)

	count, err = db.Core.NewSelect().Model(&database.Mirror{}).Where("repository_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)

	count, err = db.Core.NewSelect().Model(&database.MirrorTask{}).Where("mirror_id=?", mirror.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)

	count, err = db.Core.NewSelect().Model(&database.Mirror{}).Where("repository_id=?", rn1.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	count, err = db.Core.NewSelect().Model(&database.MirrorTask{}).Where("mirror_id=?", mirror1.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	count, err = db.Core.NewSelect().Model(&database.RepositoriesRuntimeFramework{}).Where("repo_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)
	count, err = db.Core.NewSelect().Model(&database.UserLike{}).Where("repo_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)

}

func TestRepoStore_DeleteAllFiles(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)

	_, err := db.Core.NewInsert().Model(&database.File{RepositoryID: 123}).Exec(ctx)
	require.Nil(t, err)
	err = store.DeleteAllFiles(ctx, 123)
	require.Nil(t, err)
	count, err := db.Core.NewSelect().Model(&database.File{}).Where("repository_id = ?", 123).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)
}

func TestRepoStore_DeleteAllTags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)

	_, err := db.Core.NewInsert().Model(&database.RepositoryTag{RepositoryID: 123}).Exec(ctx)
	require.Nil(t, err)
	err = store.DeleteAllTags(ctx, 123)
	require.Nil(t, err)
	count, err := db.Core.NewSelect().Model(&database.RepositoryTag{}).Where("repository_id = ?", 123).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)
}

// TestRepoStore_UpdateLicenseByTag preserves missing-tag behavior and model/dataset compliance classification.
func TestRepoStore_UpdateLicenseByTag(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	tag := database.Tag{
		Category: "license",
		Name:     "MIT",
	}
	_, err := db.Core.NewInsert().Model(&tag).Exec(ctx)
	require.NoError(t, err)
	for _, repoType := range []types.RepositoryType{types.CodeRepo, types.ModelRepo, types.DatasetRepo} {
		for _, withTag := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/with_tag_%t", repoType, withTag), func(t *testing.T) {
				name := fmt.Sprintf("%s-%t", repoType, withTag)
				repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
					Path: "license-tags/" + name, GitPath: fmt.Sprintf("%ss_license-tags/%s", repoType, name),
					Name: name, RepositoryType: repoType, License: "foo", Private: true,
					PendingPublish: repoType == types.ModelRepo,
				}))
				require.NoError(t, err)
				if withTag {
					require.NoError(t, store.BatchCreateRepoTags(ctx, []database.RepositoryTag{{TagID: tag.ID, RepositoryID: repo.ID}}))
				}
				want, err := store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				if withTag {
					want.License = "MIT"
					if types.SupportsLicenseCompliance(repoType) {
						want.ComplianceStatus = types.ComplianceStatusCompliant
						want.CommercialPermission = types.CommercialPermissionAllowed
					}
				} else if types.SupportsLicenseCompliance(repoType) {
					want.License = ""
				}
				require.NoError(t, store.UpdateLicenseByTag(ctx, repo.ID))
				got, err := store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				require.Equal(t, want, got)
			})
		}
	}
}

// licenseTagUpdateContextKey scopes the paused tag query to one background writer.
type licenseTagUpdateContextKey struct{}

// licenseTagUpdateHook pauses license lookup after the repository snapshot has been read.
type licenseTagUpdateHook struct {
	ready  chan struct{}
	resume chan struct{}
}

// BeforeQuery lets a real settings transaction commit before the stale license writer continues.
func (h *licenseTagUpdateHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if ctx.Value(licenseTagUpdateContextKey{}) == h && strings.Contains(event.Query, `FROM "tags" AS "tag"`) {
		close(h.ready)
		select {
		case <-h.resume:
		case <-ctx.Done():
		}
	}
	return ctx
}

// AfterQuery completes the Bun query hook contract without changing query results.
func (h *licenseTagUpdateHook) AfterQuery(context.Context, *bun.QueryEvent) {}

// TestRepoStore_UpdateLicenseByTagPreservesConcurrentSettings verifies a stale
// license writer changes only the license after a user's visibility update commits.
func TestRepoStore_UpdateLicenseByTagPreservesConcurrentSettings(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	for _, repoType := range []types.RepositoryType{
		types.CodeRepo, types.PromptRepo, types.MCPServerRepo,
		types.SkillRepo, types.SpaceRepo, types.TemplateRepo,
	} {
		tag := database.Tag{Category: "license", Name: "MIT", Scope: types.TagScope(repoType)}
		_, err := db.Core.NewInsert().Model(&tag).Exec(context.Background())
		require.NoError(t, err)
		for _, private := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/private_%t", repoType, private), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				name := fmt.Sprintf("%s-%t", repoType, private)
				repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
					Path: "license-race/" + name, GitPath: fmt.Sprintf("%ss_license-race/%s", repoType, name),
					Name: name, RepositoryType: repoType, Private: !private, License: "old-license",
					SyncStatus: types.SyncStatusPending,
				}))
				require.NoError(t, err)
				require.NoError(t, store.BatchCreateRepoTags(ctx, []database.RepositoryTag{{TagID: tag.ID, RepositoryID: repo.ID}}))
				hook := &licenseTagUpdateHook{ready: make(chan struct{}), resume: make(chan struct{})}
				db.BunDB.AddQueryHook(hook)
				resume := sync.OnceFunc(func() { close(hook.resume) })
				defer resume()
				finished := make(chan error, 1)
				go func() {
					finished <- store.UpdateLicenseByTag(context.WithValue(ctx, licenseTagUpdateContextKey{}, hook), repo.ID)
				}()
				select {
				case <-hook.ready:
				case <-ctx.Done():
					t.Fatal("license writer did not reach the lookup after reading its repository snapshot")
				}
				configured := *repo
				configured.Private = private
				configured.Description = "user description"
				configured.Nickname = "user nickname"
				configured.SyncStatus = types.SyncStatusCompleted
				_, err = store.UpdateRepo(ctx, configured)
				require.NoError(t, err)
				want, err := store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				require.Equal(t, private, want.Private)
				want.License = "MIT"
				resume()
				select {
				case err := <-finished:
					require.NoError(t, err)
				case <-ctx.Done():
					t.Fatal("license writer did not finish after the settings update committed")
				}
				got, err := store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				require.Equal(t, want, got, "only the license may change after the settings update")
			})
		}
	}
}

func TestRepoStore_GetRepoRuntimeByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path:           "codes_ns/n",
		License:        "foo",
		RepositoryType: types.ModelRepo,
	}))
	require.Nil(t, err)

	rf := &database.RepositoriesRuntimeFramework{
		RepoID:             repo.ID,
		RuntimeFrameworkID: 999,
	}
	_, err = db.Core.NewInsert().Model(rf).Exec(ctx, rf)
	require.Nil(t, err)

	rs, err := store.GetRepoWithRuntimeByID(ctx, rf.RuntimeFrameworkID, []string{"codes_ns/n"})
	require.Nil(t, err)
	require.Equal(t, 1, len(rs))

	rs, err = store.GetRepoWithoutRuntimeByID(ctx, rf.RuntimeFrameworkID, []string{"codes_ns/n"}, 1000, 0)
	require.Nil(t, err)
	require.Equal(t, 0, len(rs))

}

func TestRepoStore_BatchMethods(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))

	repos := []*database.Repository{
		{
			Name: "rp1", Path: "fixtures/rp1", UserID: 123, RepositoryType: types.CodeRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
			Source:               types.HuggingfaceSource,
		},
		{
			Name: "rp2", Path: "fixtures/rp2", UserID: 123, RepositoryType: types.CodeRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
			Source:               types.HuggingfaceSource,
		},
		{
			Name: "rp3", Path: "fixtures/rp3", UserID: 123, RepositoryType: types.CodeRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
			Source:               types.HuggingfaceSource,
		},
		{
			Name: "rp4", Path: "fixtures/rp4", UserID: 456, RepositoryType: types.CodeRepo,
			SensitiveCheckStatus: types.SensitiveCheckPass,
			Source:               types.HuggingfaceSource,
		},
		{
			Name: "rp5", Path: "fixtures/rp5", UserID: 456, RepositoryType: types.DatasetRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
		},
	}

	names := func(rs []database.Repository) []string {
		names := []string{}
		for _, r := range rs {
			names = append(names, r.Name)
		}
		return names
	}

	rids := []int64{}
	for _, repo := range repos {
		repo.GitPath = repo.Path
		rn, err := store.CreateRepo(ctx, withTupleNamespace(t, db, *repo))
		require.Nil(t, err)
		rids = append(rids, rn.ID)
	}
	// Test with pending filter
	pendingStatus := types.SensitiveCheckPending
	pendingFilter := &types.BatchGetFilter{
		RepoType:             types.CodeRepo,
		SensitiveCheckStatus: &pendingStatus,
	}
	rs, err := store.BatchGet(ctx, 0, 10, pendingFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 3)
	require.ElementsMatch(t, []string{"rp1", "rp2", "rp3"}, names(rs))

	rs, err = store.BatchGet(ctx, rids[1], 10, pendingFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.ElementsMatch(t, []string{"rp3"}, names(rs))

	rs, err = store.BatchGet(ctx, 0, 1, pendingFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.ElementsMatch(t, []string{"rp1"}, names(rs))

	// Test with pass filter (should return CodeRepo with Pass status)
	passStatus := types.SensitiveCheckPass
	passFilter := &types.BatchGetFilter{
		RepoType:             types.CodeRepo,
		SensitiveCheckStatus: &passStatus,
	}
	rs, err = store.BatchGet(ctx, 0, 10, passFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1) // Should return rp4 which is CodeRepo with Pass status
	require.ElementsMatch(t, []string{"rp4"}, names(rs))

	// Test with different status for DatasetRepo
	pendingStatus2 := types.SensitiveCheckPending
	datasetFilter := &types.BatchGetFilter{
		RepoType:             types.DatasetRepo,
		SensitiveCheckStatus: &pendingStatus2,
	}
	rs, err = store.BatchGet(ctx, 0, 10, datasetFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.ElementsMatch(t, []string{"rp5"}, names(rs))

	// Test with nil filter (should return all repos)
	rs, err = store.BatchGet(ctx, 0, 10, nil)
	require.Nil(t, err)
	require.Equal(t, len(rs), 5) // Should return all 5 repos
	require.ElementsMatch(t, []string{"rp1", "rp2", "rp3", "rp4", "rp5"}, names(rs))

	// Test descending keyset pagination. The cursor is the last (smallest) ID
	// returned by the previous page.
	descendingFilter := &types.BatchGetFilter{OrderByIDDesc: true}
	rs, err = store.BatchGet(ctx, 0, 2, descendingFilter)
	require.Nil(t, err)
	require.Equal(t, []string{"rp5", "rp4"}, names(rs))

	rs, err = store.BatchGet(ctx, rids[3], 2, descendingFilter)
	require.Nil(t, err)
	require.Equal(t, []string{"rp3", "rp2"}, names(rs))

	// Test with empty filter (should return all repos)
	emptyFilter := &types.BatchGetFilter{}
	rs, err = store.BatchGet(ctx, 0, 10, emptyFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 5) // Should return all 5 repos
	require.ElementsMatch(t, []string{"rp1", "rp2", "rp3", "rp4", "rp5"}, names(rs))

	// Test with only RepoType filter
	repoTypeOnlyFilter := &types.BatchGetFilter{
		RepoType: types.CodeRepo,
	}
	rs, err = store.BatchGet(ctx, 0, 10, repoTypeOnlyFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 4) // Should return 4 CodeRepo repos
	require.ElementsMatch(t, []string{"rp1", "rp2", "rp3", "rp4"}, names(rs))

	// Test with only SensitiveCheckStatus filter
	pendingStatus3 := types.SensitiveCheckPending
	statusOnlyFilter := &types.BatchGetFilter{
		SensitiveCheckStatus: &pendingStatus3,
	}
	rs, err = store.BatchGet(ctx, 0, 10, statusOnlyFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 4) // Should return 4 repos with pending status
	require.ElementsMatch(t, []string{"rp1", "rp2", "rp3", "rp5"}, names(rs))

	// Test with SensitiveCheckStatus = 0 (Pending) to ensure 0 is treated as a valid filter value
	pendingStatusZero := types.SensitiveCheckStatus(0) // Explicitly set to 0 (Pending)
	statusZeroFilter := &types.BatchGetFilter{
		SensitiveCheckStatus: &pendingStatusZero,
	}
	rs, err = store.BatchGet(ctx, 0, 10, statusZeroFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 4) // Should return 4 repos with pending status (0)
	require.ElementsMatch(t, []string{"rp1", "rp2", "rp3", "rp5"}, names(rs))

	rs, err = store.FindWithBatch(ctx, 2, 1)
	require.Nil(t, err)
	require.Equal(t, len(rs), 2)
	require.ElementsMatch(t, []string{"rp3", "rp2"}, names(rs))

	rs, err = store.FindWithBatch(ctx, 2, 0, types.DatasetRepo)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.ElementsMatch(t, []string{"rp5"}, names(rs))

	rs, err = store.ByUser(ctx, 123, 100, 0)
	require.Nil(t, err)
	require.Equal(t, len(rs), 3)
	require.ElementsMatch(t, []string{"rp1", "rp2", "rp3"}, names(rs))

	rs, err = store.FindByRepoSourceWithBatch(ctx, types.HuggingfaceSource, 2, 1)
	require.Nil(t, err)
	require.Equal(t, len(rs), 2)
	require.ElementsMatch(t, []string{"rp2", "rp1"}, names(rs))
}

func TestRepoStore_FindMirrorReposByUserAndSource(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	repoStore := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	userStore := database.NewUserStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	mirrorStore := database.NewMirrorStoreWithDB(db)
	mirrorSouceStore := database.NewMirrorSourceStoreWithDB(db)

	namespace := &database.Namespace{
		Path: "test",
	}
	user := &database.User{
		Username: "test",
	}
	err := userStore.Create(ctx, user, namespace)
	require.Nil(t, err)

	repo := database.Repository{
		Path:           "test/test",
		GitPath:        "test/test",
		Name:           "test",
		Nickname:       "test",
		Private:        false,
		DefaultBranch:  "main",
		RepositoryType: types.CodeRepo,
		UserID:         user.ID,
	}
	repoRes, err := repoStore.CreateRepo(ctx, repo)
	require.Nil(t, err)

	mirrorSource := &database.MirrorSource{
		SourceName: "gitlab",
	}
	_, err = mirrorSouceStore.Create(ctx, mirrorSource)
	require.Nil(t, err)

	mirror := &database.Mirror{
		RepositoryID:   repoRes.ID,
		MirrorSourceID: mirrorSource.ID,
	}
	_, err = mirrorStore.Create(ctx, mirror)
	require.Nil(t, err)

	repos, err := repoStore.FindMirrorReposByUserAndSource(ctx, user.ID, "gitlab", 10, 1)
	require.Nil(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, repoRes.ID, repos[0].ID)
}

func TestRepoStore_UpdateSourcePath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	repoStore := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	userStore := database.NewUserStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))

	namespace := &database.Namespace{
		Path: "test",
	}
	user := &database.User{
		Username: "test",
	}
	err := userStore.Create(ctx, user, namespace)
	require.Nil(t, err)

	repo := database.Repository{
		Path:           "test/test",
		GitPath:        "test/test",
		Name:           "test",
		Nickname:       "test",
		Private:        false,
		DefaultBranch:  "main",
		RepositoryType: types.CodeRepo,
		UserID:         user.ID,
	}
	repoRes, err := repoStore.CreateRepo(ctx, repo)
	require.Nil(t, err)

	err = repoStore.UpdateSourcePath(ctx, repoRes.ID, "abc/def", "opencsg")
	require.Nil(t, err)
	err = repoStore.UpdateSourcePath(ctx, repoRes.ID, "aaa/bbb", "huggingface")
	require.Nil(t, err)
	err = repoStore.UpdateSourcePath(ctx, repoRes.ID, "ccc/ddd", "modelscope")
	require.Nil(t, err)
	repoFinal, err := repoStore.FindById(ctx, repoRes.ID)
	require.Nil(t, err)
	require.Equal(t, repoFinal.CSGPath, "abc/def")
	require.Equal(t, repoFinal.HFPath, "aaa/bbb")
	require.Equal(t, repoFinal.MSPath, "ccc/ddd")
}

func TestRepoStore_FindMirrorReposWithBatch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	rn, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path:    "ns/n",
		GitPath: "codes_ns/n",
	}))
	require.Nil(t, err)
	mi := &database.Mirror{RepositoryID: rn.ID}
	_, err = db.Core.NewInsert().Model(mi).Exec(ctx)
	require.Nil(t, err)
	_, err = store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath: "codes_ns2/n",
		Path:    "fixtures/zzz",
	}))
	require.Nil(t, err)
	rs, err := store.FindMirrorReposWithBatch(ctx, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(rs))
	require.Equal(t, "codes_ns/n", rs[0].GitPath)
}

func TestRepoStore_BulkUpdateSourcePath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	var repos []*database.Repository
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	rn, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path:    "ns/n",
		GitPath: "codes_ns/n",
	}))
	require.Nil(t, err)

	rn2, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath: "codes_ns2/n",
		Path:    "fixtures/zzz",
	}))
	require.Nil(t, err)

	rn.CSGPath = "abc/def"
	rn2.MSPath = "aaa/bbb"

	repos = append(repos, rn, rn2)
	err = store.BulkUpdateSourcePath(ctx, repos)
	require.Nil(t, err)

	rn, err = store.FindById(ctx, rn.ID)
	require.Nil(t, err)
	require.Equal(t, "abc/def", rn.CSGPath)

	rn2, err = store.FindById(ctx, rn2.ID)
	require.Nil(t, err)
	require.Equal(t, "aaa/bbb", rn2.MSPath)
}

func TestRepoStore_FindByMirrorSourceURL(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	sourceUrl := "https://github.com/codes_ns/n"
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	rn, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path:    "ns/n",
		GitPath: "codes_ns/n",
	}))

	require.Nil(t, err)

	mirrorStore := database.NewMirrorStoreWithDB(db)
	_, err = mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:    sourceUrl,
		RepositoryID: rn.ID,
	})

	require.Nil(t, err)

	rn, err = store.FindByMirrorSourceURL(ctx, sourceUrl)
	require.Nil(t, err)
	require.Equal(t, "codes_ns/n", rn.GitPath)
}

func TestRepoStore_RefreshLFSObjectsSize(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	rn, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path:    "ns/n",
		GitPath: "codes_ns/n",
	}))

	require.Nil(t, err)

	lfsStore := database.NewLfsMetaObjectStoreWithDB(db)
	err = lfsStore.BulkUpdateOrCreate(ctx, rn.ID, []database.LfsMetaObject{
		{
			RepositoryID: rn.ID,
			Size:         12,
			Oid:          "123",
		},
		{
			RepositoryID: rn.ID,
			Size:         10,
			Oid:          "321",
		},
	})

	require.Nil(t, err)

	err = store.RefreshLFSObjectsSize(ctx, rn.ID)
	require.Nil(t, err)

	repo, err := store.FindById(ctx, rn.ID)
	require.Nil(t, err)
	require.Equal(t, int64(22), repo.LFSObjectsSize)
}

// TestRepoStore_FindMirrorFinishedPrivateModelRepo selects only explicit publication intents.
func TestRepoStore_FindMirrorFinishedPrivateModelRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDB(db)
	for i, status := range []types.SensitiveCheckStatus{types.SensitiveCheckPass, types.SensitiveCheckSkip, types.SensitiveCheckFail} {
		repo, _ := newPublicationFixture(t, db, fmt.Sprintf("candidate-%d", i), true, status)
		require.True(t, repo.Private)
	}
	_, _ = newPublicationFixture(t, db, "user-private", false, types.SensitiveCheckPass)
	repos, err := store.FindMirrorFinishedPrivateModelRepo(ctx)
	require.NoError(t, err)
	require.Len(t, repos, 2)
	for _, repo := range repos {
		require.True(t, repo.PendingPublish)
	}
}

func TestRepoStore_BatchUpdate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	r, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath:              "codes_ns/n",
		Path:                 "ns/n",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckPass,
		Private:              true,
	}))

	require.Nil(t, err)

	r1, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath:              "codes_ns/n1",
		Path:                 "ns/n1",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckPass,
		Private:              true,
	}))
	require.Nil(t, err)

	r2, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath:              "codes_ns/n2",
		Path:                 "ns/n2",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckFail,
		Private:              true,
	}))
	require.Nil(t, err)

	r.Private = false
	r1.Private = false
	r2.Private = false

	err = store.BatchUpdate(ctx, []*database.Repository{r, r1, r2})
	require.Nil(t, err)

	repos, err := store.FindByIds(ctx, []int64{r.ID, r1.ID, r2.ID})
	require.Nil(t, err)

	require.Equal(t, 3, len(repos))
	require.Equal(t, false, repos[0].Private)
	require.Equal(t, false, repos[1].Private)
	require.Equal(t, false, repos[2].Private)
}

func TestRepoStore_FindByRepoTypeAndPaths(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	_, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath:              "codes_ns/n",
		Path:                 "ns/n",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckPass,
		Private:              true,
	}))

	require.Nil(t, err)

	_, err = store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath:              "codes_ns/n1",
		Path:                 "ns/n1",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckPass,
		Private:              true,
	}))
	require.Nil(t, err)

	_, err = store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath:              "codes_ns/n2",
		Path:                 "ns/n2",
		RepositoryType:       types.DatasetRepo,
		SensitiveCheckStatus: types.SensitiveCheckFail,
		Private:              true,
	}))
	require.Nil(t, err)

	repos, err := store.FindByRepoTypeAndPaths(ctx, types.ModelRepo, []string{"ns/n", "ns/n1", "ns/n2"})
	require.Nil(t, err)

	require.Equal(t, 2, len(repos))
	actualPaths := make([]string, len(repos))
	for i, r := range repos {
		actualPaths[i] = r.GitPath
	}
	require.ElementsMatch(t, []string{"codes_ns/n", "codes_ns/n1"}, actualPaths)
}

// TestRepoStore_FindWithMirror verifies repository lookup loads its mirror association.
func TestRepoStore_FindWithMirror(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	repoStore := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	repo, err := repoStore.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		GitPath:        "models_ns/mirrored",
		Path:           "ns/mirrored",
		RepositoryType: types.ModelRepo,
	}))
	require.NoError(t, err)

	mirrorStore := database.NewMirrorStoreWithDB(db)
	_, err = mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:    "https://huggingface.co/source/mirrored.git",
		RepositoryID: repo.ID,
	})
	require.NoError(t, err)

	got, err := repoStore.FindWithMirror(ctx, types.ModelRepo, "ns", "mirrored")
	require.NoError(t, err)
	require.Equal(t, repo.ID, got.ID)
	require.NotNil(t, got.Mirror)
}

func TestRepoStore_PublicToUserMirror(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// insert a new repo
	rs := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	userName := "user_name_" + uuid.NewString()
	repoName := "repo_name_" + uuid.NewString()
	repo, err := rs.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName, repoName),
		GitPath:        fmt.Sprintf("datasets_%s/%s", userName, repoName),
		Name:           repoName,
		DefaultBranch:  "main",
		Nickname:       "ww",
		Description:    "ww",
		Private:        false,
		RepositoryType: types.DatasetRepo,
		Source:         "local",
	}))
	require.Nil(t, err)
	require.NotNil(t, repo)

	// Create corresponding record in datasets table for DatasetRepo
	dataset := database.Dataset{
		RepositoryID: repo.ID,
	}
	_, err = db.Core.NewInsert().Model(&dataset).Exec(ctx)
	require.Nil(t, err)

	userName1 := "user_name_" + uuid.NewString()
	repoName1 := "repo_name_" + uuid.NewString()
	repo1, err := rs.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName1, repoName1),
		GitPath:        fmt.Sprintf("datasets_%s/%s", userName1, repoName1),
		Name:           repoName,
		DefaultBranch:  "main",
		Nickname:       "ww",
		Description:    "ww",
		Private:        false,
		RepositoryType: types.DatasetRepo,
		Source:         "opencsg",
	}))
	require.Nil(t, err)
	require.NotNil(t, repo1)

	// Create corresponding record in datasets table for DatasetRepo
	dataset1 := database.Dataset{
		RepositoryID: repo1.ID,
	}
	_, err = db.Core.NewInsert().Model(&dataset1).Exec(ctx)
	require.Nil(t, err)

	mirrorStore := database.NewMirrorStoreWithDB(db)
	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		RepositoryID: repo1.ID,
	})
	require.Nil(t, err)

	mirrorTaskStore := database.NewMirrorTaskStoreWithDB(db)
	_, err = mirrorTaskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorLfsSyncFinished,
	})
	require.Nil(t, err)

	filter := &types.RepoFilter{
		Source: "local",
	}
	// case 1: one tag
	repos, _, err := rs.PublicToUserWithAccess(ctx, repo.RepositoryType, database.NewRepositoryAccessScope(database.RepositoryAccessAdmin, nil), filter, 20, 1)
	require.Nil(t, err)
	require.NotNil(t, repos)
	require.Equal(t, 2, len(repos))
}

func TestRepoStore_FindUnhashedRepos(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// insert a new repo
	rs := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	for i := 0; i < 10; i++ {
		_, err := db.Operator.Core.NewInsert().Model(&database.Repository{
			UserID:         1,
			Path:           fmt.Sprintf("%s/%d", "path", i),
			GitPath:        fmt.Sprintf("datasets_%s/%d", "path", i),
			Name:           fmt.Sprintf("name_%d", i),
			DefaultBranch:  "main",
			Nickname:       "ww",
			Description:    "ww",
			Private:        false,
			RepositoryType: types.DatasetRepo,
			Source:         "opencsg",
			Hashed:         false,
		}).Exec(ctx)
		require.Nil(t, err)
	}

	userName1 := "user_name_" + uuid.NewString()
	repoName1 := "repo_name_" + uuid.NewString()
	repo1, err := rs.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName1, repoName1),
		GitPath:        fmt.Sprintf("datasets_%s/%s", userName1, repoName1),
		Name:           repoName1,
		DefaultBranch:  "main",
		Nickname:       "ww",
		Description:    "ww",
		Private:        false,
		RepositoryType: types.DatasetRepo,
		Source:         "opencsg",
	}))
	require.Nil(t, err)
	require.NotNil(t, repo1)

	repos, err := rs.FindUnhashedRepos(ctx, 10, 0)
	require.Nil(t, err)
	require.NotNil(t, repos)
	require.Equal(t, 10, len(repos))
	for _, repo := range repos {
		require.Equal(t, false, repo.Hashed)
	}
}

func TestRepoStore_PublicToUserWithCacheFailed(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	cache := mockcache.NewMockRedisClient(t)
	cache.EXPECT().Exists(mock.Anything, mock.Anything).Return(0, nil).Once()
	cache.EXPECT().Pipelined(mock.Anything, mock.Anything).Return(nil, errors.New("error")).Once()

	config := &config.Config{}
	config.Search.RepoSearchCacheTTL = 300
	config.Search.RepoSearchLimit = 10
	config.Database.Driver = "pg"
	config.Database.SearchConfiguration = "opencsgchinese"

	store := database.NewRepoStoreWithCacheAndRebacTupleJobClient(config, db, cache, tests.NewFixtureRebacTupleJobClient(db))
	_, err := setupTestRepositories(t, ctx, store, db)
	require.Nil(t, err)

	c := repoSearchCase{

		admin:    false,
		repoType: types.CodeRepo,
		search:   "billionaire",
		expected: []string{"ChineseBlue", "ChineseMedicalBooksCollection"},
	}
	rs, count, err := store.PublicToUserWithAccess(ctx, c.repoType,
		database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil), &types.RepoFilter{
			Tags:   c.tags,
			Sort:   c.sort,
			Search: c.search,
			Source: c.source,
		}, 10, 1)
	require.Nil(t, err)
	names := []string{}
	for _, r := range rs {
		names = append(names, r.Name)
	}
	require.ElementsMatch(t, c.expected, names)
	require.Equal(t, len(c.expected), count)
}

func TestRepoStore_UpdateRepoSensitiveCheckStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))

	repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path:    "u/bar",
		Name:    "repo1",
		UserID:  123,
		GitPath: "foos_u/bar",
	}))
	require.Nil(t, err)

	err = store.UpdateRepoSensitiveCheckStatus(ctx, repo.ID, types.SensitiveCheckPass)
	require.Nil(t, err)

	rp := &database.Repository{}
	err = db.Core.NewSelect().Model(rp).Where("id=?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SensitiveCheckPass, rp.SensitiveCheckStatus)
}

func TestRepoStore_GetReposBySearch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// insert a new repo
	rs := database.NewRepoStoreWithDB(db)
	for i := 0; i < 10; i++ {
		_, err := db.Operator.Core.NewInsert().Model(&database.Repository{
			UserID:         1,
			Path:           fmt.Sprintf("%s/%d", "path", i),
			GitPath:        fmt.Sprintf("datasets_%s/%d", "path", i),
			Name:           fmt.Sprintf("name_%d", i),
			DefaultBranch:  "main",
			Nickname:       "ww",
			Description:    "ww",
			Private:        false,
			RepositoryType: types.DatasetRepo,
			Source:         "opencsg",
			Hashed:         false,
		}).Exec(ctx)
		require.Nil(t, err)
	}

	repos, total, err := rs.GetReposBySearch(ctx, "path/1", types.DatasetRepo, 1, 10)
	require.Nil(t, err)
	require.NotNil(t, repos)
	require.Equal(t, 1, total)
}

// TestRepoStore_PostgresSearchTags preserves legacy tags across cache paths while keeping V2 light.
func TestRepoStore_PostgresSearchTags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	client := tests.InitTestRedis()
	defer client.Close()
	cfg := &config.Config{}
	cfg.Database.Driver = "pg"
	cfg.Database.SearchConfiguration = "simple"
	cfg.Search.RepoSearchLimit = 10
	cfg.Search.RepoSearchCacheTTL = 300
	store := database.NewRepoStoreWithCacheAndRebacTupleJobClient(cfg, db, cache.NewCacheWithClient(ctx, client), tests.NewFixtureRebacTupleJobClient(db))
	repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Name: "tagsearch", Path: "owner/tagsearch", GitPath: "owner/tagsearch", RepositoryType: types.CodeRepo,
	}))
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&database.Code{RepositoryID: repo.ID}).Exec(ctx)
	require.NoError(t, err)
	tag := &database.Tag{Name: "search-tag", Category: "task"}
	_, err = db.Core.NewInsert().Model(tag).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, store.BatchCreateRepoTags(ctx, []database.RepositoryTag{{RepositoryID: repo.ID, TagID: tag.ID}}))
	_, err = db.Core.NewUpdate().Model((*database.Repository)(nil)).
		Set("search_vector = to_tsvector('simple', 'tagsearch')").Where("id = ?", repo.ID).Exec(ctx)
	require.NoError(t, err)
	for _, v2 := range []bool{false, true} {
		require.NoError(t, client.FlushDB(ctx).Err())
		for _, path := range []string{"miss", "hit", "bypass"} {
			t.Run(fmt.Sprintf("v2=%v/%s", v2, path), func(t *testing.T) {
				scope := database.NewRepositoryAccessScope(database.RepositoryAccessPublic, nil)
				if path == "bypass" {
					scope = database.NewRepositoryAccessScope(database.RepositoryAccessReadable, []int64{repo.ID})
				}
				filter := &types.RepoFilter{Search: "tagsearch"}
				var rows []*database.Repository
				var count int
				var err error
				if v2 {
					rows, count, err = store.PublicToUserV2WithAccess(ctx, types.CodeRepo, scope, filter, 1, 1)
				} else {
					rows, count, err = store.PublicToUserWithAccess(ctx, types.CodeRepo, scope, filter, 1, 1)
				}
				require.NoError(t, err)
				require.Equal(t, 1, count)
				require.Len(t, rows, 1)
				require.Equal(t, repo.ID, rows[0].ID)
				if v2 {
					require.Empty(t, rows[0].Tags)
				} else {
					require.Len(t, rows[0].Tags, 1)
					require.Equal(t, tag.ID, rows[0].Tags[0].ID)
					require.Equal(t, tag.Name, rows[0].Tags[0].Name)
				}
			})
		}
	}
}

// TestRepoStore_PendingPublishMigration verifies rollback and safe defaults for existing private repositories.
func TestRepoStore_PendingPublishMigration(t *testing.T) {
	// DDL must use an isolated database so shared test transactions cannot block it.
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	repo, _ := newPublicationFixture(t, db, "migration-default", true, types.SensitiveCheckPass)
	for _, migration := range migrations.Migrations.Sorted() {
		if migration.Name != "20261018042946" {
			continue
		}
		require.NoError(t, migration.Down(ctx, db.BunDB, nil))
		var private bool
		require.NoError(t, db.BunDB.NewRaw("SELECT private FROM repositories WHERE id = ?", repo.ID).Scan(ctx, &private))
		require.True(t, private)
		require.NoError(t, migration.Up(ctx, db.BunDB, nil))
		got, err := database.NewRepoStoreWithDB(db).FindById(ctx, repo.ID)
		require.NoError(t, err)
		require.True(t, got.Private)
		require.False(t, got.PendingPublish)
		return
	}
	t.Fatal("pending publication migration was not registered")
}

// TestRepoStore_VisibilityWithUnchangedInheritance keeps explicit visibility requests
// out of the inheritance no-op path without enqueueing unchanged permissions.
func TestRepoStore_VisibilityWithUnchangedInheritance(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		private bool
		status  types.SensitiveCheckStatus
	}{
		{"cancel-publication", true, types.SensitiveCheckPass},
		{"publish", false, types.SensitiveCheckPass},
		{"reject-failed-check", false, types.SensitiveCheckFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := newPublicationFixture(t, db, tc.name, true, tc.status)
			request := *repo
			request.TupleNamespacePath = "publication"
			request.SensitiveCheckStatus = types.SensitiveCheckPass
			before := tupleJobCount(t, db)
			// An unchanged permission setting must not require a tuple producer.
			store := database.NewRepoStoreWithDB(db)
			result, err := store.UpdateRepo(ctx, request, types.RepositoryUpdateOptions{
				Private: &tc.private, RequireSensitiveCheckPass: !tc.private,
			})
			if tc.status == types.SensitiveCheckFail {
				require.ErrorIs(t, err, errorx.ErrSensitiveCheckNotPassed)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.private, result.Private)
				require.False(t, result.PendingPublish)
			}
			stored, err := store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.Equal(t, tc.private || tc.status == types.SensitiveCheckFail, stored.Private)
			require.Equal(t, tc.status == types.SensitiveCheckFail, stored.PendingPublish)
			require.Equal(t, tc.status, stored.SensitiveCheckStatus)
			require.Equal(t, repo.OrgInheritBlocked, stored.OrgInheritBlocked)
			require.Equal(t, before, tupleJobCount(t, db))
		})
	}
}

// newPublicationFixture creates a private model with a current successful mirror task.
func newPublicationFixture(t *testing.T, db *database.DB, name string, pending bool, status types.SensitiveCheckStatus) (*database.Repository, *database.Mirror) {
	t.Helper()
	ctx := context.Background()
	repo, err := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db)).CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path: "publication/" + name, GitPath: "models_publication/" + name, Name: name,
		RepositoryType: types.ModelRepo, Private: true, PendingPublish: pending,
		SensitiveCheckStatus: status, SyncStatus: types.SyncStatusCompleted,
	}))
	require.NoError(t, err)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	mirror, err := mirrorStore.Create(ctx, &database.Mirror{RepositoryID: repo.ID})
	require.NoError(t, err)
	task, err := database.NewMirrorTaskStoreWithDB(db).Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID, Status: types.MirrorLfsSyncFinished,
	})
	require.NoError(t, err)
	mirror.CurrentTaskID = task.ID
	require.NoError(t, database.NewMirrorTaskStoreWithDB(db).SetMirrorCurrentTaskID(ctx, task))
	return repo, mirror
}

// TestRepoStore_PublishPendingModelRepo checks one-time publication and current-task eligibility.
func TestRepoStore_PublishPendingModelRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDB(db)
	for i, tc := range []struct {
		name       string
		pending    bool
		status     types.SensitiveCheckStatus
		current    types.MirrorTaskStatus
		wantPublic bool
	}{
		{name: "internal successful model", pending: true, status: types.SensitiveCheckPass, current: types.MirrorLfsSyncFinished, wantPublic: true},
		{name: "skipped check", pending: true, status: types.SensitiveCheckSkip, current: types.MirrorLfsSyncFinished, wantPublic: true},
		{name: "user private model", status: types.SensitiveCheckPass, current: types.MirrorLfsSyncFinished},
		{name: "failed sensitive check", pending: true, status: types.SensitiveCheckFail, current: types.MirrorLfsSyncFinished},
		{name: "pending sensitive check", pending: true, status: types.SensitiveCheckPending, current: types.MirrorLfsSyncFinished},
		{name: "historical success current queued", pending: true, status: types.SensitiveCheckPass, current: types.MirrorQueued},
		{name: "historical success current failed", pending: true, status: types.SensitiveCheckPass, current: types.MirrorLfsSyncFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, mirror := newPublicationFixture(t, db, fmt.Sprintf("publish-%d", i), tc.pending, tc.status)
			if tc.current != types.MirrorLfsSyncFinished {
				task, err := database.NewMirrorTaskStoreWithDB(db).Create(ctx, database.MirrorTask{MirrorID: mirror.ID, Status: tc.current})
				require.NoError(t, err)
				mirror.CurrentTaskID = task.ID
				require.NoError(t, database.NewMirrorTaskStoreWithDB(db).SetMirrorCurrentTaskID(ctx, task))
				candidates, err := store.FindMirrorFinishedPrivateModelRepo(ctx)
				require.NoError(t, err)
				for _, candidate := range candidates {
					require.NotEqual(t, repo.ID, candidate.ID)
				}
			}
			require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
			require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
			got, err := store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.Equal(t, !tc.wantPublic, got.Private)
			require.Equal(t, tc.pending && !tc.wantPublic, got.PendingPublish)
		})
	}
}

// TestRepoStore_PrivateSettingCancelsPendingPublish covers both unchanged private settings
// and private settings after publication, including stale candidates and metadata snapshots.
func TestRepoStore_PrivateSettingCancelsPendingPublish(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDB(db)
	for _, publishFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("already_published_%t", publishFirst), func(t *testing.T) {
			repo, _ := newPublicationFixture(t, db, fmt.Sprintf("cancel-%t", publishFirst), true, types.SensitiveCheckPass)
			stale := *repo
			if publishFirst {
				require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
			}
			got, err := store.UpdateRepo(ctx, *repo, types.RepositoryUpdateOptions{Private: new(true)})
			require.NoError(t, err)
			require.True(t, got.Private)
			require.False(t, got.PendingPublish)
			// Neither a stale scheduler candidate nor a metadata edit may undo cancellation.
			require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
			stale.Private = false
			stale.Description = "new description"
			_, err = store.UpdateRepo(ctx, stale)
			require.NoError(t, err)
			require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
			got, err = store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.True(t, got.Private)
			require.False(t, got.PendingPublish)
			require.Equal(t, "new description", got.Description)
		})
	}
}

// TestRepoStore_UpdateRepoDefersModelPublication keeps incomplete models private
// and preserves cancellation across the actual task requeue transaction.
func TestRepoStore_UpdateRepoDefersModelPublication(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDB(db)
	repo, mirror := newPublicationFixture(t, db, "defer-and-retry", false, types.SensitiveCheckPass)
	tasks := database.NewMirrorTaskJobStoreWithDB(db)
	_, err := tasks.RequeueMirrorRepoTask(ctx, database.RequeueMirrorRepoTaskInput{
		MirrorID: mirror.ID, RepositoryID: repo.ID, JobClient: &fakeMirrorRepoJobClient{},
	})
	require.NoError(t, err)
	got, err := store.UpdateRepo(ctx, *repo, types.RepositoryUpdateOptions{Private: new(false)})
	require.NoError(t, err)
	require.True(t, got.Private)
	require.True(t, got.PendingPublish)
	// Metadata snapshots must preserve, rather than clear, the pending request.
	got, err = store.UpdateRepo(ctx, *repo)
	require.NoError(t, err)
	require.True(t, got.PendingPublish)
	_, err = store.UpdateRepo(ctx, *got, types.RepositoryUpdateOptions{Private: new(true)})
	require.NoError(t, err)
	retried, err := tasks.RequeueMirrorRepoTask(ctx, database.RequeueMirrorRepoTaskInput{
		MirrorID: mirror.ID, RepositoryID: repo.ID, JobClient: &fakeMirrorRepoJobClient{},
	})
	require.NoError(t, err)
	// Finishing the retried task still must not publish a model whose owner cancelled.
	retried, err = tasks.UpdateStatusAndRepoSyncStatus(ctx, retried, database.MirrorContinue)
	require.NoError(t, err)
	retried, err = tasks.UpdateStatusAndRepoSyncStatus(ctx, retried, database.MirrorNoLfsToSync)
	require.NoError(t, err)
	require.Equal(t, types.MirrorLfsSyncFinished, retried.Status)
	require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
	got, err = store.FindById(ctx, repo.ID)
	require.NoError(t, err)
	require.True(t, got.Private)
	require.False(t, got.PendingPublish)
}

// newMirrorDeletionFixture creates active or failed work with a pending publication request.
func newMirrorDeletionFixture(t *testing.T, db *database.DB, name string, syncStatus types.RepositorySyncStatus) (*database.Repository, *database.Mirror) {
	t.Helper()
	ctx := context.Background()
	repo, mirror := newPublicationFixture(t, db, name, true, types.SensitiveCheckPass)
	tasks := database.NewMirrorTaskJobStoreWithDB(db)
	task, err := tasks.RequeueMirrorRepoTask(ctx, database.RequeueMirrorRepoTaskInput{
		MirrorID: mirror.ID, RepositoryID: repo.ID, JobClient: &fakeMirrorRepoJobClient{},
	})
	require.NoError(t, err)
	if syncStatus == types.SyncStatusFailed {
		_, err = tasks.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorFatal)
		require.NoError(t, err)
	}
	repo, err = database.NewRepoStoreWithDB(db).FindById(ctx, repo.ID)
	require.NoError(t, err)
	return repo, mirror
}

// TestRepoStore_PublicModelAfterMirrorDelete preserves manual visibility after real active/failed mirror deletion.
func TestRepoStore_PublicModelAfterMirrorDelete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDB(db)
	for _, syncStatus := range []types.RepositorySyncStatus{types.SyncStatusCanceled, types.SyncStatusFailed} {
		t.Run(string(syncStatus), func(t *testing.T) {
			repo, mirror := newMirrorDeletionFixture(t, db, "deleted-"+string(syncStatus), syncStatus)
			require.NoError(t, database.NewMirrorStoreWithDB(db).DeleteWithTaskCancelTx(ctx, mirror.ID, &fakeMirrorJobCancelClient{}))
			repo, err := store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.True(t, repo.PendingPublish)
			got, err := store.UpdateRepo(ctx, *repo, types.RepositoryUpdateOptions{Private: new(false)})
			require.NoError(t, err)
			require.False(t, got.Private)
			require.False(t, got.PendingPublish)
			require.Equal(t, syncStatus, got.SyncStatus)
			persisted, err := store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.False(t, persisted.Private)
			require.False(t, persisted.PendingPublish)
			_, err = store.UpdateRepo(ctx, *got, types.RepositoryUpdateOptions{Private: new(true)})
			require.NoError(t, err)
			_, err = store.UpdateRepo(ctx, *got)
			require.NoError(t, err)
			require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
			persisted, err = store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.True(t, persisted.Private)
			require.False(t, persisted.PendingPublish)
		})
	}
}

// TestRepoStore_ImmediatePublicCommandFromPendingState replays the CLI's read,
// check-status update, and explicit public request without rewriting persisted intent.
// An immediate request must still leave unfinished synchronization hidden.
func TestRepoStore_ImmediatePublicCommandFromPendingState(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDB(db)
	for _, syncStatus := range []types.RepositorySyncStatus{types.SyncStatusCompleted, types.SyncStatusCanceled, types.SyncStatusFailed, types.SyncStatusPending} {
		t.Run(string(syncStatus), func(t *testing.T) {
			var repo *database.Repository
			if syncStatus == types.SyncStatusCompleted {
				repo, _ = newPublicationFixture(t, db, "command-completed", true, types.SensitiveCheckPending)
			} else {
				var mirror *database.Mirror
				repo, mirror = newMirrorDeletionFixture(t, db, "command-"+string(syncStatus), syncStatus)
				if syncStatus != types.SyncStatusPending {
					require.NoError(t, database.NewMirrorStoreWithDB(db).DeleteWithTaskCancelTx(ctx, mirror.ID, &fakeMirrorJobCancelClient{}))
				}
				require.NoError(t, store.UpdateRepoSensitiveCheckStatus(ctx, repo.ID, types.SensitiveCheckPending))
			}
			snapshot, err := store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.True(t, snapshot.Private)
			require.True(t, snapshot.PendingPublish)
			snapshot.Private = false
			snapshot.SensitiveCheckStatus = types.SensitiveCheckPass
			require.NoError(t, store.UpdateRepoSensitiveCheckStatus(ctx, repo.ID, types.SensitiveCheckPass))
			got, err := store.UpdateRepo(ctx, *snapshot, types.RepositoryUpdateOptions{
				Private: new(false), DeferPublication: false,
			})
			require.NoError(t, err)
			unfinished := syncStatus == types.SyncStatusPending
			require.Equal(t, unfinished, got.Private)
			require.Equal(t, unfinished, got.PendingPublish)
			persisted, err := store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.Equal(t, unfinished, persisted.Private)
			require.Equal(t, unfinished, persisted.PendingPublish)
			require.Equal(t, syncStatus, persisted.SyncStatus)
			require.Equal(t, types.SensitiveCheckPass, persisted.SensitiveCheckStatus)
		})
	}
}

// TestRepoStore_PublicModelKeepsSynchronizationHidden excludes bound terminal mirrors and pending initial bindings from the exception.
func TestRepoStore_PublicModelKeepsSynchronizationHidden(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	for _, syncStatus := range []types.RepositorySyncStatus{types.SyncStatusPending, types.SyncStatusCanceled, types.SyncStatusFailed} {
		t.Run(string(syncStatus), func(t *testing.T) {
			repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
				Path: "lifecycle/" + string(syncStatus), GitPath: "models_lifecycle/" + string(syncStatus),
				RepositoryType: types.ModelRepo, Private: true, PendingPublish: true, SyncStatus: syncStatus,
			}))
			require.NoError(t, err)
			if syncStatus != types.SyncStatusPending {
				_, err = database.NewMirrorStoreWithDB(db).Create(ctx, &database.Mirror{RepositoryID: repo.ID})
				require.NoError(t, err)
			}
			got, err := store.UpdateRepo(ctx, *repo, types.RepositoryUpdateOptions{Private: new(false)})
			require.NoError(t, err)
			require.True(t, got.Private)
			require.True(t, got.PendingPublish)
		})
	}
}

// modelVisibilityUpdateContextKey scopes the concurrency gate to one settings request.
type modelVisibilityUpdateContextKey struct{}

// sensitiveFailureUpdateContextKey identifies the competing moderation update.
type sensitiveFailureUpdateContextKey struct{}

// modelVisibilityUpdateHook pauses settings immediately before or after acquiring the row lock.
type modelVisibilityUpdateHook struct {
	ready         chan struct{}
	resume        chan struct{}
	afterLock     bool
	reviewAttempt chan struct{}
}

// BeforeQuery permits an independent mirror transaction to commit before settings recheck the locked row.
func (h *modelVisibilityUpdateHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if h.reviewAttempt != nil && ctx.Value(sensitiveFailureUpdateContextKey{}) == h && strings.HasPrefix(event.Query, `UPDATE "repositories"`) {
		close(h.reviewAttempt)
	}
	if !h.afterLock {
		h.pause(ctx, event)
	}
	return ctx
}

// pause scopes the deterministic gate to one settings request and one repository lock query.
func (h *modelVisibilityUpdateHook) pause(ctx context.Context, event *bun.QueryEvent) {
	if ctx.Value(modelVisibilityUpdateContextKey{}) == h && strings.Contains(event.Query, `FROM "repositories" AS "repository"`) && strings.Contains(event.Query, "FOR UPDATE") {
		close(h.ready)
		select {
		case <-h.resume:
		case <-ctx.Done():
		}
	}
}

// AfterQuery can retain the settings row lock while a concurrent moderation update starts.
func (h *modelVisibilityUpdateHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	if h.afterLock && event.Err == nil {
		h.pause(ctx, event)
	}
}

// TestRepoStore_ManualPublicSensitiveCheckGuard checks every review state and repository type.
func TestRepoStore_ManualPublicSensitiveCheckGuard(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	// Reject malformed guards before any repository lookup or write.
	for _, options := range [][]types.RepositoryUpdateOptions{
		{{RequireSensitiveCheckPass: true}},
		{{Private: new(true), RequireSensitiveCheckPass: true}},
		{{DeferPublication: true}},
		{{Private: new(true), DeferPublication: true}},
		{{}, {}},
	} {
		_, err := store.UpdateRepo(ctx, database.Repository{}, options...)
		require.Error(t, err)
	}
	for _, repoType := range []types.RepositoryType{types.ModelRepo, types.DatasetRepo, types.CodeRepo, types.SpaceRepo,
		types.PromptRepo, types.MCPServerRepo, types.SkillRepo, types.TemplateRepo} {
		for _, status := range []types.SensitiveCheckStatus{types.SensitiveCheckFail, types.SensitiveCheckPending,
			types.SensitiveCheckPass, types.SensitiveCheckSkip, types.SensitiveCheckException, types.SensitiveCheckStatus(99)} {
			t.Run(fmt.Sprintf("%s/%d", repoType, status), func(t *testing.T) {
				name := fmt.Sprintf("guard-%s-%d", repoType, status)
				repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{Path: "visibility/" + name, GitPath: name,
					RepositoryType: repoType, Private: true, SensitiveCheckStatus: status, SyncStatus: types.SyncStatusCompleted}))
				require.NoError(t, err)
				stale := *repo
				stale.Private = false
				stale.SensitiveCheckStatus = types.SensitiveCheckPass
				got, err := store.UpdateRepo(ctx, stale, types.RepositoryUpdateOptions{Private: new(false), RequireSensitiveCheckPass: true})
				if status == types.SensitiveCheckPass {
					require.NoError(t, err)
					require.False(t, got.Private)
				} else {
					require.ErrorIs(t, err, errorx.ErrSensitiveCheckNotPassed)
					require.Nil(t, got)
					// Preserve the repository error code rather than exposing a generic database failure.
					var customErr errorx.CustomError
					require.ErrorAs(t, err, &customErr)
					require.Equal(t, errorx.ErrSensitiveCheckNotPassed.(errorx.CustomError).Code(), customErr.Code())
				}
				persisted, err := store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				require.Equal(t, status != types.SensitiveCheckPass, persisted.Private)
				require.Equal(t, status, persisted.SensitiveCheckStatus)
			})
		}
	}
}

// TestRepoStore_ManualPublicSerializesSensitiveFailure exercises both transaction orders.
func TestRepoStore_ManualPublicSerializesSensitiveFailure(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	for _, syncStatus := range []types.RepositorySyncStatus{types.SyncStatusCompleted, types.SyncStatusCanceled, types.SyncStatusFailed} {
		for _, publicFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/public_first_%t", syncStatus, publicFirst), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{Path: fmt.Sprintf("visibility/race-%s-%t", syncStatus, publicFirst),
					GitPath: fmt.Sprintf("race-%s-%t", syncStatus, publicFirst), RepositoryType: types.ModelRepo,
					Private: true, SensitiveCheckStatus: types.SensitiveCheckPass, SyncStatus: syncStatus}))
				require.NoError(t, err)
				hook := &modelVisibilityUpdateHook{ready: make(chan struct{}), resume: make(chan struct{}), afterLock: publicFirst, reviewAttempt: make(chan struct{})}
				db.BunDB.AddQueryHook(hook)
				resume := sync.OnceFunc(func() { close(hook.resume) })
				defer resume()
				publicDone := make(chan error, 1)
				go func() {
					_, err := store.UpdateRepo(context.WithValue(ctx, modelVisibilityUpdateContextKey{}, hook), *repo,
						types.RepositoryUpdateOptions{Private: new(false), RequireSensitiveCheckPass: true})
					publicDone <- err
				}()
				select {
				case <-hook.ready:
				case <-ctx.Done():
					t.Fatal("manual public request did not reach its row-lock gate")
				}
				failureDone := make(chan error, 1)
				if publicFirst {
					go func() {
						failureDone <- store.UpdateRepoSensitiveCheckStatus(context.WithValue(ctx, sensitiveFailureUpdateContextKey{}, hook), repo.ID, types.SensitiveCheckFail)
					}()
					select {
					case <-hook.reviewAttempt:
					case <-ctx.Done():
						t.Fatal("moderation did not attempt its update while manual public held the row lock")
					}
				} else {
					require.NoError(t, store.UpdateRepoSensitiveCheckStatus(ctx, repo.ID, types.SensitiveCheckFail))
				}
				resume()
				select {
				case err := <-publicDone:
					if publicFirst {
						require.NoError(t, err)
					} else {
						require.ErrorIs(t, err, errorx.ErrSensitiveCheckNotPassed)
					}
				case <-ctx.Done():
					t.Fatal("manual public request did not finish")
				}
				if publicFirst {
					select {
					case err := <-failureDone:
						require.NoError(t, err)
					case <-ctx.Done():
						t.Fatal("moderation update did not finish after manual public committed")
					}
				}
				got, err := store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				require.True(t, got.Private)
				require.False(t, got.PendingPublish)
				require.Equal(t, types.SensitiveCheckFail, got.SensitiveCheckStatus)
			})
		}
	}
}

// TestRepoStore_ModelPublicSettingRechecksConcurrentMirrorChanges exercises deletion,
// rebinding, and a later private choice using real transactions with stale settings snapshots.
func TestRepoStore_ModelPublicSettingRechecksConcurrentMirrorChanges(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	store := database.NewRepoStoreWithDB(db)
	mirrors := database.NewMirrorStoreWithDB(db)
	for _, syncStatus := range []types.RepositorySyncStatus{types.SyncStatusCanceled, types.SyncStatusFailed} {
		for _, action := range []string{"delete", "rebind", "rebind_then_private"} {
			t.Run(string(syncStatus)+"/"+action, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				repo, mirror := newMirrorDeletionFixture(t, db, "concurrent-"+string(syncStatus)+"-"+action, syncStatus)
				if action != "delete" {
					require.NoError(t, mirrors.DeleteWithTaskCancelTx(ctx, mirror.ID, &fakeMirrorJobCancelClient{}))
					var err error
					repo, err = store.FindById(ctx, repo.ID)
					require.NoError(t, err)
				}
				hook := &modelVisibilityUpdateHook{ready: make(chan struct{}), resume: make(chan struct{})}
				db.BunDB.AddQueryHook(hook)
				resume := sync.OnceFunc(func() { close(hook.resume) })
				defer resume()
				finished := make(chan error, 1)
				go func() {
					_, err := store.UpdateRepo(context.WithValue(ctx, modelVisibilityUpdateContextKey{}, hook), *repo, types.RepositoryUpdateOptions{
						Private: new(false), DeferPublication: action == "delete",
					})
					finished <- err
				}()
				select {
				case <-hook.ready:
				case <-ctx.Done():
					t.Fatal("settings did not reach the repository row lock")
				}
				if action == "delete" {
					require.NoError(t, mirrors.DeleteWithTaskCancelTx(ctx, mirror.ID, &fakeMirrorJobCancelClient{}))
				} else {
					var err error
					mirror, err = database.NewMirrorRepoStoreWithDB(db, nil).CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
						Repository: repo, Mirror: database.Mirror{SourceUrl: "https://opencsg.com/source/model.git"},
					})
					require.NoError(t, err)
				}
				resume()
				select {
				case err := <-finished:
					if action == "delete" {
						require.ErrorContains(t, err, "retry repository settings")
					} else {
						require.NoError(t, err)
					}
				case <-ctx.Done():
					t.Fatal("settings did not finish after the mirror transaction committed")
				}
				got, err := store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				require.True(t, got.Private)
				require.True(t, got.PendingPublish)
				if action == "delete" {
					require.Equal(t, syncStatus, got.SyncStatus)
					// A fresh request supplies immediate intent without rewriting the pending state.
					got, err = store.UpdateRepo(ctx, *got, types.RepositoryUpdateOptions{Private: new(false)})
					require.NoError(t, err)
					require.False(t, got.Private)
					require.False(t, got.PendingPublish)
					return
				}
				require.Equal(t, types.SyncStatusPending, got.SyncStatus)
				if action == "rebind_then_private" {
					_, err = store.UpdateRepo(ctx, *got, types.RepositoryUpdateOptions{Private: new(true)})
					require.NoError(t, err)
				}
				tasks := database.NewMirrorTaskStoreWithDB(db)
				task, err := tasks.FindByID(ctx, mirror.CurrentTaskID)
				require.NoError(t, err)
				started, err := tasks.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorContinue)
				require.NoError(t, err)
				_, err = tasks.UpdateStatusAndRepoSyncStatus(ctx, started, database.MirrorNoLfsToSync)
				require.NoError(t, err)
				require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
				got, err = store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				require.Equal(t, action == "rebind_then_private", got.Private)
				require.False(t, got.PendingPublish)
			})
		}
	}
}

// TestRepoStore_ModelMetadataSyncPreservesVisibility prevents upstream refreshes
// from undoing local private settings or cancelling an internal publication request.
func TestRepoStore_ModelMetadataSyncPreservesVisibility(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	for _, pending := range []bool{true, false} {
		repo, _ := newPublicationFixture(t, db, fmt.Sprintf("metadata-%t", pending), pending, types.SensitiveCheckPass)
		incoming := *repo
		incoming.Private = false
		incoming.PendingPublish = !pending
		incoming.SyncStatus = types.SyncStatusPending
		incoming.SensitiveCheckStatus = types.SensitiveCheckFail
		incoming.Description = "refreshed upstream metadata"
		got, err := store.UpdateOrCreateRepo(ctx, incoming)
		require.NoError(t, err)
		require.True(t, got.Private)
		require.Equal(t, pending, got.PendingPublish)
		require.Equal(t, types.SyncStatusCompleted, got.SyncStatus)
		require.Equal(t, types.SensitiveCheckPass, got.SensitiveCheckStatus)
		require.Equal(t, incoming.Description, got.Description)
	}
}

// TestRepoStore_NonModelMetadataSyncPreservesVisibility checks both user visibility
// settings across repeated upstream refreshes without changing initial import defaults.
func TestRepoStore_NonModelMetadataSyncPreservesVisibility(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	for _, repoType := range []types.RepositoryType{
		types.DatasetRepo, types.CodeRepo, types.PromptRepo, types.MCPServerRepo,
		types.SkillRepo, types.SpaceRepo, types.TemplateRepo,
	} {
		for _, private := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/private_%t", repoType, private), func(t *testing.T) {
				name := fmt.Sprintf("%s-%t", repoType, private)
				created, err := store.UpdateOrCreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
					Path: "metadata/" + name, GitPath: fmt.Sprintf("%ss_metadata/%s", repoType, name),
					Name: name, RepositoryType: repoType, Private: !private,
					SyncStatus: types.SyncStatusCompleted,
				}))
				require.NoError(t, err)
				require.Equal(t, !private, created.Private)
				// Apply the user's choice through the ordinary settings write, leaving an old upstream snapshot.
				configured := *created
				configured.Private = private
				_, err = store.UpdateRepo(ctx, configured)
				require.NoError(t, err)
				incoming := *created
				incoming.SyncStatus = types.SyncStatusPending
				for revision := 1; revision <= 2; revision++ {
					incoming.Description = fmt.Sprintf("upstream revision %d", revision)
					got, err := store.UpdateOrCreateRepo(ctx, incoming)
					require.NoError(t, err)
					require.Equal(t, created.ID, got.ID)
					require.Equal(t, private, got.Private)
					persisted, err := store.FindById(ctx, created.ID)
					require.NoError(t, err)
					require.Equal(t, private, persisted.Private)
					require.False(t, persisted.PendingPublish)
					require.Equal(t, incoming.Description, persisted.Description)
					require.Equal(t, types.SyncStatusPending, persisted.SyncStatus)
				}
			})
		}
	}
}

// TestRepoStore_MetadataSyncRespectsConcurrentVisibilitySetting verifies an
// upstream refresh waiting on a real settings transaction cannot overwrite its choice.
func TestRepoStore_MetadataSyncRespectsConcurrentVisibilitySetting(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	for _, private := range []bool{true, false} {
		t.Run(fmt.Sprintf("private_%t", private), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			name := fmt.Sprintf("concurrent-visibility-%t", private)
			repo, err := store.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
				Path: "metadata/" + name, GitPath: "datasets_metadata/" + name,
				Name: name, RepositoryType: types.DatasetRepo, Private: !private,
			}))
			require.NoError(t, err)
			tx, err := db.BunDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			_, err = tx.NewUpdate().Model(repo).Set("private = ?", private).WherePK().Exec(ctx)
			require.NoError(t, err)
			hook := &publicationLockHook{locking: make(chan struct{})}
			db.BunDB.AddQueryHook(hook)
			finished := make(chan error, 1)
			incoming := *repo
			incoming.Description = "concurrent upstream refresh"
			go func() {
				_, err := store.UpdateOrCreateRepo(ctx, incoming)
				finished <- err
			}()
			select {
			case <-hook.locking:
			case <-ctx.Done():
				t.Fatal("metadata synchronization did not reach the competing row lock")
			}
			require.NoError(t, tx.Commit())
			select {
			case err := <-finished:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("metadata synchronization did not finish after the visibility setting committed")
			}
			got, err := store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.Equal(t, private, got.Private)
			require.Equal(t, incoming.Description, got.Description)
		})
	}
}

// TestRepoStore_DeferredPublicRequestSurvivesConcurrentCompletion prevents a
// settings snapshot from bypassing or overwriting checks after synchronization finishes.
func TestRepoStore_DeferredPublicRequestSurvivesConcurrentCompletion(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	store := database.NewRepoStoreWithDB(db)
	for _, status := range []types.SensitiveCheckStatus{types.SensitiveCheckPending, types.SensitiveCheckPass} {
		t.Run(fmt.Sprintf("check-%d", status), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			repo, mirror := newPublicationFixture(t, db, fmt.Sprintf("completion-race-%d", status), false, status)
			tasks := database.NewMirrorTaskJobStoreWithDB(db)
			task, err := tasks.RequeueMirrorRepoTask(ctx, database.RequeueMirrorRepoTaskInput{
				MirrorID: mirror.ID, RepositoryID: repo.ID, JobClient: &fakeMirrorRepoJobClient{},
			})
			require.NoError(t, err)
			snapshot, err := store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.Equal(t, types.SyncStatusPending, snapshot.SyncStatus)
			require.False(t, snapshot.PendingPublish)
			hook := &modelVisibilityUpdateHook{ready: make(chan struct{}), resume: make(chan struct{})}
			db.BunDB.AddQueryHook(hook)
			resume := sync.OnceFunc(func() { close(hook.resume) })
			defer resume()
			finished := make(chan error, 1)
			go func() {
				_, err := store.UpdateRepo(context.WithValue(ctx, modelVisibilityUpdateContextKey{}, hook), *snapshot, types.RepositoryUpdateOptions{
					Private: new(false), DeferPublication: true,
				})
				finished <- err
			}()
			select {
			case <-hook.ready:
			case <-ctx.Done():
				t.Fatal("settings did not reach the repository row lock")
			}
			// Complete the actual current task before settings acquire the repository lock.
			task, err = tasks.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorContinue)
			require.NoError(t, err)
			_, err = tasks.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorNoLfsToSync)
			require.NoError(t, err)
			resume()
			select {
			case err := <-finished:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("settings did not finish after synchronization committed")
			}
			got, err := store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.True(t, got.Private)
			require.True(t, got.PendingPublish)
			require.Equal(t, types.SyncStatusCompleted, got.SyncStatus)
			require.Equal(t, status, got.SensitiveCheckStatus)
			require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
			got, err = store.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.Equal(t, status != types.SensitiveCheckPass, got.Private)
			require.Equal(t, status != types.SensitiveCheckPass, got.PendingPublish)
		})
	}
}

// TestRepoStore_SensitiveFailurePreservesPendingPublish distinguishes temporary
// moderation hiding from an explicit user cancellation of publication.
func TestRepoStore_SensitiveFailurePreservesPendingPublish(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDB(db)
	for _, pending := range []bool{true, false} {
		repo, _ := newPublicationFixture(t, db, fmt.Sprintf("moderation-%t", pending), pending, types.SensitiveCheckPass)
		require.NoError(t, store.UpdateRepoSensitiveCheckStatus(ctx, repo.ID, types.SensitiveCheckFail))
		require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
		got, err := store.FindById(ctx, repo.ID)
		require.NoError(t, err)
		require.True(t, got.Private)
		require.Equal(t, pending, got.PendingPublish)
		require.NoError(t, store.UpdateRepoSensitiveCheckStatus(ctx, repo.ID, types.SensitiveCheckPass))
		require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
		got, err = store.FindById(ctx, repo.ID)
		require.NoError(t, err)
		require.Equal(t, !pending, got.Private)
		require.False(t, got.PendingPublish)
	}
}

// TestRepoStore_DownloadUpdatesPreservePublicationState replays stale download
// snapshots after privacy changes and publication, including both statistics paths.
func TestRepoStore_DownloadUpdatesPreservePublicationState(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewRepoStoreWithDB(db)
	for _, kind := range []string{"file", "clone"} {
		for _, state := range []string{"cancel_pending", "private_after_publication", "publish_after_snapshot"} {
			t.Run(kind+"/"+state, func(t *testing.T) {
				repo, _ := newPublicationFixture(t, db, kind+"-"+state, true, types.SensitiveCheckPass)
				if state == "private_after_publication" {
					require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
				}
				stale, err := store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				wantPrivate := state != "publish_after_snapshot"
				if wantPrivate {
					_, err = store.UpdateRepo(ctx, *stale, types.RepositoryUpdateOptions{Private: new(true)})
					require.NoError(t, err)
				} else {
					require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
				}
				// The download started with the old visibility and finishes after the change.
				if kind == "file" {
					err = store.UpdateRepoFileDownloads(ctx, stale, time.Now(), 7)
				} else {
					err = store.UpdateRepoCloneDownloads(ctx, stale, time.Now(), 7)
				}
				require.NoError(t, err)
				got, err := store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				require.Equal(t, wantPrivate, got.Private)
				require.False(t, got.PendingPublish)
				require.EqualValues(t, 7, got.DownloadCount)
				require.NoError(t, store.PublishPendingModelRepo(ctx, repo.ID))
				got, err = store.FindById(ctx, repo.ID)
				require.NoError(t, err)
				require.Equal(t, wantPrivate, got.Private)
				require.False(t, got.PendingPublish)
			})
		}
	}
}

// publicationLockHook signals when a repository writer reaches the row lock.
type publicationLockHook struct {
	locking chan struct{}
	once    sync.Once
}

// BeforeQuery observes the repository lock without interfering with database execution.
func (h *publicationLockHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if strings.Contains(event.Query, `FROM "repositories" AS "repository"`) && strings.Contains(event.Query, "FOR UPDATE") {
		h.once.Do(func() { close(h.locking) })
	}
	return ctx
}

// AfterQuery completes the Bun query hook contract.
func (h *publicationLockHook) AfterQuery(context.Context, *bun.QueryEvent) {}

// TestRepoStore_PrivateSettingWinsAgainstWaitingPublication exercises real competing transactions.
func TestRepoStore_PrivateSettingWinsAgainstWaitingPublication(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, _ := newPublicationFixture(t, db, "concurrent-cancellation", true, types.SensitiveCheckPass)
	store := database.NewRepoStoreWithDB(db)
	tx, err := db.BunDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.NewUpdate().Model(repo).Set("private = TRUE").Set("pending_publish = FALSE").WherePK().Exec(ctx)
	require.NoError(t, err)
	hook := &publicationLockHook{locking: make(chan struct{})}
	db.BunDB.AddQueryHook(hook)
	finished := make(chan error, 1)
	go func() { finished <- store.PublishPendingModelRepo(ctx, repo.ID) }()
	select {
	case <-hook.locking:
	case <-ctx.Done():
		t.Fatal("publication did not reach the competing row lock")
	}
	require.NoError(t, tx.Commit())
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("publication did not finish after cancellation committed")
	}
	got, err := store.FindById(ctx, repo.ID)
	require.NoError(t, err)
	require.True(t, got.Private)
	require.False(t, got.PendingPublish)
}
