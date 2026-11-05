package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	coredb "opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database/migrations"
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
	unitStore := coredb.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, tests.NewFixtureRebacTupleJobClient(db))
	unit := createChild(t, ctx, unitStore, root, "membership-child", nil, 1)
	user := createOrganizationUnitMemberTestUser(t, ctx, db, "organization-member")
	store := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

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

	_, err = coredb.NewMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db)).Find(ctx, root.ID, user.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

// TestOrganizationUnitMemberStore_UpsertMembersProtectsLastAdmin verifies role replacement cannot downgrade every administrator.
func TestOrganizationUnitMemberStore_UpsertMembersProtectsLastAdmin(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "upsert-last-admin")
	unitStore := coredb.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, tests.NewFixtureRebacTupleJobClient(db))
	unit := createChild(t, ctx, unitStore, root, "upsert-last-admin-child", nil, 1)
	organizationID := organizationIDForUnit(t, ctx, unitStore, unit.UUID)
	unitID := organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID)
	firstAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "upsert-first-admin")
	secondAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "upsert-second-admin")
	store := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

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
	unitStore := coredb.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, tests.NewFixtureRebacTupleJobClient(db))
	unit := createChild(t, ctx, unitStore, root, "remove-last-admin-child", nil, 1)
	organizationID := organizationIDForUnit(t, ctx, unitStore, unit.UUID)
	unitID := organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID)
	firstAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "remove-first-admin")
	secondAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "remove-second-admin")
	store := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

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
	unitStore := coredb.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, tests.NewFixtureRebacTupleJobClient(db))
	unit := createChild(t, ctx, unitStore, root, "role-update-child", nil, 1)
	organizationID := organizationIDForUnit(t, ctx, unitStore, unit.UUID)
	unitID := organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID)
	admin := createOrganizationUnitMemberTestUser(t, ctx, db, "role-update-admin")
	user := createOrganizationUnitMemberTestUser(t, ctx, db, "role-update-member")
	store := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

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
	unitStore := coredb.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, tests.NewFixtureRebacTupleJobClient(db))
	unit := createChild(t, ctx, unitStore, root, "downgrade-last-admin-child", nil, 1)
	organizationID := organizationIDForUnit(t, ctx, unitStore, unit.UUID)
	unitID := organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID)
	firstAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "downgrade-first-admin")
	secondAdmin := createOrganizationUnitMemberTestUser(t, ctx, db, "downgrade-second-admin")
	store := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

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
	store := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

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
	unitStore := coredb.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, tests.NewFixtureRebacTupleJobClient(db))
	child := createChild(t, ctx, unitStore, root, "member-list-child", nil, 1)
	childOrganizationID := organizationIDForUnit(t, ctx, unitStore, child.UUID)
	childUnitID := organizationUnitIDForUnit(t, ctx, unitStore, child.UUID)
	rootUser := createOrganizationUnitMemberTestUser(t, ctx, db, "root-direct-member")
	childUser := createOrganizationUnitMemberTestUser(t, ctx, db, "child-direct-member")
	store := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

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
	store := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

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
	setOrgStoreTestDB(t, db)
	ctx := context.Background()

	root := createRootOrganization(t, ctx, db, "hierarchy-membership-query")
	firstUser := createOrganizationUnitMemberTestUser(t, ctx, db, "hierarchy-membership-first")
	secondUser := createOrganizationUnitMemberTestUser(t, ctx, db, "hierarchy-membership-second")
	membershipStore := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

	for _, userUUID := range []string{firstUser.UUID, secondUser.UUID} {
		_, err := membershipStore.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
			RootOrganizationID: root.ID,
			OrganizationID:     root.ID,
			UserUUIDs:          []string{userUUID},
			Role:               types.UserAdmin,
		})
		require.NoError(t, err)
	}

	organizationStore := coredb.NewOrgStore(true, nil)
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
	unitStore := coredb.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, tests.NewFixtureRebacTupleJobClient(db))
	unit := createChild(t, ctx, unitStore, root, "user-delete-child", nil, 1)
	user := createOrganizationUnitMemberTestUser(t, ctx, db, "deleted-member")
	store := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))
	_, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{
		RootOrganizationID: root.ID,
		OrganizationID:     organizationIDForUnit(t, ctx, unitStore, unit.UUID),
		OrganizationUnitID: organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID),
		UserUUIDs:          []string{user.UUID},
		Role:               types.UserRead,
	})
	require.NoError(t, err)

	require.NoError(t, coredb.NewUserStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db)).SoftDeleteUserAndRelations(ctx, *user, types.CloseAccountReq{}))
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
	require.NoError(t, coredb.NewUserStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db)).Create(ctx, user, &coredb.Namespace{Path: username}))
	return user
}

