package database_test

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestOrganizationStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	uuid := uuid.New()

	store := database.NewOrgStoreWithDB(db)
	err := store.Create(ctx, &database.Organization{
		Name:     "o1",
		Nickname: "o1_nickname",
		UUID:     uuid,
	}, &database.Namespace{Path: "o1"})
	require.Nil(t, err)

	//search with name
	orgs, total, err := store.Search(ctx, "o1", 10, 1, "", "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "o1", orgs[0].Name)
	require.Equal(t, uuid, orgs[0].UUID)
	//search with nickname
	orgs, total, err = store.Search(ctx, "nickname", 10, 1, "", "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "o1_nickname", orgs[0].Nickname)
	//empty search second page
	orgs, total, err = store.Search(ctx, "nickname", 10, 2, "", "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Empty(t, orgs)

	org := &database.Organization{}
	err = db.Core.NewSelect().Model(org).Where("path=?", "o1").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "o1", org.Name)
	ns := &database.Namespace{}
	err = db.Core.NewSelect().Model(ns).Where("path=?", "o1").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "o1", ns.Path)
	require.Equal(t, database.OrgNamespace, ns.NamespaceType)

	orgv, err := store.FindByPath(ctx, "o1")
	require.Nil(t, err)
	require.Equal(t, "o1", orgv.Name)

	exist, err := store.Exists(ctx, "o1")
	require.Nil(t, err)
	require.True(t, exist)
	exist, err = store.Exists(ctx, "bar")
	require.Nil(t, err)
	require.False(t, exist)

	org.Homepage = "abc"
	err = store.Update(ctx, org)
	require.Nil(t, err)
	org = &database.Organization{}
	err = db.Core.NewSelect().Model(org).Where("path=?", "o1").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "abc", org.Homepage)

	owner := &database.User{Username: "u1"}
	err = db.Core.NewInsert().Model(owner).Scan(ctx, owner)
	require.Nil(t, err)

	member := &database.Member{
		OrganizationID: org.ID,
		UserID:         321,
		Role:           string(types.UserRead),
	}

	err = store.Create(ctx, &database.Organization{
		Name:     "o2",
		Nickname: "o2_nickname",
	}, &database.Namespace{Path: "o2"})
	require.Nil(t, err)

	org2 := &database.Organization{}
	err = db.Core.NewSelect().Model(org2).Where("path=?", "o2").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "o2", org2.Name)

	member2 := &database.Member{
		OrganizationID: org2.ID,
		UserID:         321,
		Role:           string(types.UserRead),
		DeletedAt:      time.Now(),
	}

	err = db.Core.NewInsert().Model(member).Scan(ctx, member)
	require.Nil(t, err)
	org.UserID = owner.ID
	err = store.Update(ctx, org)
	require.Nil(t, err)

	err = db.Core.NewInsert().Model(member2).Scan(ctx, member2)
	require.Nil(t, err)

	orgs, total, err = store.GetUserOwnOrgs(ctx, "u1")
	require.Nil(t, err)
	require.Equal(t, 1, len(orgs))
	require.Equal(t, 1, total)
	isLastAdmin, err := store.IsLastOrganizationAdmin(ctx, "u1")
	require.NoError(t, err)
	require.False(t, isLastAdmin)
	adminMember := &database.Member{
		OrganizationID: org.ID,
		UserID:         owner.ID,
		Role:           string(types.UserAdmin),
	}
	require.NoError(t, db.Core.NewInsert().Model(adminMember).Scan(ctx, adminMember))
	isLastAdmin, err = store.IsLastOrganizationAdmin(ctx, "u1")
	require.NoError(t, err)
	require.True(t, isLastAdmin)
	require.NoError(t, database.NewMemberStoreWithDB(db).Delete(ctx, org.ID, owner.ID))
	isLastAdmin, err = store.IsLastOrganizationAdmin(ctx, "u1")
	require.NoError(t, err)
	require.False(t, isLastAdmin)

	orgs, err = store.GetUserBelongOrgs(ctx, 321)
	require.Nil(t, err)
	require.Equal(t, 1, len(orgs))

	err = store.Delete(ctx, "o1")
	require.Nil(t, err)
	membershipCount, err := db.Core.NewSelect().Model((*database.Member)(nil)).Where("member.organization_id = ?", org.ID).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, membershipCount)
	exist, err = store.Exists(ctx, "foo")
	require.Nil(t, err)
	require.False(t, exist)

}

