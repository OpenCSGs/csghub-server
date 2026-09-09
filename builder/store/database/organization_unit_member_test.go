package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	coredb "opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// TestOrganizationUnitMemberStore_UpsertAndRemove verifies organization-scoped idempotent membership writes.
func TestOrganizationUnitMemberStore_UpsertAndRemove(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "membership")
	unitStore := coredb.NewOrganizationUnitStoreWithDB(db)
	unit := createChild(t, ctx, unitStore, root, "membership-child", nil, 1)
	user := createOrganizationUnitMemberTestUser(t, ctx, db, "organization-member")
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)

	result, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationIDForUnit(t, ctx, unitStore, unit.UUID),
		OrganizationUnitID: organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID),
		UserUUIDs:          []string{user.UUID},
		Role:               types.UserWrite,
	})
	require.NoError(t, err)
	require.Equal(t, []string{user.UUID}, result.MembersAdded)

	role, err := store.FindRole(ctx, organizationIDForUnit(t, ctx, unitStore, unit.UUID), user.ID)
	require.NoError(t, err)
	require.Equal(t, types.UserWrite, role)

	result, err = store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationIDForUnit(t, ctx, unitStore, unit.UUID),
		OrganizationUnitID: organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID),
		UserUUIDs:          []string{user.UUID},
		Role:               types.UserRead,
	})
	require.NoError(t, err)
	require.Equal(t, []string{user.UUID}, result.MembersAdded)

	members, total, err := store.ListMembers(ctx, coredb.ListOrganizationMembersInput{
		OrganizationID: organizationIDForUnit(t, ctx, unitStore, unit.UUID), Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, types.UserRead, members[0].UserRole)

	result, err = store.RemoveMembers(ctx, coredb.RemoveOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationIDForUnit(t, ctx, unitStore, unit.UUID),
		OrganizationUnitID: organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID),
		UserUUIDs:          []string{user.UUID},
	})
	require.NoError(t, err)
	require.Equal(t, []string{user.UUID}, result.MembersRemoved)
	_, err = store.FindRole(ctx, organizationIDForUnit(t, ctx, unitStore, unit.UUID), user.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)

	_, err = coredb.NewMemberStoreWithDB(db).Find(ctx, root.ID, user.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

// TestOrganizationUnitMemberStore_UpsertMembersProtectsLastAdmin verifies role replacement cannot downgrade every administrator.
func TestOrganizationUnitMemberStore_UpsertMembersProtectsLastAdmin(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "upsert-last-admin")
	unitStore := coredb.NewOrganizationUnitStoreWithDB(db)
	unit := createChild(t, ctx, unitStore, root, "upsert-last-admin-child", nil, 1)
	organizationID := organizationIDForUnit(t, ctx, unitStore, unit.UUID)
	unitID := organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID)
	firstAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "upsert-first-admin")
	secondAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "upsert-second-admin")
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)

	_, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		UserUUIDs: []string{firstAdmin.UUID, secondAdmin.UUID}, Role: types.UserAdmin,
	})
	require.NoError(t, err)
	_, err = store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		UserUUIDs: []string{secondAdmin.UUID}, Role: types.UserWrite,
	})
	require.NoError(t, err)

	_, err = store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		UserUUIDs: []string{firstAdmin.UUID}, Role: types.UserRead,
	})
	require.ErrorIs(t, err, errorx.ErrLastOrgAdmin)
	role, err := store.FindRole(ctx, organizationID, firstAdmin.ID)
	require.NoError(t, err)
	require.Equal(t, types.UserAdmin, role)
}