// TestOrganizationMemberSearchStore verifies text and phone matching, pagination, memberships, and exclusion using the existing schema.
func TestOrganizationMemberSearchStore(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "member-search")
	// Fixture writes require transactional permission jobs just like production writes.
	tupleJobs := tests.NewFixtureRebacTupleJobClient(db)
	units := coredb.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, tupleJobs)
	research := createChild(t, ctx, units, root, "研发中心", nil, 1)
	it := createChild(t, ctx, units, root, "IT研发部", &research.UUID, 2)
	sales := createChild(t, ctx, units, root, "销售一组", nil, 3)
	lex := createOrganizationUnitMemberTestUser(t, ctx, db, "Lex")
	lex.NickName, lex.Email, lex.Phone, lex.Avatar = "大师兄", "123@gmail.com", "13812345678", "https://example.com/头像.png"
	_, err := db.Core.NewUpdate().Model(lex).Column("name", "email", "phone", "avatar").WherePK().Exec(ctx)
	require.NoError(t, err)
	store := coredb.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, tupleJobs)
	for _, unit := range []*types.OrganizationUnit{research, it, sales} {
		_, err := store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{RootOrganizationID: root.ID, OrganizationID: organizationIDForUnit(t, ctx, units, unit.UUID), OrganizationUnitID: organizationUnitIDForUnit(t, ctx, units, unit.UUID), UserUUIDs: []string{lex.UUID}, Role: types.UserRead})
		require.NoError(t, err)
	}
	for _, keyword := range []string{"lex", "LEX", " ex ", "大师兄", "师兄", "123@GMAIL", "gmail.com", "13812345678", "345678", " 345678 "} {
		t.Run(keyword, func(t *testing.T) {
			data, total, err := store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: keyword, Per: 1, Page: 1})
			require.NoError(t, err)
			require.Equal(t, 1, total)
			require.Len(t, data, 1)
			require.Equal(t, lex.UUID, data[0].UserUUID)
			require.Equal(t, "大师兄", data[0].Nickname)
			require.Equal(t, "123@gmail.com", data[0].Email)
			require.Equal(t, lex.Avatar, data[0].Avatar)
			require.Len(t, data[0].Organizations, 3)
			require.Equal(t, []string{"研发中心", "IT研发部", "销售一组"}, []string{data[0].Organizations[0].Nickname, data[0].Organizations[1].Nickname, data[0].Organizations[2].Nickname})
			require.Equal(t, types.UserRead, data[0].Organizations[0].Role)
		})
	}
	// The root itself is also a searchable organization; an ancestor is returned only for direct membership.
	_, err = store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{RootOrganizationID: root.ID, OrganizationID: root.ID, UserUUIDs: []string{lex.UUID}, Role: types.UserWrite})
	require.NoError(t, err)
	data, total, err := store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: "lex", Per: 1, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, data[0].Organizations, 4)
	data, total, err = store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: "lex", Per: 1, Page: 2})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Empty(t, data)
	require.NotNil(t, data)
	// Pages around Bun's int32 offset boundary and native integer overflow must remain empty.
	for _, page := range []int{21474837, 21474838, 42949674, int(^uint(0) >> 1)} {
		t.Run(fmt.Sprintf("out-of-range page %d", page), func(t *testing.T) {
			data, total, err := store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: "lex", Per: 100, Page: page})
			require.NoError(t, err)
			require.Equal(t, 1, total)
			require.Empty(t, data)
			require.NotNil(t, data)
		})
	}

	// A second user must occupy another page, even when the first has multiple memberships.
	other := createOrganizationUnitMemberTestUser(t, ctx, db, "zlex")
	_, err = db.Core.NewUpdate().Model(other).Set("phone = ?", "13887654321").WherePK().Exec(ctx)
	require.NoError(t, err)
	_, err = store.UpsertMembers(ctx, coredb.AddOrganizationMembersInput{RootOrganizationID: root.ID, OrganizationID: root.ID, UserUUIDs: []string{other.UUID}, Role: types.UserRead})
	require.NoError(t, err)
	for _, keyword := range []string{"lex", "138"} {
		data, total, err = store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: keyword, Per: 1, Page: 2})
		require.NoError(t, err)
		require.Equal(t, 2, total)
		require.Equal(t, other.UUID, data[0].UserUUID)
		require.Len(t, data[0].Organizations, 1)
	}

	// Wildcards and the chosen escape character are literal characters in phone searches too.
	_, err = db.Core.NewUpdate().Model(other).Set("phone = ?", "literal%_!phone").WherePK().Exec(ctx)
	require.NoError(t, err)
	for _, keyword := range []string{"%", "_", "!", "%_!"} {
		data, total, err := store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: keyword, Per: 20, Page: 1})
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Equal(t, other.UUID, data[0].UserUUID)
	}
	// Active users without any direct membership are excluded.
	outsider := createOrganizationUnitMemberTestUser(t, ctx, db, "lex-outsider")
	_, err = db.Core.NewUpdate().Model(outsider).Set("phone = ?", lex.Phone).WherePK().Exec(ctx)
	require.NoError(t, err)
	data, total, err = store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: lex.Phone, Per: 20, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, lex.UUID, data[0].UserUUID)
	data, total, err = store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: "lex", Per: 20, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, data, 2)

	// Deleted units, organizations, and memberships independently remove their corresponding associations.
	_, err = db.Core.NewUpdate().Model((*coredb.OrganizationUnit)(nil)).Set("deleted_at = CURRENT_TIMESTAMP").Where("organization_id = ?", organizationIDForUnit(t, ctx, units, research.UUID)).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewUpdate().Model((*coredb.Organization)(nil)).Set("deleted_at = CURRENT_TIMESTAMP").Where("id = ?", organizationIDForUnit(t, ctx, units, it.UUID)).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewDelete().Model((*coredb.Member)(nil)).Where("organization_id = ? AND user_id = ?", organizationIDForUnit(t, ctx, units, sales.UUID), lex.ID).Exec(ctx)
	require.NoError(t, err)
	data, total, err = store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: "lex", Per: 20, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, data[0].Organizations, 1)
	require.Equal(t, root.UUID.String(), data[0].Organizations[0].UUID)
	_, err = db.Core.NewDelete().Model(other).WherePK().Exec(ctx)
	require.NoError(t, err)
	_, total, err = store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: "lex", Per: 20, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	_, err = db.Core.NewDelete().Model(root).WherePK().Exec(ctx)
	require.NoError(t, err)
	data, total, err = store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: "lex", Per: 20, Page: 1})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, data)
}

