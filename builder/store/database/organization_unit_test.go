package database_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	coredb "opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

type repositoryNamespaceLockHook struct {
	path   string
	locked chan struct{}
	once   sync.Once
}

func (h *repositoryNamespaceLockHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

func (h *repositoryNamespaceLockHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	query := strings.ToUpper(event.Query)
	if event.Err == nil && strings.Contains(query, "FOR KEY SHARE") && strings.Contains(event.Query, h.path) {
		h.once.Do(func() { close(h.locked) })
	}
}

// TestNewOrgStore_SelectsStoreByOrganizationMode verifies Store selection is centralized in the constructor.
func TestNewOrgStore_SelectsStoreByOrganizationMode(t *testing.T) {
	cfg := &config.Config{}
	require.IsType(t, coredb.NewOrgStore(&config.Config{}), coredb.NewOrgStore(cfg))

	cfg.Organization.EnableUnit = true
	require.NotEqual(t, fmt.Sprintf("%T", coredb.NewOrgStore(&config.Config{})), fmt.Sprintf("%T", coredb.NewOrgStore(cfg)))
}

// TestOrganizationUnitStore_CreateRoot verifies root hierarchy initialization and administrator membership.
func TestOrganizationUnitStore_CreateRoot(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	creator := createOrganizationUnitMemberTestUser(t, ctx, db, "root-creator")
	organizationUUID := uuid.New()
	store := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, &testRepositoryDeletionJobClient{})
	created, err := store.CreateRoot(ctx, coredb.CreateRootOrganizationInput{
		Organization: &coredb.Organization{
			Name: "root-created", Nickname: "Root Created", UUID: organizationUUID, UserID: creator.ID, IsRoot: true,
		},
		Namespace:     &coredb.Namespace{Path: "root-created", UUID: organizationUUID.String()},
		CreatorUserID: creator.ID,
	})
	require.NoError(t, err)
	require.Equal(t, organizationUUID, created.UUID)
	require.True(t, created.IsRoot)
	require.NotNil(t, created.Namespace)

	var unit coredb.OrganizationUnit
	require.NoError(t, db.Core.NewSelect().Model(&unit).
		Where("organization_id = ? AND deleted_at IS NULL", created.ID).Scan(ctx))
	require.Equal(t, created.ID, unit.RootOrganizationID)
	require.Nil(t, unit.ParentUnitID)

	var closure coredb.OrganizationUnitClosure
	require.NoError(t, db.Core.NewSelect().Model(&closure).
		Where("root_organization_id = ? AND ancestor_unit_id = descendant_unit_id", created.ID).Scan(ctx))
	require.Equal(t, 0, closure.Depth)

	var member coredb.Member
	require.NoError(t, db.Core.NewSelect().Model(&member).
		Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", created.ID, creator.ID).Scan(ctx))
	require.Equal(t, string(types.UserAdmin), member.Role)

	childUUID := uuid.New()
	child, err := store.Create(ctx, coredb.CreateOrganizationUnitInput{
		RootOrganizationID: created.ID,
		Organization: &coredb.Organization{
			Name: "root-created-child", Nickname: "Root Child", UUID: childUUID, UserID: creator.ID,
		},
		Namespace: &coredb.Namespace{Path: "root-created-child", UUID: childUUID.String(), UserID: creator.ID},
		MaxDepth:  types.OrganizationUnitMaxDepth,
	})
	require.NoError(t, err)
	require.NotNil(t, child.ParentUnitUUID)
	require.Equal(t, created.UUID.String(), *child.ParentUnitUUID)
	require.NotNil(t, child.Tags)
	require.Empty(t, child.Tags)
	childRecord, err := store.FindByUUID(ctx, child.UUID)
	require.NoError(t, err)
	require.Equal(t, childRecord.OrganizationID, child.OrganizationID)
	var rootToChild coredb.OrganizationUnitClosure
	require.NoError(t, db.Core.NewSelect().Model(&rootToChild).
		Where("root_organization_id = ? AND ancestor_unit_id = ? AND descendant_unit_id = ?", created.ID, unit.ID, childRecord.ID).
		Scan(ctx))
	require.Equal(t, 1, rootToChild.Depth)

	roots, total, err := store.ListRoots(ctx, coredb.ListOrganizationUnitInput{RootOrganizationID: created.ID, Per: 20, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, child.UUID, roots[0].UUID)
	require.Equal(t, childRecord.OrganizationID, roots[0].OrganizationID)
	require.NotNil(t, roots[0].Tags)
	require.Empty(t, roots[0].Tags)

	_, err = store.Delete(ctx, coredb.DeleteOrganizationUnitInput{RootOrganizationID: created.ID, UnitID: unit.ID})
	require.Error(t, err)
	active, err := db.Core.NewSelect().Model((*coredb.Organization)(nil)).
		Where("id = ? AND deleted_at IS NULL", created.ID).Exists(ctx)
	require.NoError(t, err)
	require.True(t, active)
}

// TestOrganizationUnitStore_DeleteRoot removes the complete hierarchy in one transaction and is idempotent.
func TestOrganizationUnitStore_DeleteRoot(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	creator := createOrganizationUnitMemberTestUser(t, ctx, db, "root-delete-creator")
	member := createOrganizationUnitMemberTestUser(t, ctx, db, "root-delete-member")
	rootUUID := uuid.New()
	root, err := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, &testRepositoryDeletionJobClient{}).CreateRoot(ctx, coredb.CreateRootOrganizationInput{
		Organization: &coredb.Organization{Name: "root-delete", Nickname: "Root Delete", UUID: rootUUID, UserID: creator.ID, IsRoot: true, IsHierarchical: true},
		Namespace:    &coredb.Namespace{Path: "root-delete", UUID: rootUUID.String()}, CreatorUserID: creator.ID,
	})
	require.NoError(t, err)
	store := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, &testRepositoryDeletionJobClient{})
	child := createChild(t, ctx, store, root, "root-delete-child", nil, 1)
	grandchild := createChild(t, ctx, store, root, "root-delete-grandchild", &child.UUID, 1)
	childID := organizationIDForUnit(t, ctx, store, child.UUID)
	grandchildID := organizationIDForUnit(t, ctx, store, grandchild.UUID)
	require.NoError(t, coredb.NewMemberStoreWithDB(db).Add(ctx, childID, member.ID, string(types.UserRead)))
	require.NoError(t, coredb.NewMemberStoreWithDB(db).Add(ctx, grandchildID, member.ID, string(types.UserWrite)))

	result, err := store.DeleteRoot(ctx, coredb.DeleteRootOrganizationInput{OrganizationUUID: root.UUID.String()})
	require.NoError(t, err)
	require.False(t, result.AlreadyDeleted)
	require.Equal(t, 3, result.OrganizationsDeleted)
	require.Equal(t, 3, result.UnitsDeleted)
	require.Equal(t, 3, result.MembersRemoved)
	require.Equal(t, 2, result.UsersAffected)
	require.ElementsMatch(t, []string{root.UUID.String(), child.UUID, grandchild.UUID}, result.DeletedOrganizationUUIDs)
	require.ElementsMatch(t, []types.OrganizationHierarchyRelationship{
		{ParentOrganizationUUID: root.UUID.String(), ChildOrganizationUUID: child.UUID},
		{ParentOrganizationUUID: child.UUID, ChildOrganizationUUID: grandchild.UUID},
	}, result.DeletedHierarchyRelationships)
	cleanupByOrganization := make(map[string]types.OrganizationReBACCleanup, len(result.DeletedReBACRelationships))
	for _, cleanup := range result.DeletedReBACRelationships {
		cleanupByOrganization[cleanup.OrganizationUUID] = cleanup
	}
	require.Equal(t, root.UUID.String(), cleanupByOrganization[root.UUID.String()].NamespaceUUID)
	require.Equal(t, child.UUID, cleanupByOrganization[child.UUID].NamespaceUUID)
	require.Equal(t, grandchild.UUID, cleanupByOrganization[grandchild.UUID].NamespaceUUID)
	require.Contains(t, cleanupByOrganization[root.UUID.String()].UserUUIDs, creator.UUID)
	require.Contains(t, cleanupByOrganization[child.UUID].UserUUIDs, member.UUID)
	require.Contains(t, cleanupByOrganization[grandchild.UUID].UserUUIDs, member.UUID)

	var deletedRoot coredb.Organization
	require.NoError(t, db.Core.NewSelect().Model(&deletedRoot).WhereAllWithDeleted().Where("id = ?", root.ID).Scan(ctx))
	require.False(t, deletedRoot.DeletedAt.IsZero())
	var deletedChild coredb.Organization
	require.NoError(t, db.Core.NewSelect().Model(&deletedChild).WhereAllWithDeleted().Where("id = ?", childID).Scan(ctx))
	require.False(t, deletedChild.DeletedAt.IsZero())
	var deletedNamespace coredb.Namespace
	require.NoError(t, db.Core.NewSelect().Model(&deletedNamespace).WhereAllWithDeleted().Where("id = ?", deletedRoot.NamespaceID).Scan(ctx))
	require.False(t, deletedNamespace.DeletedAt.IsZero())
	closureCount, err := db.Core.NewSelect().Model((*coredb.OrganizationUnitClosure)(nil)).
		Where("root_organization_id = ?", root.ID).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, closureCount)
	activeMembers, err := db.Core.NewSelect().Model((*coredb.Member)(nil)).
		Where("organization_id IN (?) AND deleted_at IS NULL", bun.In([]int64{root.ID, childID, grandchildID})).Exists(ctx)
	require.NoError(t, err)
	require.False(t, activeMembers)

	repeated, err := store.DeleteRoot(ctx, coredb.DeleteRootOrganizationInput{OrganizationUUID: root.UUID.String()})
	require.NoError(t, err)
	require.True(t, repeated.AlreadyDeleted)
	require.ElementsMatch(t, result.DeletedOrganizationUUIDs, repeated.DeletedOrganizationUUIDs)
	require.ElementsMatch(t, result.DeletedHierarchyRelationships, repeated.DeletedHierarchyRelationships)
	require.ElementsMatch(t, result.DeletedReBACRelationships, repeated.DeletedReBACRelationships)
}

