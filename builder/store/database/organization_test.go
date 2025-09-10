package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestOrganizationStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewOrgStoreWithDB(db)
	err := store.Create(ctx, &database.Organization{
		Name:     "o1",
		Nickname: "o1_nickname",
	}, &database.Namespace{Path: "o1"})
	require.Nil(t, err)

	//search with name
	orgs, total, err := store.Search(ctx, "o1", 10, 1, "", "")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "o1", orgs[0].Name)
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

	// Create organizations
	err := store.Create(ctx, &database.Organization{
		Name:     "org1",
		Nickname: "org1_nickname",
	}, &database.Namespace{Path: "org1"})
	require.Nil(t, err)
	err = store.Create(ctx, &database.Organization{
		Name:     "org2",
		Nickname: "org2_nickname",
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