func TestOrganizationStore_ModeFilters(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	legacyStore := database.NewOrgStoreWithDB(db)
	hierarchyStore := database.NewOrgStoreWithMode(db, true)
	legacyOrganization := &database.Organization{
		Name: "mode-legacy", Nickname: "Legacy", UUID: uuid.New(),
	}
	hierarchyOrganization := &database.Organization{
		Name: "mode-hierarchy", Nickname: "Hierarchy", UUID: uuid.New(),
	}
	require.NoError(t, legacyStore.Create(ctx, legacyOrganization, &database.Namespace{Path: legacyOrganization.Name}))
	require.NoError(t, hierarchyStore.Create(ctx, hierarchyOrganization, &database.Namespace{Path: hierarchyOrganization.Name}))

	var storedLegacy, storedHierarchy database.Organization
	require.NoError(t, db.Core.NewSelect().Model(&storedLegacy).Where("id = ?", legacyOrganization.ID).Scan(ctx))
	require.NoError(t, db.Core.NewSelect().Model(&storedHierarchy).Where("id = ?", hierarchyOrganization.ID).Scan(ctx))
	require.False(t, storedLegacy.IsHierarchical)
	require.True(t, storedHierarchy.IsHierarchical)

	_, err := legacyStore.FindByPath(ctx, hierarchyOrganization.Name)
	require.ErrorIs(t, err, sql.ErrNoRows)
	_, err = hierarchyStore.FindByPath(ctx, legacyOrganization.Name)
	require.ErrorIs(t, err, sql.ErrNoRows)

	legacyOrganizations, total, err := legacyStore.Search(ctx, "mode-", 20, 1, "", "", "")
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, legacyOrganization.Name, legacyOrganizations[0].Name)

	hierarchyOrganizations, total, err := hierarchyStore.Search(ctx, "mode-", 20, 1, "", "", "")
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, hierarchyOrganization.Name, hierarchyOrganizations[0].Name)
}

// TestOrganizationStore_GetUserRootOrganizationsReturnsHierarchyRoots verifies descendant memberships resolve to the same top-level organization.
func TestOrganizationStore_GetUserRootOrganizationsReturnsHierarchyRoots(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	root := createRootOrganization(t, ctx, db, "user-root-orgs")
	unitStore := database.NewOrganizationUnitStoreWithDB(db)
	research := createChild(t, ctx, unitStore, root, "user-root-orgs-research", nil, 1)
	it := createChild(t, ctx, unitStore, root, "user-root-orgs-it", &research.UUID, 1)

	users := []*database.User{
		createOrganizationUnitMemberTestUser(t, ctx, db, "user-root-orgs-a"),
		createOrganizationUnitMemberTestUser(t, ctx, db, "user-root-orgs-b"),
		createOrganizationUnitMemberTestUser(t, ctx, db, "user-root-orgs-c"),
		createOrganizationUnitMemberTestUser(t, ctx, db, "user-root-orgs-d"),
	}
	memberStore := database.NewMemberStoreWithDB(db)
	researchOrganizationID := organizationIDForUnit(t, ctx, unitStore, research.UUID)
	itOrganizationID := organizationIDForUnit(t, ctx, unitStore, it.UUID)
	for _, membership := range []struct {
		userID         int64
		organizationID int64
	}{
		{users[0].ID, researchOrganizationID},
		{users[1].ID, itOrganizationID},
		{users[2].ID, root.ID},
		{users[3].ID, researchOrganizationID},
		{users[3].ID, itOrganizationID},
	} {
		require.NoError(t, memberStore.Add(ctx, membership.organizationID, membership.userID, string(types.UserRead)))
	}

	store := database.NewOrgStoreWithMode(db, true)
	for _, user := range users {
		organizations, err := store.GetUserRootOrganizations(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, organizations, 1)
		require.Equal(t, root.ID, organizations[0].ID)
		require.Equal(t, root.UUID, organizations[0].UUID)
	}
}