func TestOrganizationUnitStore_DeleteRootRepositories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	creator := createOrganizationUnitMemberTestUser(t, ctx, db, "root-repository-delete-creator")
	jobClient := &testRepositoryDeletionJobClient{}
	store := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, jobClient)
	rootUUID := uuid.New()
	root, err := store.CreateRoot(ctx, coredb.CreateRootOrganizationInput{
		Organization: &coredb.Organization{Name: "root-repository-delete", UUID: rootUUID, UserID: creator.ID, IsRoot: true, IsHierarchical: true},
		Namespace:    &coredb.Namespace{Path: "root-repository-delete", UUID: rootUUID.String()}, CreatorUserID: creator.ID,
	})
	require.NoError(t, err)
	child := createChild(t, ctx, store, root, "root-repository-delete-child", nil, 1)
	grandchild := createChild(t, ctx, store, root, "root-repository-delete-grandchild", &child.UUID, 1)

	repositories := []coredb.Repository{
		{UserID: creator.ID, Name: "root", Path: root.Name + "/root", GitPath: "models_" + root.Name + "/root", RepositoryType: types.ModelRepo},
		{UserID: creator.ID, Name: "child", Path: child.Name + "/child", GitPath: "models_" + child.Name + "/child", RepositoryType: types.ModelRepo},
		{UserID: creator.ID, Name: "grandchild", Path: grandchild.Name + "/grandchild", GitPath: "models_" + grandchild.Name + "/grandchild", RepositoryType: types.ModelRepo},
		{UserID: creator.ID, Name: "similar", Path: root.Name + "-other/similar", GitPath: "models_" + root.Name + "-other/similar", RepositoryType: types.ModelRepo},
	}
	_, err = db.Core.NewInsert().Model(&repositories).Exec(ctx)
	require.NoError(t, err)
	models := make([]coredb.Model, 0, len(repositories))
	for _, repository := range repositories {
		models = append(models, coredb.Model{RepositoryID: repository.ID})
	}
	_, err = db.Core.NewInsert().Model(&models).Exec(ctx)
	require.NoError(t, err)

	result, err := store.DeleteRoot(ctx, coredb.DeleteRootOrganizationInput{OrganizationUUID: root.UUID.String()})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{repositories[0].ID, repositories[1].ID, repositories[2].ID}, deletedRepositoryIDs(result.DeletedRepositories))
	repositoryOrganizations := deletedRepositoryOrganizations(result.DeletedRepositories)
	require.Equal(t, root.UUID.String(), repositoryOrganizations[repositories[0].ID])
	require.Equal(t, child.UUID, repositoryOrganizations[repositories[1].ID])
	require.Equal(t, grandchild.UUID, repositoryOrganizations[repositories[2].ID])
	require.NoError(t, db.Core.NewSelect().Model(&coredb.Repository{}).Where("id = ?", repositories[3].ID).Scan(ctx))
	require.Len(t, jobClient.recordedInputs(), 3)

	repeated, err := store.DeleteRoot(ctx, coredb.DeleteRootOrganizationInput{OrganizationUUID: root.UUID.String()})
	require.NoError(t, err)
	require.True(t, repeated.AlreadyDeleted)
	require.Empty(t, repeated.DeletedRepositories)
	require.Len(t, jobClient.recordedInputs(), 3)
}

