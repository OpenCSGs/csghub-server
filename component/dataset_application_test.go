//go:build saas

package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func setupSaaSComponent(t *testing.T) (*datasetComponentImpl, *mockdb.MockDatasetStore, *mockdb.MockUserStore, *mockcomponent.MockRepoComponent, *mockdb.MockDatasetApplicationStore) {
	t.Helper()
	dsStore := mockdb.NewMockDatasetStore(t)
	userStore := mockdb.NewMockUserStore(t)
	repoComp := mockcomponent.NewMockRepoComponent(t)
	appStore := mockdb.NewMockDatasetApplicationStore(t)

	c := &datasetComponentImpl{
		datasetStore:  dsStore,
		userStore:     userStore,
		repoComponent: repoComp,
		extendDatasetImpl: extendDatasetImpl{
			datasetApplicationStore: appStore,
		},
	}
	return c, dsStore, userStore, repoComp, appStore
}

func TestCreateDatasetApplication_NoPermission(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, userStore, repoComp, _ := setupSaaSComponent(t)

	dataset := &database.Dataset{
		ID:         1,
		Status:     types.DatasetStatusNormal,
		Repository: &database.Repository{Path: "user/d1"},
	}
	dsStore.On("FindByPath", ctx, "user", "d1").Return(dataset, nil)
	userStore.On("FindByUsername", ctx, "u").Return(database.User{ID: 1, Username: "u"}, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", dataset.Repository).Return(&types.UserRepoPermission{CanWrite: false}, nil)

	req := &types.CreateDatasetApplicationReq{
		Namespace:        "user",
		Name:             "d1",
		Action:           "list",
		Price:            99,
		RelatedDatasetID: 2,
		CurrentUser:      "u",
	}
	_, err := c.CreateDatasetApplication(ctx, req)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrForbidden))
}

func TestCreateDatasetApplication_PendingExists(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, userStore, repoComp, appStore := setupSaaSComponent(t)

	dataset := &database.Dataset{
		ID:         1,
		Status:     types.DatasetStatusNormal,
		Repository: &database.Repository{Path: "user/d1"},
	}
	dsStore.On("FindByPath", ctx, "user", "d1").Return(dataset, nil)
	userStore.On("FindByUsername", ctx, "u").Return(database.User{ID: 1, Username: "u"}, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", dataset.Repository).Return(&types.UserRepoPermission{CanWrite: true}, nil)
	appStore.On("FindPendingByDatasetID", ctx, int64(1)).Return(&database.DatasetApplication{ID: 99}, nil)

	req := &types.CreateDatasetApplicationReq{
		Namespace:        "user",
		Name:             "d1",
		Action:           "list",
		Price:            99,
		RelatedDatasetID: 2,
		CurrentUser:      "u",
	}
	_, err := c.CreateDatasetApplication(ctx, req)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrPendingApplicationExists))
}

func TestCreateDatasetApplication_RelatedDatasetNotFound(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, userStore, repoComp, appStore := setupSaaSComponent(t)

	dataset := &database.Dataset{
		ID:         1,
		Status:     types.DatasetStatusNormal,
		Repository: &database.Repository{Path: "user/d1"},
	}
	dsStore.On("FindByPath", ctx, "user", "d1").Return(dataset, nil)
	userStore.On("FindByUsername", ctx, "u").Return(database.User{ID: 1, Username: "u"}, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", dataset.Repository).Return(&types.UserRepoPermission{CanWrite: true}, nil)
	appStore.On("FindPendingByDatasetID", ctx, int64(1)).Return(nil, errorx.HandleDBError(sql.ErrNoRows, nil))
	dsStore.On("ByID", ctx, int64(999)).Return(nil, sql.ErrNoRows)

	req := &types.CreateDatasetApplicationReq{
		Namespace:        "user",
		Name:             "d1",
		Action:           "list",
		Price:            99,
		RelatedDatasetID: 999,
		CurrentUser:      "u",
	}
	_, err := c.CreateDatasetApplication(ctx, req)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "related dataset does not exist")
}