// TestOrganizationMemberSearchStorePagination verifies large valid offsets and rejects invalid arithmetic inputs.
func TestOrganizationMemberSearchStorePagination(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "pagination-arithmetic")
	recorder := &memberSearchQueryRecorder{}
	db.BunDB.AddQueryHook(recorder)
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)
	offsetPattern := regexp.MustCompile(`(?s)OFFSET \((CASE WHEN .*? END)\)`)
	maxInt := int(^uint(0) >> 1)
	for _, tc := range []struct {
		name       string
		total      int64
		per        int
		page       int64
		wantOffset int64
		invalid    bool
	}{
		{name: "below int32 boundary", total: math.MaxInt32, per: 1, page: math.MaxInt32, wantOffset: math.MaxInt32 - 1},
		{name: "at int32 boundary", total: math.MaxInt32 + 1, per: 1, page: math.MaxInt32 + 1, wantOffset: math.MaxInt32},
		{name: "above int32 boundary", total: math.MaxInt32 + 2, per: 1, page: math.MaxInt32 + 2, wantOffset: math.MaxInt32 + 1},
		{name: "offset wraps to positive", total: 4294967301, per: 100, page: 42949674, wantOffset: 4294967300},
		{name: "largest in-range page", total: int64(maxInt), per: 1, page: int64(maxInt), wantOffset: int64(maxInt) - 1},
		{name: "empty search results", total: 0, per: 100, page: int64(maxInt)},
		{name: "out-of-range page", total: 1, per: 100, page: 21474838, wantOffset: 1},
		{name: "out-of-range multiplication exceeds int64", total: 1, per: maxInt, page: int64(maxInt), wantOffset: 1},
		{name: "largest per on first page", total: int64(maxInt), per: maxInt, page: 1},
		{name: "zero per", per: 0, page: 1, invalid: true},
		{name: "negative per", per: -1, page: 1, invalid: true},
		{name: "zero page", per: 1, page: 0, invalid: true},
		{name: "negative page", per: 1, page: -1, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.total > int64(maxInt) || tc.page > int64(maxInt) {
				t.Skip("pagination case requires a 64-bit integer")
			}
			recorder.queries = nil
			data, total, err := store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: "le", Per: tc.per, Page: int(tc.page)})
			if tc.invalid {
				require.ErrorIs(t, err, errorx.ErrReqParamInvalid)
				require.Empty(t, recorder.queries)
				return
			}
			require.NoError(t, err)
			require.Zero(t, total)
			require.NotNil(t, data)
			require.Empty(t, data)
			require.Len(t, recorder.queries, 1)
			expression := offsetPattern.FindStringSubmatch(recorder.queries[0])
			require.Len(t, expression, 2)
			// Execute the Store's actual SQL arithmetic with large totals, without billions of fixture rows.
			var offset int64
			require.NoError(t, db.Core.NewRaw("SELECT "+expression[1]+" FROM (SELECT ?::bigint AS total) t", tc.total).Scan(ctx, &offset))
			require.Equal(t, tc.wantOffset, offset)
		})
	}
}