// TestOrganizationStore_IsLastOrganizationAdmin verifies every organization is evaluated independently.
func TestOrganizationStore_IsLastOrganizationAdmin(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	legacyStore := database.NewOrgStoreWithDB(db)
	hierarchyStore := database.NewOrgStoreWithMode(db, true)
	organizations := map[string]*database.Organization{}
	createOrganization := func(store database.OrgStore, name string) {
		organization := &database.Organization{Name: name, Nickname: name, UUID: uuid.New()}
		require.NoError(t, store.Create(ctx, organization, &database.Namespace{Path: name}))
		organizations[name] = organization
	}
	createOrganization(legacyStore, "last-admin-kng")
	createOrganization(hierarchyStore, "last-admin-dep")
	createOrganization(hierarchyStore, "last-admin-qa")
	createOrganization(hierarchyStore, "last-admin-ci")

	users := map[string]*database.User{}
	for _, username := range []string{"last-admin-test", "last-admin-backup-one", "last-admin-backup-two"} {
		user := &database.User{Username: username, UUID: uuid.NewString(), Password: "test-password"}
		require.NoError(t, db.Core.NewInsert().Model(user).Scan(ctx, user))
		users[username] = user
	}
	addMember := func(organizationName, username string, role types.UserRole) {
		member := &database.Member{
			OrganizationID: organizations[organizationName].ID,
			UserID:         users[username].ID,
			Role:           string(role),
		}
		require.NoError(t, db.Core.NewInsert().Model(member).Scan(ctx, member))
	}

	addMember("last-admin-kng", "last-admin-test", types.UserAdmin)
	addMember("last-admin-kng", "last-admin-backup-one", types.UserAdmin)
	addMember("last-admin-kng", "last-admin-backup-two", types.UserAdmin)
	addMember("last-admin-dep", "last-admin-test", types.UserAdmin)
	addMember("last-admin-dep", "last-admin-backup-one", types.UserAdmin)
	addMember("last-admin-qa", "last-admin-test", types.UserWrite)
	addMember("last-admin-ci", "last-admin-test", types.UserAdmin)

	isLastAdmin, err := legacyStore.IsLastOrganizationAdmin(ctx, "last-admin-test")
	require.NoError(t, err)
	require.False(t, isLastAdmin)

	isLastAdmin, err = hierarchyStore.IsLastOrganizationAdmin(ctx, "last-admin-test")
	require.NoError(t, err)
	require.True(t, isLastAdmin)

	addMember("last-admin-ci", "last-admin-backup-two", types.UserAdmin)
	isLastAdmin, err = hierarchyStore.IsLastOrganizationAdmin(ctx, "last-admin-test")
	require.NoError(t, err)
	require.False(t, isLastAdmin)
}