// TestOrganizationUnitMemberStore_RemoveMembersProtectsLastAdmin verifies single and batch removals retain an administrator.
func TestOrganizationUnitMemberStore_RemoveMembersProtectsLastAdmin(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "remove-last-admin")
	unitStore := coredb.NewOrganizationUnitStoreWithDB(db)
	unit := createChild(t, ctx, unitStore, root, "remove-last-admin-child", nil, 1)
	organizationID := organizationIDForUnit(t, ctx, unitStore, unit.UUID)
	unitID := organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID)
	firstAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "remove-first-admin")
	secondAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "remove-second-admin")
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)

	_, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		UserUUIDs: []string{firstAdmin.UUID, secondAdmin.UUID}, Role: types.UserAdmin,
	})
	require.NoError(t, err)

	result, err := store.RemoveMembers(ctx, coredb.RemoveOrganizationMembersInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		UserUUIDs: []string{firstAdmin.UUID},
	})
	require.NoError(t, err)
	require.Equal(t, []string{firstAdmin.UUID}, result.MembersRemoved)

	_, err = store.RemoveMembers(ctx, coredb.RemoveOrganizationMembersInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		UserUUIDs: []string{secondAdmin.UUID},
	})
	require.ErrorIs(t, err, errorx.ErrLastOrgAdmin)
	role, err := store.FindRole(ctx, organizationID, secondAdmin.ID)
	require.NoError(t, err)
	require.Equal(t, types.UserAdmin, role)

	_, err = store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		UserUUIDs: []string{firstAdmin.UUID}, Role: types.UserAdmin,
	})
	require.NoError(t, err)
	_, err = store.RemoveMembers(ctx, coredb.RemoveOrganizationMembersInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		UserUUIDs: []string{firstAdmin.UUID, secondAdmin.UUID},
	})
	require.ErrorIs(t, err, errorx.ErrLastOrgAdmin)
	for _, admin := range []*coredb.User{firstAdmin, secondAdmin} {
		role, err = store.FindRole(ctx, organizationID, admin.ID)
		require.NoError(t, err)
		require.Equal(t, types.UserAdmin, role)
	}
}

// TestOrganizationUnitMemberStore_UpdateMemberRole verifies transactional role updates and missing-member errors.
func TestOrganizationUnitMemberStore_UpdateMemberRole(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "role-update")
	unitStore := coredb.NewOrganizationUnitStoreWithDB(db)
	unit := createChild(t, ctx, unitStore, root, "role-update-child", nil, 1)
	organizationID := organizationIDForUnit(t, ctx, unitStore, unit.UUID)
	unitID := organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID)
	admin := createOrganizationUnitMemberTestUser(t, ctx, db, "role-update-admin")
	user := createOrganizationUnitMemberTestUser(t, ctx, db, "role-update-member")
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)

	_, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationID,
		OrganizationUnitID: unitID,
		UserUUIDs:          []string{admin.UUID},
		Role:               types.UserAdmin,
	})
	require.NoError(t, err)
	_, err = store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationID,
		OrganizationUnitID: unitID,
		UserUUIDs:          []string{user.UUID},
		Role:               types.UserRead,
	})
	require.NoError(t, err)

	err = store.UpdateMemberRole(ctx, coredb.UpdateOrganizationMemberRoleInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationID,
		OrganizationUnitID: unitID,
		OrganizationUUID:   unit.UUID,
		UserID:             user.ID,
		Role:               types.UserAdmin,
	})
	require.NoError(t, err)
	role, err := store.FindRole(ctx, organizationID, user.ID)
	require.NoError(t, err)
	require.Equal(t, types.UserAdmin, role)

	writer := createOrganizationUnitMemberTestUser(t, ctx, db, "role-update-writer")
	_, err = store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationID,
		OrganizationUnitID: unitID,
		UserUUIDs:          []string{writer.UUID},
		Role:               types.UserWrite,
	})
	require.NoError(t, err)
	err = store.UpdateMemberRole(ctx, coredb.UpdateOrganizationMemberRoleInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationID,
		OrganizationUnitID: unitID,
		OrganizationUUID:   unit.UUID,
		UserID:             user.ID,
		Role:               types.UserRead,
	})
	require.NoError(t, err)
	role, err = store.FindRole(ctx, organizationID, user.ID)
	require.NoError(t, err)
	require.Equal(t, types.UserRead, role)

	err = store.UpdateMemberRole(ctx, coredb.UpdateOrganizationMemberRoleInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationID,
		OrganizationUnitID: unitID,
		OrganizationUUID:   unit.UUID,
		UserID:             int64(1 << 62),
		Role:               types.UserWrite,
	})
	require.ErrorIs(t, err, errorx.ErrOrganizationMemberNotFound)
}