// memberSearchQueryRecorder captures the actual production query for plan and cache assertions.
type memberSearchQueryRecorder struct {
	queries []string
}

// BeforeQuery leaves the request context unchanged.
func (h *memberSearchQueryRecorder) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

// AfterQuery records Store SQL without retaining its result data.
func (h *memberSearchQueryRecorder) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	h.queries = append(h.queries, event.Query)
}

// TestOrganizationMemberSearchIndexMigration exercises concurrent DDL in an isolated database without txdb.
func TestOrganizationMemberSearchIndexMigration(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	for _, migration := range migrations.Migrations.Sorted() {
		if migration.Comment != "add_org_member_search_combined_index" {
			continue
		}
		// The cloned template already has the index; remove it and its extension to test a fresh install.
		require.NoError(t, migration.Down(ctx, db.BunDB, nil))
		_, err := db.BunDB.ExecContext(ctx, "DROP EXTENSION pg_trgm")
		require.NoError(t, err)
		var firstOID int64
		t.Run("first build installs extension and valid index", func(t *testing.T) {
			require.NoError(t, migration.Up(ctx, db.BunDB, nil))
			firstOID = requireValidMemberSearchIndex(t, ctx, db)
		})
		t.Run("repeat preserves existing valid index", func(t *testing.T) {
			require.NoError(t, migration.Up(ctx, db.BunDB, nil))
			require.Equal(t, firstOID, requireValidMemberSearchIndex(t, ctx, db))
		})
		t.Run("failed concurrent build is replaced", func(t *testing.T) {
			require.NoError(t, migration.Down(ctx, db.BunDB, nil))
			createOrganizationUnitMemberTestUser(t, ctx, db, "invalid-index-first")
			createOrganizationUnitMemberTestUser(t, ctx, db, "invalid-index-second")
			// A duplicate constant guarantees a real failed concurrent build and an invalid catalog entry.
			_, err := db.BunDB.ExecContext(ctx, "CREATE UNIQUE INDEX CONCURRENTLY users_member_search_combined_idx ON users ((1))")
			require.Error(t, err)
			var invalidOID int64
			var valid bool
			require.NoError(t, db.BunDB.QueryRowContext(ctx, `SELECT indexrelid::bigint, indisvalid FROM pg_index
				WHERE indexrelid = 'users_member_search_combined_idx'::regclass`).Scan(&invalidOID, &valid))
			require.False(t, valid)
			require.NoError(t, migration.Up(ctx, db.BunDB, nil))
			require.NotEqual(t, invalidOID, requireValidMemberSearchIndex(t, ctx, db))
		})
		t.Run("rebuilt index supports actual search query", func(t *testing.T) {
			requireMemberSearchIndexEligible(t, ctx, db)
		})
		t.Run("rollback is repeatable and retains extension", func(t *testing.T) {
			for range 2 {
				require.NoError(t, migration.Down(ctx, db.BunDB, nil))
				var exists, extension bool
				require.NoError(t, db.BunDB.QueryRowContext(ctx, `SELECT
					to_regclass('users_member_search_combined_idx') IS NOT NULL,
					EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm')`).Scan(&exists, &extension))
				require.False(t, exists)
				require.True(t, extension)
			}
		})
		return
	}
	t.Fatal("combined search index migration was not registered")
}