func TestOrganizationStore_CreateWithRelations(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	userStore := database.NewUserStoreWithDB(db)
	require.NoError(t, userStore.Create(ctx, &database.User{
		GitID:    90001,
		UUID:     uuid.NewString(),
		Username: "atomic-owner",
		Password: "test-password",
	}, &database.Namespace{Path: "atomic-owner"}))
	owner, err := userStore.FindByUsername(ctx, "atomic-owner")
	require.NoError(t, err)

	store := database.NewOrgStoreWithDB(db)

	org := &database.Organization{
		Name:     "atomic-org",
		Nickname: "Atomic Org",
		UUID:     uuid.New(),
		UserID:   owner.ID,
	}
	require.NoError(t, store.CreateWithRelations(ctx, org, &database.Namespace{Path: org.Name}, []int64{201, 202}))

	storedOrg, err := store.FindByPath(ctx, org.Name)
	require.NoError(t, err)
	require.Equal(t, org.ID, storedOrg.ID)

	member, err := database.NewMemberStoreWithDB(db).Find(ctx, org.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, string(types.UserAdmin), member.Role)

	var tagCount int
	tagCount, err = db.Core.NewSelect().
		Model((*database.OrganizationTag)(nil)).
		Where("organization_tag.organization_id = ?", org.ID).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, tagCount)
}

func TestOrganizationStore_CreateWithRelationsRollback(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	userStore := database.NewUserStoreWithDB(db)
	require.NoError(t, userStore.Create(ctx, &database.User{
		GitID:    90002,
		UUID:     uuid.NewString(),
		Username: "rollback-owner",
		Password: "test-password",
	}, &database.Namespace{Path: "rollback-owner"}))
	owner, err := userStore.FindByUsername(ctx, "rollback-owner")
	require.NoError(t, err)

	store := database.NewOrgStoreWithDB(db)

	org := &database.Organization{
		Name:     "rollback-org",
		Nickname: "Rollback Org",
		UUID:     uuid.New(),
		UserID:   owner.ID,
	}
	err = store.CreateWithRelations(ctx, org, &database.Namespace{Path: org.Name}, []int64{301, 301})
	require.Error(t, err)

	_, err = store.FindByPath(ctx, org.Name)
	require.ErrorIs(t, err, sql.ErrNoRows)

	namespaceCount, err := db.Core.NewSelect().
		Model((*database.Namespace)(nil)).
		Where("namespace.path = ?", org.Name).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, namespaceCount)
}

func TestOrganization_CreateWithForceDelete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	nsStore := database.NewNamespaceStoreWithDB(db)
	orgStore := database.NewOrgStoreWithDB(db)
	err := orgStore.Create(ctx, &database.Organization{
		Name:     "o1",
		Nickname: "o1_nickname",
	}, &database.Namespace{Path: "o1", DeletedAt: time.Now()})
	require.Nil(t, err)

	err = orgStore.Delete(ctx, "o1")
	require.Nil(t, err)

	orgs, total, err := orgStore.Search(ctx, "o1", 10, 1, "", "", "")
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Empty(t, orgs)

	_, err = nsStore.FindByPath(ctx, "o1")
	require.Equal(t, true, errors.Is(err, sql.ErrNoRows))
}