// TestOrganizationUnitMemberStore_UpdateMemberRoleProtectsLastAdmin verifies only non-final administrators may be downgraded.
func TestOrganizationUnitMemberStore_UpdateMemberRoleProtectsLastAdmin(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "downgrade-last-admin")
	unitStore := coredb.NewOrganizationUnitStoreWithDB(db)
	unit := createChild(t, ctx, unitStore, root, "downgrade-last-admin-child", nil, 1)
	organizationID := organizationIDForUnit(t, ctx, unitStore, unit.UUID)
	unitID := organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID)
	firstAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "downgrade-first-admin")
	secondAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "downgrade-second-admin")
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)

	_, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		UserUUIDs: []string{firstAdmin.UUID, secondAdmin.UUID}, Role: types.UserAdmin,
	})
	require.NoError(t, err)

	err = store.UpdateMemberRole(ctx, coredb.UpdateOrganizationMemberRoleInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		OrganizationUUID: unit.UUID, UserID: secondAdmin.ID,
		Role: types.UserWrite,
	})
	require.NoError(t, err)

	err = store.UpdateMemberRole(ctx, coredb.UpdateOrganizationMemberRoleInput{
		RootOrganizationID: root.ID, OrganizationID: organizationID, OrganizationUnitID: unitID,
		OrganizationUUID: unit.UUID, UserID: firstAdmin.ID,
		Role: types.UserRead,
	})
	require.ErrorIs(t, err, errorx.ErrLastOrgAdmin)
	role, err := store.FindRole(ctx, organizationID, firstAdmin.ID)
	require.NoError(t, err)
	require.Equal(t, types.UserAdmin, role)
}

// TestOrganizationUnitMemberStore_RootMembershipUsesMembersTable verifies top-level memberships use the common table.
func TestOrganizationUnitMemberStore_RootMembershipUsesMembersTable(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "root-membership")
	user := createOrganizationUnitMemberTestUser(t, ctx, db, "root-member")
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)

	result, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     root.ID,
		UserUUIDs:          []string{user.UUID},
		Role:               types.UserAdmin,
	})
	require.NoError(t, err)
	require.Equal(t, []string{user.UUID}, result.MembersAdded)

	var relation coredb.Member
	require.NoError(t, db.Core.NewSelect().Model(&relation).
		Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", root.ID, user.ID).
		Scan(ctx))
	require.Equal(t, string(types.UserAdmin), relation.Role)
}

// TestOrganizationUnitMemberStore_ListRootMembersExcludesChildMembers verifies root queries only return direct root memberships.
func TestOrganizationUnitMemberStore_ListRootMembersExcludesChildMembers(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "member-list-root")
	unitStore := coredb.NewOrganizationUnitStoreWithDB(db)
	child := createChild(t, ctx, unitStore, root, "member-list-child", nil, 1)
	childOrganizationID := organizationIDForUnit(t, ctx, unitStore, child.UUID)
	childUnitID := organizationUnitIDForUnit(t, ctx, unitStore, child.UUID)
	rootUser := createOrganizationUnitMemberTestUser(t, ctx, db, "root-direct-member")
	childUser := createOrganizationUnitMemberTestUser(t, ctx, db, "child-direct-member")
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)

	_, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     root.ID,
		UserUUIDs:          []string{rootUser.UUID},
		Role:               types.UserAdmin,
	})
	require.NoError(t, err)
	_, err = store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     childOrganizationID,
		OrganizationUnitID: childUnitID,
		UserUUIDs:          []string{childUser.UUID},
		Role:               types.UserWrite,
	})
	require.NoError(t, err)

	members, total, err := store.ListMembers(ctx, coredb.ListOrganizationMembersInput{
		OrganizationID: root.ID, Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, members, 1)
	require.Equal(t, rootUser.UUID, members[0].UserUUID)
	require.Equal(t, types.UserAdmin, members[0].UserRole)
}

// TestOrganizationUnitMemberStore_ListMembersFiltersByRole verifies role filters affect both rows and totals.
func TestOrganizationUnitMemberStore_ListMembersFiltersByRole(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "member-role-filter")
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)

	usersByRole := map[types.UserRole]*coredb.User{
		types.UserAdmin: createOrganizationUnitMemberTestUser(t, ctx, db, "member-role-filter-admin"),
		types.UserWrite: createOrganizationUnitMemberTestUser(t, ctx, db, "member-role-filter-write"),
		types.UserRead:  createOrganizationUnitMemberTestUser(t, ctx, db, "member-role-filter-read"),
	}
	for role, user := range usersByRole {
		_, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
			RootOrganizationID: root.ID,
			OrganizationID:     root.ID,
			UserUUIDs:          []string{user.UUID},
			Role:               role,
		})
		require.NoError(t, err)
	}

	tests := []struct {
		name string
		role types.UserRole
		want int
	}{
		{name: "all roles", want: 3},
		{name: "administrators", role: types.UserAdmin, want: 1},
		{name: "writers", role: types.UserWrite, want: 1},
		{name: "readers", role: types.UserRead, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			members, total, err := store.ListMembers(ctx, coredb.ListOrganizationMembersInput{
				OrganizationID: root.ID,
				Role:           test.role,
				Per:            20,
				Page:           1,
			})
			require.NoError(t, err)
			require.Equal(t, test.want, total)
			require.Len(t, members, test.want)
			if test.role != "" {
				require.Equal(t, test.role, members[0].UserRole)
			}
		})
	}

	members, total, err := store.ListMembers(ctx, coredb.ListOrganizationMembersInput{
		OrganizationID: root.ID,
		Search:         "write",
		Role:           types.UserAdmin,
		Per:            20,
		Page:           1,
	})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, members)
}