func deletedRepositoryIDs(repositories []types.DeletedRepository) []int64 {
	ids := make([]int64, 0, len(repositories))
	for _, repository := range repositories {
		ids = append(ids, repository.ID)
	}
	return ids
}

func deletedRepositoryOrganizations(repositories []types.DeletedRepository) map[int64]string {
	organizations := make(map[int64]string, len(repositories))
	for _, repository := range repositories {
		organizations[repository.ID] = repository.OrganizationUUID
	}
	return organizations
}

func TestOrganizationUnitStore_DeleteRepositories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	creator := createOrganizationUnitMemberTestUser(t, ctx, db, "subtree-repository-delete-creator")
	jobClient := &testRepositoryDeletionJobClient{}
	store := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, jobClient)
	rootUUID := uuid.New()
	root, err := store.CreateRoot(ctx, coredb.CreateRootOrganizationInput{
		Organization: &coredb.Organization{Name: "subtree-repository-root", UUID: rootUUID, UserID: creator.ID, IsRoot: true, IsHierarchical: true},
		Namespace:    &coredb.Namespace{Path: "subtree-repository-root", UUID: rootUUID.String()}, CreatorUserID: creator.ID,
	})
	require.NoError(t, err)
	child := createChild(t, ctx, store, root, "subtree-repository-child", nil, 1)
	grandchild := createChild(t, ctx, store, root, "subtree-repository-grandchild", &child.UUID, 1)
	sibling := createChild(t, ctx, store, root, "subtree-repository-sibling", nil, 2)
	childUnit, err := store.FindByUUID(ctx, child.UUID)
	require.NoError(t, err)
	repositories := []coredb.Repository{
		{UserID: creator.ID, Name: "root", Path: root.Name + "/root", GitPath: "models_" + root.Name + "/root", RepositoryType: types.ModelRepo},
		{UserID: creator.ID, Name: "child", Path: child.Name + "/child", GitPath: "models_" + child.Name + "/child", RepositoryType: types.ModelRepo},
		{UserID: creator.ID, Name: "grandchild", Path: grandchild.Name + "/grandchild", GitPath: "models_" + grandchild.Name + "/grandchild", RepositoryType: types.ModelRepo},
		{UserID: creator.ID, Name: "sibling", Path: sibling.Name + "/sibling", GitPath: "models_" + sibling.Name + "/sibling", RepositoryType: types.ModelRepo},
	}
	_, err = db.Core.NewInsert().Model(&repositories).Exec(ctx)
	require.NoError(t, err)
	models := make([]coredb.Model, 0, len(repositories))
	for _, repository := range repositories {
		models = append(models, coredb.Model{RepositoryID: repository.ID})
	}
	_, err = db.Core.NewInsert().Model(&models).Exec(ctx)
	require.NoError(t, err)

	result, err := store.Delete(ctx, coredb.DeleteOrganizationUnitInput{RootOrganizationID: root.ID, UnitID: childUnit.ID})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{repositories[1].ID, repositories[2].ID}, deletedRepositoryIDs(result.DeletedRepositories))
	repositoryOrganizations := deletedRepositoryOrganizations(result.DeletedRepositories)
	require.Equal(t, child.UUID, repositoryOrganizations[repositories[1].ID])
	require.Equal(t, grandchild.UUID, repositoryOrganizations[repositories[2].ID])
	require.Len(t, jobClient.recordedInputs(), 2)
	for _, repository := range repositories[:1] {
		require.NoError(t, db.Core.NewSelect().Model(&coredb.Repository{}).Where("id = ?", repository.ID).Scan(ctx))
	}
	require.NoError(t, db.Core.NewSelect().Model(&coredb.Repository{}).Where("id = ?", repositories[3].ID).Scan(ctx))
}