func TestOrganizationStore_GetOrgByUserIDs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewOrgStoreWithDB(db)

	// Create organizations with explicit UUID values
	err := store.Create(ctx, &database.Organization{
		Name:     "org1",
		Nickname: "org1_nickname",
		UUID:     uuid.New(),
	}, &database.Namespace{Path: "org1"})
	require.Nil(t, err)
	err = store.Create(ctx, &database.Organization{
		Name:     "org2",
		Nickname: "org2_nickname",
		UUID:     uuid.New(),
	}, &database.Namespace{Path: "org2"})
	require.Nil(t, err)

	// Get org IDs
	org1 := &database.Organization{}
	err = db.Core.NewSelect().Model(org1).Where("path = ?", "org1").Scan(ctx)
	require.Nil(t, err)
	org2 := &database.Organization{}
	err = db.Core.NewSelect().Model(org2).Where("path = ?", "org2").Scan(ctx)
	require.Nil(t, err)

	// Add members
	userIDs := []int64{101, 102}
	for _, uid := range userIDs {
		member := &database.Member{
			OrganizationID: org1.ID,
			UserID:         uid,
			Role:           string(types.UserRead),
		}
		err = db.Core.NewInsert().Model(member).Scan(ctx, member)
		require.Nil(t, err)
	}
	// org2 only has one member
	member := &database.Member{
		OrganizationID: org2.ID,
		UserID:         101,
		Role:           string(types.UserRead),
	}
	err = db.Core.NewInsert().Model(member).Scan(ctx, member)
	require.Nil(t, err)

	// Should return org1 for both userIDs
	orgs, err := store.GetSharedOrgIDs(ctx, userIDs)
	require.Nil(t, err)
	require.Equal(t, 1, len(orgs))

	// Should return org2 for userID 101 only
	orgs, err = store.GetSharedOrgIDs(ctx, []int64{101})
	require.Nil(t, err)
	require.Len(t, orgs, 2)
	var foundOrg2 bool
	for _, o := range orgs {
		if o == org2.ID {
			foundOrg2 = true
		}
	}
	require.True(t, foundOrg2)

	// Should return empty for userID not in any org
	orgs, err = store.GetSharedOrgIDs(ctx, []int64{999})
	require.Nil(t, err)
	require.Empty(t, orgs)

	// Should return empty for empty input
	orgs, err = store.GetSharedOrgIDs(ctx, []int64{})
	require.Nil(t, err)
	require.Empty(t, orgs)
}

func TestOrganizationStore_FindByUUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewOrgStoreWithDB(db)

	// Test case 1: Find an existing organization by UUID
	testUUID := uuid.New()
	err := store.Create(ctx, &database.Organization{
		Name:     "test_org",
		Nickname: "test_org_nickname",
		UUID:     testUUID,
	}, &database.Namespace{Path: "test_org"})
	require.Nil(t, err)

	// Find the organization by UUID
	org, err := store.FindByUUID(ctx, testUUID.String())
	require.Nil(t, err)
	require.NotNil(t, org)
	require.Equal(t, "test_org", org.Name)
	require.Equal(t, testUUID, org.UUID)

	// Test case 2: Find non-existent organization by UUID
	nonExistentUUID := uuid.New()
	org, err = store.FindByUUID(ctx, nonExistentUUID.String())
	require.Nil(t, err)
	require.Nil(t, org)

	// Test case 3: Find organization with invalid UUID format
	// Database returns error for invalid UUID format
	org, err = store.FindByUUID(ctx, "invalid-uuid-format")
	require.NotNil(t, err)
	require.Nil(t, org)
}

func TestOrganizationStore_SearchOrder(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewOrgStoreWithDB(db)
	orgsToCreate := []database.Organization{
		{
			Name:     "sss",
			Nickname: "zzz org",
			UUID:     uuid.New(),
		},
		{
			Name:     "sss-team",
			Nickname: "alpha org",
			UUID:     uuid.New(),
		},
		{
			Name:     "team-01",
			Nickname: "sss",
			UUID:     uuid.New(),
		},
		{
			Name:     "team-02",
			Nickname: "sss group",
			UUID:     uuid.New(),
		},
		{
			Name:     "team-03",
			Nickname: "group sss",
			UUID:     uuid.New(),
		},
	}

	for _, org := range orgsToCreate {
		err := store.Create(ctx, &org, &database.Namespace{Path: org.Name})
		require.Nil(t, err)
	}

	orgs, total, err := store.Search(ctx, "sss", 10, 1, "", "", "")
	require.Nil(t, err)
	require.Equal(t, 5, total)

	gotNames := make([]string, 0, len(orgs))
	for _, org := range orgs {
		gotNames = append(gotNames, org.Name)
	}

	require.Equal(t, []string{"sss", "sss-team", "team-01", "team-02", "team-03"}, gotNames)
}