// requireValidMemberSearchIndex checks the rebuilt index's validity, method, predicate, and identity.
func requireValidMemberSearchIndex(t *testing.T, ctx context.Context, db *coredb.DB) int64 {
	t.Helper()
	var oid int64
	var valid bool
	var method, definition string
	require.NoError(t, db.BunDB.QueryRowContext(ctx, `SELECT i.indexrelid::bigint, i.indisvalid, am.amname, pg_get_indexdef(i.indexrelid)
		FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid JOIN pg_am am ON am.oid = c.relam
		WHERE i.indexrelid = 'users_member_search_combined_idx'::regclass AND i.indrelid = 'users'::regclass`).
		Scan(&oid, &valid, &method, &definition))
	require.True(t, valid)
	require.Equal(t, "gin", method)
	require.Contains(t, definition, "gin_trgm_ops")
	require.Contains(t, definition, "WHERE (deleted_at IS NULL)")
	return oid
}

// requireMemberSearchIndexEligible explains real Store SQL on a pinned connection with diagnostic settings only.
func requireMemberSearchIndexEligible(t *testing.T, ctx context.Context, db *coredb.DB) {
	t.Helper()
	root := createRootOrganization(t, ctx, db, "index-search")
	recorder := &memberSearchQueryRecorder{}
	db.BunDB.AddQueryHook(recorder)
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)
	_, _, err := store.SearchMembers(ctx, coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: "lex", Per: 20, Page: 1})
	require.NoError(t, err)
	query := recorder.queries[0]
	conn, err := db.BunDB.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	// SET LOCAL needs an explicit transaction on this non-txdb connection and must not affect concurrent DDL.
	require.NoError(t, conn.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
			return err
		}
		var plan []string
		if err := tx.NewRaw("EXPLAIN (COSTS OFF) "+query).Scan(ctx, &plan); err != nil {
			return err
		}
		require.Contains(t, strings.Join(plan, "\n"), "users_member_search_combined_idx")
		return nil
	}))
}

// memberSearchRouteMode reads the production routing query's decision in the same test transaction.
func memberSearchRouteMode(t *testing.T, ctx context.Context, db *coredb.DB, query string) string {
	t.Helper()
	var mode string
	require.NoError(t, db.Core.NewRaw("SELECT route->>'mode' FROM (?) AS routed(route)", bun.Safe(query)).Scan(ctx, &mode))
	return mode
}