func TestOrganizationUnitStore_DeleteRepositoriesRollback(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	creator := createOrganizationUnitMemberTestUser(t, ctx, db, "subtree-repository-rollback-creator")
	jobClient := &testRepositoryDeletionJobClient{err: errors.New("forced repository deletion enqueue failure")}
	store := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, jobClient)
	rootUUID := uuid.New()
	root, err := store.CreateRoot(ctx, coredb.CreateRootOrganizationInput{
		Organization: &coredb.Organization{Name: "subtree-repository-rollback-root", UUID: rootUUID, UserID: creator.ID, IsRoot: true, IsHierarchical: true},
		Namespace:    &coredb.Namespace{Path: "subtree-repository-rollback-root", UUID: rootUUID.String()}, CreatorUserID: creator.ID,
	})
	require.NoError(t, err)
	child := createChild(t, ctx, store, root, "subtree-repository-rollback-child", nil, 1)
	grandchild := createChild(t, ctx, store, root, "subtree-repository-rollback-grandchild", &child.UUID, 1)
	childUnit, err := store.FindByUUID(ctx, child.UUID)
	require.NoError(t, err)
	grandchildUnit, err := store.FindByUUID(ctx, grandchild.UUID)
	require.NoError(t, err)
	repository := coredb.Repository{
		UserID: creator.ID, Name: "rollback", Path: child.Name + "/rollback",
		GitPath: "models_" + child.Name + "/rollback", RepositoryType: types.ModelRepo,
	}
	_, err = db.Core.NewInsert().Model(&repository).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&coredb.Model{RepositoryID: repository.ID}).Exec(ctx)
	require.NoError(t, err)
	closureCountBefore, err := db.Core.NewSelect().Model((*coredb.OrganizationUnitClosure)(nil)).
		Where("root_organization_id = ?", root.ID).Count(ctx)
	require.NoError(t, err)

	result, err := store.Delete(ctx, coredb.DeleteOrganizationUnitInput{RootOrganizationID: root.ID, UnitID: childUnit.ID})
	require.Nil(t, result)
	require.ErrorContains(t, err, "forced repository deletion enqueue failure")

	for _, organizationID := range []int64{childUnit.OrganizationID, grandchildUnit.OrganizationID} {
		active, err := db.Core.NewSelect().Model((*coredb.Organization)(nil)).
			Where("id = ? AND deleted_at IS NULL", organizationID).Exists(ctx)
		require.NoError(t, err)
		require.True(t, active)
	}
	for _, unitID := range []int64{childUnit.ID, grandchildUnit.ID} {
		active, err := db.Core.NewSelect().Model((*coredb.OrganizationUnit)(nil)).
			Where("id = ? AND deleted_at IS NULL", unitID).Exists(ctx)
		require.NoError(t, err)
		require.True(t, active)
	}
	for _, namespacePath := range []string{child.Name, grandchild.Name} {
		active, err := db.Core.NewSelect().Model((*coredb.Namespace)(nil)).
			Where("path = ? AND deleted_at IS NULL", namespacePath).Exists(ctx)
		require.NoError(t, err)
		require.True(t, active)
	}
	closureCountAfter, err := db.Core.NewSelect().Model((*coredb.OrganizationUnitClosure)(nil)).
		Where("root_organization_id = ?", root.ID).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, closureCountBefore, closureCountAfter)
	require.NoError(t, db.Core.NewSelect().Model(&coredb.Repository{}).Where("id = ?", repository.ID).Scan(ctx))
	require.Empty(t, jobClient.recordedInputs())
}