func TestOrganizationStore_Tags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewOrgStoreWithDB(db)

	// Create an organization
	err := store.Create(ctx, &database.Organization{
		Name:     "tag_org",
		Nickname: "tag_org_nickname",
		UUID:     uuid.New(),
	}, &database.Namespace{Path: "tag_org"})
	require.Nil(t, err)

	org := &database.Organization{}
	err = db.Core.NewSelect().Model(org).Where("path = ?", "tag_org").Scan(ctx)
	require.Nil(t, err)

	// Use existing seeded industry tags
	var tagInternet database.Tag
	err = db.Core.NewSelect().Model(&tagInternet).
		Where("name = ? AND category = ? AND scope = ?", "internet", "industry", "organization").
		Scan(ctx)
	require.Nil(t, err)

	var tagFinance database.Tag
	err = db.Core.NewSelect().Model(&tagFinance).
		Where("name = ? AND category = ? AND scope = ?", "finance", "industry", "organization").
		Scan(ctx)
	require.Nil(t, err)

	// Set organization tags
	err = store.SetOrganizationTags(ctx, org.ID, []int64{tagInternet.ID, tagFinance.ID})
	require.Nil(t, err)

	// Get organization tags
	tags, err := store.GetOrganizationTags(ctx, org.ID)
	require.Nil(t, err)
	require.Len(t, tags, 2)
	require.Equal(t, "internet", tags[0].Name)
	require.Equal(t, "finance", tags[1].Name)

	// Replace tags (set new tags, old ones removed)
	var tagHealthcare database.Tag
	err = db.Core.NewSelect().Model(&tagHealthcare).
		Where("name = ? AND category = ? AND scope = ?", "healthcare", "industry", "organization").
		Scan(ctx)
	require.Nil(t, err)

	err = store.SetOrganizationTags(ctx, org.ID, []int64{tagHealthcare.ID})
	require.Nil(t, err)

	tags, err = store.GetOrganizationTags(ctx, org.ID)
	require.Nil(t, err)
	require.Len(t, tags, 1)
	require.Equal(t, "healthcare", tags[0].Name)

	// Remove all tags
	err = store.SetOrganizationTags(ctx, org.ID, []int64{})
	require.Nil(t, err)

	tags, err = store.GetOrganizationTags(ctx, org.ID)
	require.Nil(t, err)
	require.Len(t, tags, 0)
}