// TestOrganizationStore_HierarchyMembershipQueries verifies organization membership reads use common members.
func TestOrganizationStore_HierarchyMembershipQueries(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	root := createRootOrganization(t, ctx, db, "hierarchy-membership-query")
	firstUser := createOrganizationUnitMemberTestUser(t, ctx, db, "hierarchy-membership-first")
	secondUser := createOrganizationUnitMemberTestUser(t, ctx, db, "hierarchy-membership-second")
	membershipStore := coredb.NewOrganizationUnitMemberStoreWithDB(db)

	for _, userUUID := range []string{firstUser.UUID, secondUser.UUID} {
		_, err := membershipStore.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
			RootOrganizationID: root.ID,
			OrganizationID:     root.ID,
			UserUUIDs:          []string{userUUID},
			Role:               types.UserAdmin,
		})
		require.NoError(t, err)
	}

	organizationStore := coredb.NewHierarchyOrgStoreWithDB(db)
	organizations, err := organizationStore.GetUserBelongOrgs(ctx, firstUser.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 1)
	require.Equal(t, root.ID, organizations[0].ID)
	require.Equal(t, string(types.UserAdmin), organizations[0].Role)

	organizations, total, err := organizationStore.SearchUserBelongOrgs(ctx, firstUser.ID, "", 20, 1, "", "", string(types.UserAdmin), "")
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, organizations, 1)
	require.Equal(t, root.ID, organizations[0].ID)

	// The legacy owner filter is an administrator-role compatibility alias.
	organizations, total, err = organizationStore.SearchUserBelongOrgs(ctx, firstUser.ID, "", 20, 1, "", "", "owner", "")
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, organizations, 1)
	require.Equal(t, root.ID, organizations[0].ID)

	sharedOrganizationIDs, err := organizationStore.GetSharedOrgIDs(ctx, []int64{firstUser.ID, secondUser.ID})
	require.NoError(t, err)
	require.Equal(t, []int64{root.ID}, sharedOrganizationIDs)
}

// TestOrganizationUnitMemberStore_UserDeletionCleansRelations verifies common user cleanup removes memberships.
func TestOrganizationUnitMemberStore_UserDeletionCleansRelations(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "user-delete")
	unitStore := coredb.NewOrganizationUnitStoreWithDB(db)
	unit := createChild(t, ctx, unitStore, root, "user-delete-child", nil, 1)
	user := createOrganizationUnitMemberTestUser(t, ctx, db, "deleted-member")
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)
	_, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationIDForUnit(t, ctx, unitStore, unit.UUID),
		OrganizationUnitID: organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID),
		UserUUIDs:          []string{user.UUID},
		Role:               types.UserRead,
	})
	require.NoError(t, err)

	require.NoError(t, coredb.NewUserStoreWithDB(db).SoftDeleteUserAndRelations(ctx, *user, types.CloseAccountReq{}))
	active, err := db.Core.NewSelect().Model((*coredb.Member)(nil)).
		Where("user_id = ? AND deleted_at IS NULL", user.ID).Exists(ctx)
	require.NoError(t, err)
	require.False(t, active)
}

func organizationIDForUnit(t *testing.T, ctx context.Context, store coredb.OrganizationUnitStore, unitUUID string) int64 {
	t.Helper()
	unit, err := store.FindByUUID(ctx, unitUUID)
	require.NoError(t, err)
	return unit.OrganizationID
}

func organizationUnitIDForUnit(t *testing.T, ctx context.Context, store coredb.OrganizationUnitStore, unitUUID string) *int64 {
	t.Helper()
	unit, err := store.FindByUUID(ctx, unitUUID)
	require.NoError(t, err)
	return &unit.ID
}

func createOrganizationUnitMemberTestUser(t *testing.T, ctx context.Context, db *coredb.DB, username string) *coredb.User {
	t.Helper()
	user := &coredb.User{Username: username, UUID: uuid.NewString(), Email: username + "@example.com", Password: "test-password"}
	require.NoError(t, coredb.NewUserStoreWithDB(db).Create(ctx, user, &coredb.Namespace{Path: username}))
	return user
}