func TestOrganizationUnitStore_DeleteRootRepositoriesSerializesWithRepositoryCreation(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	creator := createOrganizationUnitMemberTestUser(t, ctx, db, "hierarchy-concurrent-delete-creator")
	store := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, &testRepositoryDeletionJobClient{})
	repoStore := coredb.NewRepoStoreWithDB(db)
	rootUUID := uuid.New()
	root, err := store.CreateRoot(ctx, coredb.CreateRootOrganizationInput{
		Organization: &coredb.Organization{Name: "hierarchy-concurrent-root", UUID: rootUUID, UserID: creator.ID, IsRoot: true, IsHierarchical: true},
		Namespace:    &coredb.Namespace{Path: "hierarchy-concurrent-root", UUID: rootUUID.String()}, CreatorUserID: creator.ID,
	})
	require.NoError(t, err)
	child := createChild(t, ctx, store, root, "hierarchy-concurrent-child", nil, 1)
	namespaceLocked := make(chan struct{})
	db.BunDB.AddQueryHook(&repositoryNamespaceLockHook{path: child.Name, locked: namespaceLocked})

	lockConnection, err := db.BunDB.DB.Conn(ctx)
	require.NoError(t, err)
	defer lockConnection.Close()
	_, err = lockConnection.ExecContext(ctx, "BEGIN")
	require.NoError(t, err)
	_, err = lockConnection.ExecContext(ctx, "LOCK TABLE repositories IN ACCESS EXCLUSIVE MODE")
	require.NoError(t, err)
	lockReleased := false
	defer func() {
		if !lockReleased {
			_, _ = lockConnection.ExecContext(ctx, "ROLLBACK")
		}
	}()

	createResult := make(chan error, 1)
	go func() {
		_, createErr := repoStore.CreateRepo(ctx, coredb.Repository{
			Name: "new-repository", Path: child.Name + "/new-repository",
			GitPath: "models_" + child.Name + "/new-repository", RepositoryType: types.ModelRepo,
		})
		createResult <- createErr
	}()
	select {
	case <-namespaceLocked:
	case createErr := <-createResult:
		require.NoError(t, createErr)
		require.FailNow(t, "repository creation finished before acquiring the namespace lock")
	case <-time.After(5 * time.Second):
		require.FailNow(t, "repository creation did not acquire the namespace KEY SHARE lock")
	}

	deleteResult := make(chan error, 1)
	go func() {
		_, deleteErr := store.DeleteRoot(ctx, coredb.DeleteRootOrganizationInput{OrganizationUUID: root.UUID.String()})
		deleteResult <- deleteErr
	}()
	select {
	case deleteErr := <-deleteResult:
		require.NoError(t, deleteErr)
		require.FailNow(t, "hierarchy deletion completed before in-flight repository creation")
	case <-time.After(2 * time.Second):
	}
	_, err = lockConnection.ExecContext(ctx, "COMMIT")
	require.NoError(t, err)
	lockReleased = true
	require.NoError(t, <-createResult)
	require.NoError(t, <-deleteResult)

	exists, err := db.Core.NewSelect().Model((*coredb.Repository)(nil)).Where("path = ?", child.Name+"/new-repository").Exists(ctx)
	require.NoError(t, err)
	require.False(t, exists)
}