func TestCreateDatasetApplication_Success(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, userStore, repoComp, appStore := setupSaaSComponent(t)

	dataset := &database.Dataset{
		ID:         1,
		Status:     types.DatasetStatusNormal,
		Repository: &database.Repository{Path: "user/d1"},
	}
	dsStore.On("FindByPath", ctx, "user", "d1").Return(dataset, nil)
	userStore.On("FindByUsername", ctx, "u").Return(database.User{ID: 1, Username: "u"}, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", dataset.Repository).Return(&types.UserRepoPermission{CanWrite: true}, nil)
	appStore.On("FindPendingByDatasetID", ctx, int64(1)).Return(nil, errorx.HandleDBError(sql.ErrNoRows, nil))

	relatedDs := &database.Dataset{
		ID:         2,
		Status:     types.DatasetStatusNormal,
		Repository: &database.Repository{Path: "user/d2"},
	}
	dsStore.On("ByID", ctx, int64(2)).Return(relatedDs, nil)
	dsStore.On("FindByRelatedDatasetIDs", ctx, []int64{1, 2}).Return([]database.Dataset{}, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", relatedDs.Repository).Return(&types.UserRepoPermission{CanWrite: true}, nil)

	created := &database.DatasetApplication{
		ID:               10,
		DatasetID:        1,
		ApplicantID:      1,
		Action:           types.DatasetApplicationActionInitial,
		Price:            99,
		RelatedDatasetID: 2,
		Status:           types.DatasetApplicationStatusPending,
	}
	appStore.On("CreateApplicationAndLinkDataset", ctx, mock.MatchedBy(func(a database.DatasetApplication) bool {
		return a.DatasetID == 1 && a.Action == types.DatasetApplicationActionInitial
	})).Return(created, nil)

	req := &types.CreateDatasetApplicationReq{
		Namespace:        "user",
		Name:             "d1",
		Action:           "list",
		Price:            99,
		RelatedDatasetID: 2,
		CurrentUser:      "u",
	}
	app, err := c.CreateDatasetApplication(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, app)
	require.Equal(t, types.DatasetApplicationActionInitial, app.Action)
}

func TestCreateDatasetApplication_RelatedDatasetAlreadyReferenced(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, userStore, repoComp, appStore := setupSaaSComponent(t)

	dataset := &database.Dataset{
		ID:         1,
		Status:     types.DatasetStatusNormal,
		Repository: &database.Repository{Path: "user/d1"},
	}
	dsStore.On("FindByPath", ctx, "user", "d1").Return(dataset, nil)
	userStore.On("FindByUsername", ctx, "u").Return(database.User{ID: 1, Username: "u"}, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", dataset.Repository).Return(&types.UserRepoPermission{CanWrite: true}, nil)
	appStore.On("FindPendingByDatasetID", ctx, int64(1)).Return(nil, errorx.HandleDBError(sql.ErrNoRows, nil))

	relatedDs := &database.Dataset{
		ID:         2,
		Status:     types.DatasetStatusNormal,
		Repository: &database.Repository{Path: "user/d2"},
	}
	dsStore.On("ByID", ctx, int64(2)).Return(relatedDs, nil)
	// Related dataset (ID=2) is already referenced by another dataset (ID=3)
	dsStore.On("FindByRelatedDatasetIDs", ctx, []int64{1, 2}).Return([]database.Dataset{{ID: 3, RelatedDatasetID: 2}}, nil)

	req := &types.CreateDatasetApplicationReq{
		Namespace:        "user",
		Name:             "d1",
		Action:           "list",
		Price:            99,
		RelatedDatasetID: 2,
		CurrentUser:      "u",
	}
	_, err := c.CreateDatasetApplication(ctx, req)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "related dataset is already referenced by another dataset")
}

func TestCreateDatasetApplication_DatasetAlreadyReferencingAnother(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, userStore, repoComp, appStore := setupSaaSComponent(t)

	// Current dataset (ID=1) is already referenced by another dataset (ID=3) as RelatedDatasetID
	dataset := &database.Dataset{
		ID:         1,
		Status:     types.DatasetStatusNormal,
		Repository: &database.Repository{Path: "user/d1"},
	}
	dsStore.On("FindByPath", ctx, "user", "d1").Return(dataset, nil)
	userStore.On("FindByUsername", ctx, "u").Return(database.User{ID: 1, Username: "u"}, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", dataset.Repository).Return(&types.UserRepoPermission{CanWrite: true}, nil)
	appStore.On("FindPendingByDatasetID", ctx, int64(1)).Return(nil, errorx.HandleDBError(sql.ErrNoRows, nil))

	relatedDs := &database.Dataset{
		ID:         2,
		Status:     types.DatasetStatusNormal,
		Repository: &database.Repository{Path: "user/d2"},
	}
	dsStore.On("ByID", ctx, int64(2)).Return(relatedDs, nil)
	// Current dataset (ID=1) is already referenced by another dataset (ID=3) as RelatedDatasetID
	dsStore.On("FindByRelatedDatasetIDs", ctx, []int64{1, 2}).Return([]database.Dataset{{ID: 3, RelatedDatasetID: 1}}, nil)

	req := &types.CreateDatasetApplicationReq{
		Namespace:        "user",
		Name:             "d1",
		Action:           "list",
		Price:            99,
		RelatedDatasetID: 2,
		CurrentUser:      "u",
	}
	_, err := c.CreateDatasetApplication(ctx, req)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "dataset is already referenced by another dataset")
}

func TestCreateDatasetApplication_SameDatasetAndRelatedDatasetID(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, userStore, repoComp, appStore := setupSaaSComponent(t)

	dataset := &database.Dataset{
		ID:         1,
		Status:     types.DatasetStatusNormal,
		Repository: &database.Repository{Path: "user/d1"},
	}
	dsStore.On("FindByPath", ctx, "user", "d1").Return(dataset, nil)
	userStore.On("FindByUsername", ctx, "u").Return(database.User{ID: 1, Username: "u"}, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", dataset.Repository).Return(&types.UserRepoPermission{CanWrite: true}, nil)
	appStore.On("FindPendingByDatasetID", ctx, int64(1)).Return(nil, errorx.HandleDBError(sql.ErrNoRows, nil))

	req := &types.CreateDatasetApplicationReq{
		Namespace:        "user",
		Name:             "d1",
		Action:           "list",
		Price:            99,
		RelatedDatasetID: 1, // same as dataset ID
		CurrentUser:      "u",
	}
	_, err := c.CreateDatasetApplication(ctx, req)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "related dataset cannot be the same as current dataset")
}

func TestGetDatasetApplication_NotFound(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, _, repoComp, _ := setupSaaSComponent(t)

	dataset := &database.Dataset{
		ID:                   1,
		CurrentApplicationID: 0,
		Repository:           &database.Repository{Path: "user/d1"},
	}
	dsStore.On("FindByPath", ctx, "user", "d1").Return(dataset, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", dataset.Repository).Return(&types.UserRepoPermission{CanWrite: true}, nil)

	_, err := c.GetDatasetApplication(ctx, "user", "d1", "u")
	require.NotNil(t, err)
}

func TestGetDatasetApplication_Success(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, _, repoComp, appStore := setupSaaSComponent(t)

	dataset := &database.Dataset{
		ID:                   1,
		CurrentApplicationID: 10,
		Repository:           &database.Repository{Path: "user/d1"},
	}
	dsStore.On("FindByPath", ctx, "user", "d1").Return(dataset, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", dataset.Repository).Return(&types.UserRepoPermission{CanWrite: true}, nil)

	app := &database.DatasetApplication{
		ID:     10,
		Status: types.DatasetApplicationStatusPending,
		Action: types.DatasetApplicationActionInitial,
	}
	appStore.On("FindByID", ctx, int64(10)).Return(app, nil)

	result, err := c.GetDatasetApplication(ctx, "user", "d1", "u")
	require.Nil(t, err)
	require.NotNil(t, result)
	require.Equal(t, types.DatasetApplicationStatusPending, result.Status)
}

func TestGetDatasetApplication_NoPermission(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, _, repoComp, _ := setupSaaSComponent(t)

	dataset := &database.Dataset{
		ID:         1,
		Repository: &database.Repository{Path: "user/d1"},
	}
	dsStore.On("FindByPath", ctx, "user", "d1").Return(dataset, nil)
	repoComp.On("GetUserRepoPermission", ctx, "u", dataset.Repository).Return(&types.UserRepoPermission{CanWrite: false}, nil)

	_, err := c.GetDatasetApplication(ctx, "user", "d1", "u")
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrForbidden))
}

func TestReviewDatasetApplication_Reject(t *testing.T) {
	ctx := context.TODO()
	c, _, userStore, _, appStore := setupSaaSComponent(t)

	pendingApp := &database.DatasetApplication{
		ID:     1,
		Status: types.DatasetApplicationStatusPending,
		Action: types.DatasetApplicationActionInitial,
	}
	userStore.On("FindByUsername", ctx, "admin").Return(database.User{ID: 999, Username: "admin"}, nil)
	appStore.On("FindByID", ctx, int64(1)).Return(pendingApp, nil)
	appStore.On("ReviewApplication", ctx, int64(1), int64(999), "not good", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusRejected, (*database.ReviewDatasetUpdate)(nil)).
		Return(&database.DatasetApplication{ID: 1, Status: types.DatasetApplicationStatusRejected, Action: types.DatasetApplicationActionInitial}, nil)

	req := &types.ReviewDatasetApplicationReq{
		ID:          1,
		Action:      "reject",
		ReviewMsg:   "not good",
		CurrentUser: "admin",
	}
	_, err := c.ReviewDatasetApplication(ctx, req)
	require.Nil(t, err)
}

func TestReviewDatasetApplication_Approve(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, userStore, _, appStore := setupSaaSComponent(t)

	pendingApp := &database.DatasetApplication{
		ID:               1,
		DatasetID:        10,
		Action:           types.DatasetApplicationActionInitial,
		Price:            9.99,
		RelatedDatasetID: 0,
		Status:           types.DatasetApplicationStatusPending,
	}
	userStore.On("FindByUsername", ctx, "admin").Return(database.User{ID: 999, Username: "admin"}, nil)
	appStore.On("FindByID", ctx, int64(1)).Return(pendingApp, nil)
	dsStore.On("ByID", ctx, int64(10)).Return(&database.Dataset{
		ID:     10,
		Status: types.DatasetStatusNormal,
	}, nil)
	appStore.On("ReviewApplication", ctx, int64(1), int64(999), "", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, &database.ReviewDatasetUpdate{
		ExpectedDatasetStatus: types.DatasetStatusNormal,
		NewStatus:             types.DatasetStatusListed,
		Price:                 9.99,
		DatasetType:           types.DatasetTypeCommercial,
		RepositoryPrivate:     false,
	}).
		Return(&database.DatasetApplication{ID: 1, Status: types.DatasetApplicationStatusApproved, Action: types.DatasetApplicationActionInitial}, nil)

	req := &types.ReviewDatasetApplicationReq{
		ID:          1,
		Action:      "approve",
		CurrentUser: "admin",
	}
	_, err := c.ReviewDatasetApplication(ctx, req)
	require.Nil(t, err)
}

func TestReviewDatasetApplication_Approve_ConflictDatasetReferenced(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, userStore, _, appStore := setupSaaSComponent(t)

	pendingApp := &database.DatasetApplication{
		ID:               1,
		DatasetID:        10,
		Action:           types.DatasetApplicationActionInitial,
		Price:            9.99,
		RelatedDatasetID: 20,
		Status:           types.DatasetApplicationStatusPending,
	}
	dataset := &database.Dataset{
		ID:     10,
		Status: types.DatasetStatusNormal,
	}
	userStore.On("FindByUsername", ctx, "admin").Return(database.User{ID: 999, Username: "admin"}, nil)
	appStore.On("FindByID", ctx, int64(1)).Return(pendingApp, nil)
	dsStore.On("ByID", ctx, int64(10)).Return(dataset, nil)
	appStore.On("ReviewApplication", ctx, int64(1), int64(999), "", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, &database.ReviewDatasetUpdate{
		ExpectedDatasetStatus: types.DatasetStatusNormal,
		NewStatus:             types.DatasetStatusListed,
		Price:                 9.99,
		RelatedDatasetID:      20,
		DatasetType:           types.DatasetTypeCommercial,
		RepositoryPrivate:     false,
	}).
		Return(nil, errorx.DatasetAlreadyReferenced(fmt.Errorf("conflict"), errorx.Ctx()))

	req := &types.ReviewDatasetApplicationReq{
		ID:          1,
		Action:      "approve",
		CurrentUser: "admin",
	}
	_, err := c.ReviewDatasetApplication(ctx, req)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrDatasetAlreadyReferenced))
}