// TestOrganizationMemberSearchStoreRoutingIntegration covers both probe boundaries and complete result semantics.
func TestOrganizationMemberSearchStoreRoutingIntegration(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	root := createRootOrganization(t, ctx, db, "routing-search")
	units := coredb.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, tests.NewFixtureRebacTupleJobClient(db))
	first := createChild(t, ctx, units, root, "routing-first", nil, 1)
	second := createChild(t, ctx, units, root, "routing-second", nil, 2)
	orgIDs := []int64{root.ID, organizationIDForUnit(t, ctx, units, first.UUID), organizationIDForUnit(t, ctx, units, second.UUID)}
	users := make([]coredb.User, 1000)
	for i := range users {
		users[i] = coredb.User{Username: fmt.Sprintf("routing-%04d", i), UUID: uuid.NewString(), Email: fmt.Sprintf("routing-%04d@gmail.com", i), Password: "test-password", Phone: "13800000000"}
	}
	users[0].NickName, users[0].Phone = "大师兄", "prefixABC%_!suffix"
	require.NoError(t, db.Core.NewInsert().Model(&users).Returning("id").Scan(ctx))
	members := make([]coredb.Member, 0, 3000)
	for _, user := range users {
		for _, orgID := range orgIDs {
			members = append(members, coredb.Member{OrganizationID: orgID, UserID: user.ID, Role: string(types.UserRead)})
		}
	}
	_, err := db.Core.NewInsert().Model(&members).Exec(ctx)
	require.NoError(t, err)
	recorder := &memberSearchQueryRecorder{}
	db.BunDB.AddQueryHook(recorder)
	store := coredb.NewOrganizationUnitMemberStoreWithDB(db)
	input := coredb.SearchOrganizationMembersInput{RootOrganizationID: root.ID, Search: "gmail", Per: 20, Page: 1}
	rows, total, err := store.SearchMembers(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 1000, total)
	require.Len(t, rows, 20)
	require.Equal(t, "small_scope", memberSearchRouteMode(t, ctx, db, recorder.queries[0]))
	require.Len(t, rows[0].Organizations, 3)
	// One more user with three direct memberships exceeds the bounded scope sample.
	last := createOrganizationUnitMemberTestUser(t, ctx, db, "routing-last")
	_, err = db.Core.NewUpdate().Model(last).Set("email = ?", "routing-last@gmail.com").WherePK().Exec(ctx)
	require.NoError(t, err)
	for _, orgID := range orgIDs {
		_, err = db.Core.NewInsert().Model(&coredb.Member{OrganizationID: orgID, UserID: last.ID, Role: string(types.UserRead)}).Exec(ctx)
		require.NoError(t, err)
	}
	// Exactly 1000 candidates take the selective path; 1001 candidates fall back without truncating total.
	_, err = db.Core.NewUpdate().Model(&users[999]).Set("email = ?", "routing-0999@example.com").WherePK().Exec(ctx)
	require.NoError(t, err)
	recorder.queries = nil
	_, total, err = store.SearchMembers(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 1000, total)
	require.Equal(t, "selective_index", memberSearchRouteMode(t, ctx, db, recorder.queries[0]))
	_, err = db.Core.NewUpdate().Model(&users[999]).Set("email = ?", "routing-0999@gmail.com").WherePK().Exec(ctx)
	require.NoError(t, err)
	recorder.queries = nil
	rows, total, err = store.SearchMembers(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 1001, total)
	require.Len(t, rows, 20)
	require.Equal(t, "broad_fallback", memberSearchRouteMode(t, ctx, db, recorder.queries[0]))
	_, err = db.Core.NewDelete().Model(last).WherePK().Exec(ctx)
	require.NoError(t, err)
	recorder.queries = nil
	_, total, err = store.SearchMembers(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 1000, total)
	require.True(t, strings.HasPrefix(recorder.queries[0], "WITH matched AS MATERIALIZED"))
	outsider := createOrganizationUnitMemberTestUser(t, ctx, db, "routing-outsider")
	_, err = db.Core.NewUpdate().Model(outsider).Set("name = ?", "大师兄").Set("phone = ?", users[0].Phone).WherePK().Exec(ctx)
	require.NoError(t, err)
	// Selective queries retain literal wildcards, phone case sensitivity, and stable large-page totals.
	for _, keyword := range []string{"大师兄", "ABC%_!", "abc%_!", "prefixABC", "routing-0000@GMAIL"} {
		input.Search, input.Page = keyword, 1
		rows, total, err = store.SearchMembers(ctx, input)
		require.NoError(t, err)
		if keyword == "abc%_!" {
			require.Zero(t, total)
			continue
		}
		require.Equal(t, 1, total)
		require.Equal(t, users[0].UUID, rows[0].UserUUID)
		require.Len(t, rows[0].Organizations, 3)
		input.Page = int(^uint(0) >> 1)
		rows, total, err = store.SearchMembers(ctx, input)
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.NotNil(t, rows)
		require.Empty(t, rows)
	}
	// A match spanning the concatenation separator is a candidate, not a four-field match.
	input.Search, input.Page = "0000\n大师兄", 1
	rows, total, err = store.SearchMembers(ctx, input)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, rows)
	// Deleted users and matching global candidates outside the tree must never become results.
	_, err = db.Core.NewDelete().Model(&users[0]).WherePK().Exec(ctx)
	require.NoError(t, err)
	input.Search = "大师兄"
	rows, total, err = store.SearchMembers(ctx, input)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, rows)
}