// TestOrganizationUnitStore_CreateRootDoesNotOverwriteNamespace verifies a conflicting namespace rolls back the hierarchy transaction.
func TestOrganizationUnitStore_CreateRootDoesNotOverwriteNamespace(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	creator := createOrganizationUnitMemberTestUser(t, ctx, db, "root-namespace-conflict")
	existingUUID := uuid.NewString()
	existingNamespace := &coredb.Namespace{
		Path: "existing-root-namespace", UserID: creator.ID, UUID: existingUUID, NamespaceType: coredb.UserNamespace,
	}
	_, err := db.Core.NewInsert().Model(existingNamespace).Exec(ctx)
	require.NoError(t, err)

	organizationUUID := uuid.New()
	_, err = coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, &testRepositoryDeletionJobClient{}).CreateRoot(ctx, coredb.CreateRootOrganizationInput{
		Organization: &coredb.Organization{
			Name: existingNamespace.Path, UUID: organizationUUID, UserID: creator.ID, IsRoot: true,
		},
		Namespace: &coredb.Namespace{
			Path: existingNamespace.Path, UserID: creator.ID, UUID: organizationUUID.String(),
		},
		CreatorUserID: creator.ID,
	})
	require.Error(t, err)

	var unchanged coredb.Namespace
	require.NoError(t, db.Core.NewSelect().Model(&unchanged).Where("id = ?", existingNamespace.ID).Scan(ctx))
	require.Equal(t, existingUUID, unchanged.UUID)
	require.Equal(t, coredb.UserNamespace, unchanged.NamespaceType)
	organizationExists, err := db.Core.NewSelect().Model((*coredb.Organization)(nil)).
		Where("uuid = ?", organizationUUID).Exists(ctx)
	require.NoError(t, err)
	require.False(t, organizationExists)
}

