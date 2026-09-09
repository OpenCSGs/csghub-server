package database_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	coredb "opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

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
	store := coredb.NewOrganizationUnitStoreWithDB(db)
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
	root, err := coredb.NewOrganizationUnitStoreWithDB(db).CreateRoot(ctx, coredb.CreateRootOrganizationInput{
		Organization: &coredb.Organization{Name: "root-delete", Nickname: "Root Delete", UUID: rootUUID, UserID: creator.ID, IsRoot: true, IsUnit: true},
		Namespace:    &coredb.Namespace{Path: "root-delete", UUID: rootUUID.String()}, CreatorUserID: creator.ID,
	})
	require.NoError(t, err)
	store := coredb.NewOrganizationUnitStoreWithDB(db)
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
	_, err = coredb.NewOrganizationUnitStoreWithDB(db).CreateRoot(ctx, coredb.CreateRootOrganizationInput{
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
	store := coredb.NewOrganizationUnitStoreWithDB(db)

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
	store := coredb.NewOrganizationUnitStoreWithDB(db)
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
	store := coredb.NewOrganizationUnitStoreWithDB(db)
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
	store := coredb.NewOrganizationUnitStoreWithDB(db)
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
	created, err := coredb.NewOrganizationUnitStoreWithDB(db).CreateRoot(ctx, coredb.CreateRootOrganizationInput{
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
