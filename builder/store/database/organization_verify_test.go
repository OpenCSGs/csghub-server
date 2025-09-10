package database_test

import (
	"context"
	"opencsg.com/csghub-server/common/types"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestOrganizationVerifyStore(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewOrganizationVerifyStoreWithDB(db)

	orgStore := database.NewOrgStoreWithDB(db)
	err := orgStore.Create(ctx, &database.Organization{
		Name:     "verify_test_org",
		Nickname: "verify_test_nick",
	}, &database.Namespace{Path: "verify_test_org"})
	require.Nil(t, err)

	org := &database.Organization{}
	err = db.Core.NewSelect().Model(org).Where("name = ?", "verify_test_nick").Scan(ctx)
	require.Nil(t, err)

	orgVerify := &database.OrganizationVerify{
		Name:               org.Name,
		CompanyName:        "Test Co., Ltd",
		UnifiedCreditCode:  "91350211MA2Y7XYZW1",
		Username:           "verifier",
		ContactName:        "Test Person",
		ContactEmail:       "test@example.com",
		BusinessLicenseImg: "http://example.com/license.jpg",
		Status:             "pending",
	}

	created, err := store.CreateOrganizationVerify(ctx, orgVerify)
	require.Nil(t, err)
	require.Equal(t, "Test Co., Ltd", created.CompanyName)
	require.Equal(t, types.VerifyStatusPending, created.Status)

	updated, err := store.UpdateOrganizationVerify(ctx, created.ID, types.VerifyStatusApproved, "All good")
	require.Nil(t, err)
	require.Equal(t, types.VerifyStatusApproved, updated.Status)
	require.Equal(t, "All good", updated.Reason)

	err = orgStore.UpdateVerifyStatus(ctx, org.Name, types.VerifyStatusApproved)
	require.Nil(t, err)

	fetched, err := store.GetOrganizationVerify(ctx, org.Name)
	require.Nil(t, err)
	require.Equal(t, types.VerifyStatusApproved, fetched.Status)
	require.Equal(t, "All good", fetched.Reason)
}