// TestOrganizationUnitStore_TreeLifecycle verifies real child organizations and closure updates.
func TestOrganizationUnitStore_TreeLifecycle(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "tree")
	store := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, &testRepositoryDeletionJobClient{})

	engineering := createChild(t, ctx, store, root, "engineering", nil, 10)
	platform := createChild(t, ctx, store, root, "platform", &engineering.UUID, 20)
	runtime := createChild(t, ctx, store, root, "runtime", &platform.UUID, 30)
	marketing := createChild(t, ctx, store, root, "marketing", nil, 40)

	record, err := store.FindByUUID(ctx, engineering.UUID)
	require.NoError(t, err)
	require.Equal(t, engineering.UUID, record.OrganizationUUID)
	require.Equal(t, root.UUID.String(), record.RootOrganizationUUID)

	roots, total, err := store.ListRoots(ctx, coredb.ListOrganizationUnitInput{RootOrganizationID: root.ID, Per: 50, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, []string{engineering.UUID, marketing.UUID}, []string{roots[0].UUID, roots[1].UUID})
	require.Equal(t, 1, roots[0].DirectChildrenCount)
	require.Zero(t, roots[0].SubtreeMemberCount)

	children, total, err := store.ListChildren(ctx, coredb.ListOrganizationUnitInput{RootOrganizationID: root.ID, UnitID: record.ID, Per: 50, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, platform.UUID, children[0].UUID)

	deleted, err := store.Delete(ctx, coredb.DeleteOrganizationUnitInput{RootOrganizationID: root.ID, UnitID: record.ID})
	require.NoError(t, err)
	require.Equal(t, 3, deleted.UnitsDeleted)
	require.Equal(t, 3, deleted.OrganizationsDeleted)
	require.ElementsMatch(t, []string{engineering.UUID, platform.UUID, runtime.UUID}, deleted.DeletedOrganizationUUIDs)
	require.ElementsMatch(t, []types.OrganizationHierarchyRelationship{
		{ParentOrganizationUUID: root.UUID.String(), ChildOrganizationUUID: engineering.UUID},
		{ParentOrganizationUUID: engineering.UUID, ChildOrganizationUUID: platform.UUID},
		{ParentOrganizationUUID: platform.UUID, ChildOrganizationUUID: runtime.UUID},
	}, deleted.DeletedHierarchyRelationships)
	deletedCleanupUUIDs := make([]string, 0, len(deleted.DeletedReBACRelationships))
	for _, cleanup := range deleted.DeletedReBACRelationships {
		deletedCleanupUUIDs = append(deletedCleanupUUIDs, cleanup.OrganizationUUID)
		require.NotEmpty(t, cleanup.NamespaceUUID)
	}
	require.ElementsMatch(t, []string{engineering.UUID, platform.UUID, runtime.UUID}, deletedCleanupUUIDs)

	var deletedOrganization coredb.Organization
	require.NoError(t, db.Core.NewSelect().Model(&deletedOrganization).WhereAllWithDeleted().
		Where("uuid = ?", engineering.UUID).Scan(ctx))
	require.False(t, deletedOrganization.DeletedAt.IsZero())
	var deletedNamespace coredb.Namespace
	require.NoError(t, db.Core.NewSelect().Model(&deletedNamespace).WhereAllWithDeleted().
		Where("id = ?", deletedOrganization.NamespaceID).Scan(ctx))
	require.False(t, deletedNamespace.DeletedAt.IsZero())
	var deletedUnit coredb.OrganizationUnit
	require.NoError(t, db.Core.NewSelect().Model(&deletedUnit).WhereAllWithDeleted().
		Where("id = ?", record.ID).Scan(ctx))
	require.NotNil(t, deletedUnit.DeletedAt)
	closureCount, err := db.Core.NewSelect().Model((*coredb.OrganizationUnitClosure)(nil)).
		Where("root_organization_id = ? AND (ancestor_unit_id = ? OR descendant_unit_id = ?)", root.ID, record.ID, record.ID).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, closureCount)
	remaining, total, err := store.ListRoots(ctx, coredb.ListOrganizationUnitInput{RootOrganizationID: root.ID, Per: 50, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, marketing.UUID, remaining[0].UUID)

	replacementUUID := uuid.New()
	replacement, err := store.Create(ctx, coredb.CreateOrganizationUnitInput{
		RootOrganizationID: root.ID,
		Organization: &coredb.Organization{
			Name: deletedOrganization.Name, Nickname: "Replacement Engineering", UUID: replacementUUID,
		},
		Namespace: &coredb.Namespace{
			Path: deletedNamespace.Path, UUID: replacementUUID.String(),
		},
		SortOrder: 50,
		MaxDepth:  types.OrganizationUnitMaxDepth,
	})
	require.NoError(t, err)
	require.NotEqual(t, engineering.UUID, replacement.UUID)
	replacementRecord, err := store.FindByUUID(ctx, replacement.UUID)
	require.NoError(t, err)
	require.NotEqual(t, deletedOrganization.ID, replacementRecord.OrganizationID)
}

// TestOrganizationUnitStore_ChildOrganizationFields verifies editable organization metadata.
func TestOrganizationUnitStore_ChildOrganizationFields(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "fields")
	store := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, &testRepositoryDeletionJobClient{})
	unit := createChild(t, ctx, store, root, "fields-child", nil, 1)
	record, err := store.FindByUUID(ctx, unit.UUID)
	require.NoError(t, err)
	newNickname := "Updated Child"
	newDescription := "updated description"
	newSort := 99
	updated, err := store.Update(ctx, coredb.UpdateOrganizationUnitInput{
		RootOrganizationID: root.ID, OrganizationID: record.OrganizationID, UnitID: record.ID, UnitUUID: unit.UUID,
		Nickname: &newNickname, Description: &newDescription, SortOrder: &newSort,
	})
	require.NoError(t, err)
	require.Equal(t, newNickname, updated.Nickname)
	require.Equal(t, newDescription, updated.Description)
	require.Equal(t, newSort, updated.SortOrder)
}