func TestOrganizationStore_SearchOrderCaseInsensitive(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewOrgStoreWithDB(db)
	err := store.Create(ctx, &database.Organization{
		Name:     "SSS-Exact",
		Nickname: "display",
		UUID:     uuid.New(),
	}, &database.Namespace{Path: "SSS-Exact"})
	require.Nil(t, err)
	err = store.Create(ctx, &database.Organization{
		Name:     "other",
		Nickname: "sss",
		UUID:     uuid.New(),
	}, &database.Namespace{Path: "other"})
	require.Nil(t, err)

	orgs, total, err := store.Search(ctx, "sss-exact", 10, 1, "", "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, orgs, 1)
	require.Equal(t, "SSS-Exact", orgs[0].Name)

	orgs, total, err = store.Search(ctx, "SSS", 10, 1, "", "", "")
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.True(t, slices.Equal([]string{"SSS-Exact", "other"}, []string{orgs[0].Name, orgs[1].Name}))
}

func TestOrganizationStore_SearchUserBelongOrgs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewOrgStoreWithDB(db)

	// Create users
	user1 := &database.User{Username: "belong_user1", UUID: uuid.New().String()}
	err := db.Core.NewInsert().Model(user1).Scan(ctx, user1)
	require.Nil(t, err)
	user2 := &database.User{Username: "belong_user2", UUID: uuid.New().String()}
	err = db.Core.NewInsert().Model(user2).Scan(ctx, user2)
	require.Nil(t, err)

	// Create orgs
	err = store.Create(ctx, &database.Organization{Name: "belong_org1", Nickname: "Belong Org 1", UUID: uuid.New()}, &database.Namespace{Path: "belong_org1"})
	require.Nil(t, err)
	err = store.Create(ctx, &database.Organization{Name: "belong_org2", Nickname: "Belong Org 2", UUID: uuid.New(), OrgType: "school"}, &database.Namespace{Path: "belong_org2"})
	require.Nil(t, err)
	err = store.Create(ctx, &database.Organization{Name: "belong_org3", Nickname: "Belong Org 3", UUID: uuid.New(), UserID: user1.ID}, &database.Namespace{Path: "belong_org3"})
	require.Nil(t, err)

	org1 := &database.Organization{}
	err = db.Core.NewSelect().Model(org1).Where("path = ?", "belong_org1").Scan(ctx)
	require.Nil(t, err)
	org2 := &database.Organization{}
	err = db.Core.NewSelect().Model(org2).Where("path = ?", "belong_org2").Scan(ctx)
	require.Nil(t, err)
	org3 := &database.Organization{}
	err = db.Core.NewSelect().Model(org3).Where("path = ?", "belong_org3").Scan(ctx)
	require.Nil(t, err)

	// user1 is admin of org1, write of org2, not member of org3
	members := []database.Member{
		{OrganizationID: org1.ID, UserID: user1.ID, Role: string(types.UserAdmin)},
		{OrganizationID: org2.ID, UserID: user1.ID, Role: string(types.UserWrite)},
		{OrganizationID: org1.ID, UserID: user2.ID, Role: string(types.UserRead)},
	}
	for i := range members {
		err = db.Core.NewInsert().Model(&members[i]).Scan(ctx, &members[i])
		require.Nil(t, err)
	}

	// user1: all member orgs
	orgs, total, err := store.SearchUserBelongOrgs(ctx, user1.ID, "", 10, 1, "", "", "", "")
	require.Nil(t, err)
	require.Equal(t, 2, total)
	names := make(map[string]bool, len(orgs))
	for _, o := range orgs {
		names[o.Name] = true
	}
	require.True(t, names["belong_org1"])
	require.True(t, names["belong_org2"])

	// user1: admin role only
	orgs, total, err = store.SearchUserBelongOrgs(ctx, user1.ID, "", 10, 1, "", "", string(types.UserAdmin), "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "belong_org1", orgs[0].Name)

	// user1: write role only
	orgs, total, err = store.SearchUserBelongOrgs(ctx, user1.ID, "", 10, 1, "", "", string(types.UserWrite), "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "belong_org2", orgs[0].Name)

	// user2: all member orgs (only read on org1)
	orgs, total, err = store.SearchUserBelongOrgs(ctx, user2.ID, "", 10, 1, "", "", "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "belong_org1", orgs[0].Name)

	// user2: admin role — none
	orgs, total, err = store.SearchUserBelongOrgs(ctx, user2.ID, "", 10, 1, "", "", string(types.UserAdmin), "")
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Empty(t, orgs)

	// Filter by org type
	orgs, total, err = store.SearchUserBelongOrgs(ctx, user1.ID, "", 10, 1, "school", "", "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "belong_org2", orgs[0].Name)

	// Filter by search
	orgs, total, err = store.SearchUserBelongOrgs(ctx, user1.ID, "org1", 10, 1, "", "", "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "belong_org1", orgs[0].Name)
}

func TestOrganizationStore_GetOrganizationTagsByOrgIDs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewOrgStoreWithDB(db)

	// Create two orgs
	err := store.Create(ctx, &database.Organization{Name: "tag_batch_org1", Nickname: "Batch 1", UUID: uuid.New()}, &database.Namespace{Path: "tag_batch_org1"})
	require.Nil(t, err)
	err = store.Create(ctx, &database.Organization{Name: "tag_batch_org2", Nickname: "Batch 2", UUID: uuid.New()}, &database.Namespace{Path: "tag_batch_org2"})
	require.Nil(t, err)
	err = store.Create(ctx, &database.Organization{Name: "tag_batch_org3", Nickname: "Batch 3", UUID: uuid.New()}, &database.Namespace{Path: "tag_batch_org3"})
	require.Nil(t, err)

	org1 := &database.Organization{}
	err = db.Core.NewSelect().Model(org1).Where("path = ?", "tag_batch_org1").Scan(ctx)
	require.Nil(t, err)
	org2 := &database.Organization{}
	err = db.Core.NewSelect().Model(org2).Where("path = ?", "tag_batch_org2").Scan(ctx)
	require.Nil(t, err)

	// Look up seeded industry tags
	var tagInternet, tagFinance database.Tag
	err = db.Core.NewSelect().Model(&tagInternet).Where("name = ? AND category = ? AND scope = ?", "internet", "industry", "organization").Scan(ctx)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(&tagFinance).Where("name = ? AND category = ? AND scope = ?", "finance", "industry", "organization").Scan(ctx)
	require.Nil(t, err)

	// Assign tags to orgs (org1 gets both, org2 gets only finance)
	err = store.SetOrganizationTags(ctx, org1.ID, []int64{tagInternet.ID, tagFinance.ID})
	require.Nil(t, err)
	err = store.SetOrganizationTags(ctx, org2.ID, []int64{tagFinance.ID})
	require.Nil(t, err)

	// Batch load tags for all three org IDs
	tagMap, err := store.GetOrganizationTagsByOrgIDs(ctx, []int64{org1.ID, org2.ID, org1.ID + 999})
	require.Nil(t, err)
	require.Len(t, tagMap, 2) // only org1 and org2 have tags
	require.Len(t, tagMap[org1.ID], 2)
	require.Len(t, tagMap[org2.ID], 1)
	require.Equal(t, "finance", tagMap[org2.ID][0].Name)

	// Empty input
	tagMap, err = store.GetOrganizationTagsByOrgIDs(ctx, []int64{})
	require.Nil(t, err)
	require.Empty(t, tagMap)
}

func TestOrganizationStore_Delete_CleansUpTags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewOrgStoreWithDB(db)

	err := store.Create(ctx, &database.Organization{Name: "del_org", Nickname: "Del Org", UUID: uuid.New()}, &database.Namespace{Path: "del_org"})
	require.Nil(t, err)

	org := &database.Organization{}
	err = db.Core.NewSelect().Model(org).Where("path = ?", "del_org").Scan(ctx)
	require.Nil(t, err)

	// Assign tags
	var tagInternet database.Tag
	err = db.Core.NewSelect().Model(&tagInternet).Where("name = ? AND category = ? AND scope = ?", "internet", "industry", "organization").Scan(ctx)
	require.Nil(t, err)
	err = store.SetOrganizationTags(ctx, org.ID, []int64{tagInternet.ID})
	require.Nil(t, err)

	// Verify tags exist
	tags, err := store.GetOrganizationTags(ctx, org.ID)
	require.Nil(t, err)
	require.Len(t, tags, 1)

	// Delete the organization
	err = store.Delete(ctx, "del_org")
	require.Nil(t, err)

	// Verify org is gone
	exist, err := store.Exists(ctx, "del_org")
	require.Nil(t, err)
	require.False(t, exist)

	// Verify organization_tags rows are cleaned up (re-create org with same path, check no stale tags)
	err = store.Create(ctx, &database.Organization{Name: "del_org", Nickname: "Del Org 2", UUID: uuid.New()}, &database.Namespace{Path: "del_org"})
	require.Nil(t, err)
	org2 := &database.Organization{}
	err = db.Core.NewSelect().Model(org2).Where("path = ?", "del_org").Scan(ctx)
	require.Nil(t, err)

	tags, err = store.GetOrganizationTags(ctx, org2.ID)
	require.Nil(t, err)
	require.Len(t, tags, 0)
}