func TestReviewDatasetApplication_Approve_ConflictRelatedDatasetReferenced(t *testing.T) {
	ctx := context.TODO()
	c, dsStore, userStore, _, appStore := setupSaaSComponent(t)

	pendingApp := &database.DatasetApplication{
		ID:               1,
		DatasetID:        10,
		Action:           types.DatasetApplicationActionInitial,
		Price:            9.99,
		RelatedDatasetID: 20,
		Status:           types.DatasetApplicationStatusPending,
	}
	dataset := &database.Dataset{
		ID:     10,
		Status: types.DatasetStatusNormal,
	}
	userStore.On("FindByUsername", ctx, "admin").Return(database.User{ID: 999, Username: "admin"}, nil)
	appStore.On("FindByID", ctx, int64(1)).Return(pendingApp, nil)
	dsStore.On("ByID", ctx, int64(10)).Return(dataset, nil)
	appStore.On("ReviewApplication", ctx, int64(1), int64(999), "", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, &database.ReviewDatasetUpdate{
		ExpectedDatasetStatus: types.DatasetStatusNormal,
		NewStatus:             types.DatasetStatusListed,
		Price:                 9.99,
		RelatedDatasetID:      20,
		DatasetType:           types.DatasetTypeCommercial,
		RepositoryPrivate:     false,
	}).
		Return(nil, errorx.RelatedDatasetAlreadyReferenced(fmt.Errorf("conflict"), errorx.Ctx()))

	req := &types.ReviewDatasetApplicationReq{
		ID:          1,
		Action:      "approve",
		CurrentUser: "admin",
	}
	_, err := c.ReviewDatasetApplication(ctx, req)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrRelatedDatasetAlreadyReferenced))
}