// TestOrganizationUnitStore_RootOrganizationFields verifies top-level organization metadata can use the unit update path.
func TestOrganizationUnitStore_RootOrganizationFields(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "root-fields")
	store := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, &testRepositoryDeletionJobClient{})
	record, err := store.FindByUUID(ctx, root.UUID.String())
	require.NoError(t, err)
	newNickname := "Updated Root"
	newDescription := "updated root description"
	newSort := 7

	updated, err := store.Update(ctx, coredb.UpdateOrganizationUnitInput{
		RootOrganizationID: root.ID, OrganizationID: root.ID, UnitID: record.ID, UnitUUID: root.UUID.String(),
		Nickname: &newNickname, Description: &newDescription, SortOrder: &newSort,
	})
	require.NoError(t, err)
	require.True(t, updated.IsRoot)
	require.Equal(t, newNickname, updated.Nickname)
	require.Equal(t, newDescription, updated.Description)
	require.Equal(t, newSort, updated.SortOrder)
}

// TestOrganizationUnitStore_CreatorIsRecordOnly verifies creator metadata is not transferable through updates.
func TestOrganizationUnitStore_CreatorIsRecordOnly(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "creator-record")
	creator := createOrganizationUnitMemberTestUser(t, ctx, db, "child-creator")
	store := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, &testRepositoryDeletionJobClient{})
	organizationUUID := uuid.New()
	childOrganization := &coredb.Organization{
		Name: "unit-creator-" + uuid.NewString()[:8], Nickname: "Creator Child", UUID: organizationUUID, UserID: creator.ID, IsRoot: false,
	}
	unit, err := store.Create(ctx, coredb.CreateOrganizationUnitInput{
		RootOrganizationID: root.ID,
		Organization:       childOrganization,
		Namespace:          &coredb.Namespace{Path: childOrganization.Name, UUID: organizationUUID.String()},
		SortOrder:          1,
		MaxDepth:           types.OrganizationUnitMaxDepth,
	})
	require.NoError(t, err)
	record, err := store.FindByUUID(ctx, unit.UUID)
	require.NoError(t, err)

	memberExists, err := db.Core.NewSelect().Model((*coredb.Member)(nil)).
		Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", record.OrganizationID, creator.ID).
		Exists(ctx)
	require.NoError(t, err)
	require.False(t, memberExists)

	newNickname := "Updated Child"
	_, err = store.Update(ctx, coredb.UpdateOrganizationUnitInput{
		RootOrganizationID: root.ID,
		OrganizationID:     record.OrganizationID,
		UnitID:             record.ID,
		UnitUUID:           unit.UUID,
		Nickname:           &newNickname,
	})
	require.NoError(t, err)
	var updatedOrganization coredb.Organization
	err = db.Core.NewSelect().Model(&updatedOrganization).Where("id = ?", record.OrganizationID).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, creator.ID, updatedOrganization.UserID)
	require.Equal(t, newNickname, updatedOrganization.Nickname)
}

func createRootOrganization(t *testing.T, ctx context.Context, db *coredb.DB, suffix string) *coredb.Organization {
	t.Helper()
	organization := &coredb.Organization{Name: "unit-root-" + suffix, Nickname: "Unit Root " + suffix, UUID: uuid.New(), IsRoot: true}
	created, err := coredb.NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, &testRepositoryDeletionJobClient{}).CreateRoot(ctx, coredb.CreateRootOrganizationInput{
		Organization: organization,
		Namespace:    &coredb.Namespace{Path: organization.Name, UUID: organization.UUID.String()},
	})
	require.NoError(t, err)
	return created
}

func createChild(t *testing.T, ctx context.Context, store coredb.OrganizationUnitStore, root *coredb.Organization, name string, parent *string, sortOrder int) *types.OrganizationUnit {
	t.Helper()
	organizationUUID := uuid.New()
	organization := &coredb.Organization{Name: "unit-" + name + "-" + uuid.NewString()[:8], Nickname: name, UUID: organizationUUID, IsRoot: false}
	result, err := store.Create(ctx, coredb.CreateOrganizationUnitInput{
		RootOrganizationID: root.ID, ParentUnitUUID: parent,
		Organization: organization, Namespace: &coredb.Namespace{Path: organization.Name, UUID: organizationUUID.String()},
		SortOrder: sortOrder, MaxDepth: types.OrganizationUnitMaxDepth,
	})
	require.NoError(t, err)
	return result
}
