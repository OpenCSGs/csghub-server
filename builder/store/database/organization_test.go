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
	orgs, total, err := store.Search(ctx, "o1", 10, 1, "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "o1", orgs[0].Name)
	require.Equal(t, uuid, orgs[0].UUID)
	//search with nickname
	orgs, total, err = store.Search(ctx, "nickname", 10, 1, "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "o1_nickname", orgs[0].Nickname)
	//empty search second page
	orgs, total, err = store.Search(ctx, "nickname", 10, 2, "", "")
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

	orgs, err = store.GetUserBelongOrgs(ctx, 321)
	require.Nil(t, err)
	require.Equal(t, 1, len(orgs))

	err = store.Delete(ctx, "o1")
	require.Nil(t, err)
	exist, err = store.Exists(ctx, "foo")
	require.Nil(t, err)
	require.False(t, exist)

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

	orgs, total, err := orgStore.Search(ctx, "o1", 10, 1, "", "")
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
		}
		err = db.Core.NewInsert().Model(member).Scan(ctx, member)
		require.Nil(t, err)
	}
	// org2 only has one member
	member := &database.Member{
		OrganizationID: org2.ID,
		UserID:         101,
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

	orgs, total, err := store.Search(ctx, "sss", 10, 1, "", "")
	require.Nil(t, err)
	require.Equal(t, 5, total)

	gotNames := make([]string, 0, len(orgs))
	for _, org := range orgs {
		gotNames = append(gotNames, org.Name)
	}

	require.Equal(t, []string{"sss", "sss-team", "team-01", "team-02", "team-03"}, gotNames)
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

	orgs, total, err := store.Search(ctx, "sss-exact", 10, 1, "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, orgs, 1)
	require.Equal(t, "SSS-Exact", orgs[0].Name)

	orgs, total, err = store.Search(ctx, "SSS", 10, 1, "", "")
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.True(t, slices.Equal([]string{"SSS-Exact", "other"}, []string{orgs[0].Name, orgs[1].Name}))
}