func TestReviewDatasetApplication_FSMNotAllowed(t *testing.T) {
	ctx := context.TODO()
	c, _, userStore, _, appStore := setupSaaSComponent(t)

	// Application is already approved, cannot be approved again
	appStore.On("FindByID", ctx, int64(1)).Return(&database.DatasetApplication{
		ID:     1,
		Status: types.DatasetApplicationStatusApproved,
		Action: types.DatasetApplicationActionInitial,
	}, nil)
	userStore.On("FindByUsername", ctx, "admin").Return(database.User{ID: 999, Username: "admin"}, nil)

	req := &types.ReviewDatasetApplicationReq{
		ID:          1,
		Action:      "approve",
		CurrentUser: "admin",
	}
	_, err := c.ReviewDatasetApplication(ctx, req)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrApplicationStatusNotAllowed))
}

func TestResolveApplicationAction(t *testing.T) {
	c, _, _, _, _ := setupSaaSComponent(t)

	tests := []struct {
		name     string
		status   types.DatasetStatus
		action   string
		expected types.DatasetApplicationAction
	}{
		{"normal+list", types.DatasetStatusNormal, "list", types.DatasetApplicationActionInitial},
		{"listed+list", types.DatasetStatusListed, "list", types.DatasetApplicationActionEdit},
		{"delisted+list", types.DatasetStatusDelisted, "list", types.DatasetApplicationActionRelist},
		{"listed+delist", types.DatasetStatusListed, "delist", types.DatasetApplicationActionDelist},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := &database.Dataset{Status: tt.status}
			result := c.resolveApplicationAction(ds, tt.action)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestListDatasetApplications(t *testing.T) {
	ctx := context.TODO()
	c, _, _, _, appStore := setupSaaSComponent(t)

	apps := []*database.DatasetApplication{
		{ID: 1, Status: types.DatasetApplicationStatusPending, Action: types.DatasetApplicationActionInitial},
		{ID: 2, Status: types.DatasetApplicationStatusApproved, Action: types.DatasetApplicationActionEdit},
	}
	appStore.On("List", ctx, "", "", 10, 1).Return(apps, 2, nil)

	req := &types.ListDatasetApplicationsReq{Per: 10, Page: 1}
	result, total, err := c.ListDatasetApplications(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Len(t, result, 2)
}

func TestListDatasetApplications_WithStatus(t *testing.T) {
	ctx := context.TODO()
	c, _, _, _, appStore := setupSaaSComponent(t)

	apps := []*database.DatasetApplication{
		{ID: 1, Status: types.DatasetApplicationStatusPending, Action: types.DatasetApplicationActionInitial},
	}
	appStore.On("List", ctx, "pending", "", 10, 1).Return(apps, 1, nil)

	req := &types.ListDatasetApplicationsReq{Per: 10, Page: 1, Status: "pending"}
	result, total, err := c.ListDatasetApplications(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, result, 1)
}

func TestListDatasetApplications_WithSearch(t *testing.T) {
	ctx := context.TODO()
	c, _, _, _, appStore := setupSaaSComponent(t)

	apps := []*database.DatasetApplication{
		{ID: 1, Status: types.DatasetApplicationStatusPending, Action: types.DatasetApplicationActionInitial},
	}
	appStore.On("List", ctx, "", "my-dataset", 10, 1).Return(apps, 1, nil)

	req := &types.ListDatasetApplicationsReq{Per: 10, Page: 1, Search: "my-dataset"}
	result, total, err := c.ListDatasetApplications(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, result, 1)
}
