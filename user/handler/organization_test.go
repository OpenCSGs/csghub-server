package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockapicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestOrganizationHandler_Create(t *testing.T) {
	newCreateTestCtx := func(t *testing.T) (*httptest.ResponseRecorder, *gin.Context) {
		t.Helper()
		gin.SetMode(gin.TestMode)
		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		httpbase.SetCurrentUser(ginc, "user1")
		reqBody, err := json.Marshal(types.CreateOrgReq{Name: "org1"})
		require.Nil(t, err)
		ginc.Request = httptest.NewRequest("POST", "/api/v1/organizations", bytes.NewBuffer(reqBody))
		ginc.Request.Header.Set("Content-Type", "application/json")
		return response, ginc
	}

	t.Run("create organization successfully", func(t *testing.T) {
		response, ginc := newCreateTestCtx(t)

		mockSensitiveComp := mockapicomp.NewMockSensitiveComponent(t)
		mockSensitiveComp.EXPECT().CheckRequestV2(mock.Anything, mock.Anything).Return(true, nil)
		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		mockOrgComp.EXPECT().Create(mock.Anything, mock.Anything).Return(&types.Organization{
			Name: "org1",
		}, nil)
		h := &OrganizationHandler{
			c:  mockOrgComp,
			sc: mockSensitiveComp,
		}
		h.Create(ginc)
		require.Equal(t, 200, response.Code)
	})

	t.Run("create organization with existing namespace", func(t *testing.T) {
		response, ginc := newCreateTestCtx(t)

		mockSensitiveComp := mockapicomp.NewMockSensitiveComponent(t)
		mockSensitiveComp.EXPECT().CheckRequestV2(mock.Anything, mock.Anything).Return(true, nil)
		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		mockOrgComp.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errorx.NamespaceAlreadyExists("org1"))
		h := &OrganizationHandler{
			c:  mockOrgComp,
			sc: mockSensitiveComp,
		}
		h.Create(ginc)
		require.Equal(t, 400, response.Code)
		var r httpbase.R
		err := json.Unmarshal(response.Body.Bytes(), &r)
		require.Nil(t, err)
		require.Equal(t, "USER-ERR-20", r.Code)
	})

	t.Run("create organization with server error", func(t *testing.T) {
		response, ginc := newCreateTestCtx(t)

		mockSensitiveComp := mockapicomp.NewMockSensitiveComponent(t)
		mockSensitiveComp.EXPECT().CheckRequestV2(mock.Anything, mock.Anything).Return(true, nil)
		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		mockOrgComp.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errors.New("internal error"))
		h := &OrganizationHandler{
			c:  mockOrgComp,
			sc: mockSensitiveComp,
		}
		h.Create(ginc)
		require.Equal(t, 500, response.Code)
	})
}

func TestOrganizationHandler_Index(t *testing.T) {
	response := httptest.NewRecorder()
	ginc, _ := gin.CreateTestContext(response)
	ginc.Request = httptest.NewRequest("GET", "/api/v1/organizations?search=org1&per=10&page=1", nil)

	dborgs := []types.Organization{
		{
			Name: "org1",
		},
	}
	mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
	mockOrgComp.EXPECT().Index(mock.Anything, "org1", 10, 1, "", "", "").Return(dborgs, 1, nil)
	h := &OrganizationHandler{
		c: mockOrgComp,
	}
	h.Index(ginc)
	require.Equal(t, 200, response.Code)
	var r orgsResponse
	err := json.Unmarshal(response.Body.Bytes(), &r)
	require.Nil(t, err)
	require.NotEmpty(t, r.Data)
	require.Equal(t, 1, len(r.Data.Orgs))
	require.Equal(t, 1, r.Data.Total)
}

// TestOrganizationHandler_Get verifies detail mode validation through the HTTP route.
func TestOrganizationHandler_Get(t *testing.T) {
	newGetTestContext := func(t *testing.T) (*httptest.ResponseRecorder, *gin.Engine) {
		t.Helper()
		gin.SetMode(gin.TestMode)
		response := httptest.NewRecorder()
		return response, gin.New()
	}

	for _, test := range []struct {
		name                     string
		currentHierarchical      bool
		organizationHierarchical bool
		status                   int
	}{
		{name: "single mode reads single organization", status: 200},
		{name: "single mode rejects hierarchical organization", organizationHierarchical: true, status: 400},
		{name: "hierarchical mode reads hierarchical organization", currentHierarchical: true, organizationHierarchical: true, status: 200},
		{name: "hierarchical mode rejects single organization", currentHierarchical: true, status: 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, router := newGetTestContext(t)
			mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
			mockOrgComp.EXPECT().Get(mock.Anything, "org1").Return(&types.Organization{
				Name:           "org1",
				IsHierarchical: test.organizationHierarchical,
			}, nil)
			h := &OrganizationHandler{c: mockOrgComp, isHierarchical: test.currentHierarchical}

			router.GET("/api/v1/organization/:namespace", h.Get)
			router.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/organization/org1", nil))

			require.Equal(t, test.status, response.Code)
			if test.status == 400 {
				var result httpbase.R
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
				require.Equal(t, "ORG-ERR-4", result.Code)
			}
		})
	}
}

func TestOrganizationHandler_ListUserOrgs(t *testing.T) {
	t.Run("list user orgs successfully", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		ginc.Request = httptest.NewRequest("GET", "/api/v1/user/user1/organizations?search=org&per=10&page=1", nil)
		ginc.Params = gin.Params{{Key: "username", Value: "user1"}}

		dborgs := []types.Organization{
			{Name: "org1"},
			{Name: "org2"},
		}
		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		mockOrgComp.EXPECT().ListUserOrgs(mock.Anything, &types.ListUserOrgsReq{
			Username: "user1", Search: "org", Per: 10, Page: 1,
		}).Return(dborgs, 2, nil)
		h := &OrganizationHandler{
			c:              mockOrgComp,
			isHierarchical: true,
		}
		h.ListUserOrgs(ginc)
		require.Equal(t, 200, response.Code)
		var r orgsResponse
		err := json.Unmarshal(response.Body.Bytes(), &r)
		require.Nil(t, err)
		require.Equal(t, 2, len(r.Data.Orgs))
		require.Equal(t, 2, r.Data.Total)
	})

	t.Run("list user orgs with server error", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		ginc.Request = httptest.NewRequest("GET", "/api/v1/user/user1/organizations", nil)
		ginc.Params = gin.Params{{Key: "username", Value: "user1"}}

		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		mockOrgComp.EXPECT().ListUserOrgs(mock.Anything, &types.ListUserOrgsReq{
			Username: "user1", Per: 50, Page: 1,
		}).Return(nil, 0, errors.New("internal error"))
		h := &OrganizationHandler{
			c: mockOrgComp,
		}
		h.ListUserOrgs(ginc)
		require.Equal(t, 500, response.Code)
	})
}

// TestOrganizationHandler_ListCurrentUserWritableNamespaces returns writable namespaces for the authenticated user.
func TestOrganizationHandler_ListCurrentUserWritableNamespaces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	ginc, _ := gin.CreateTestContext(response)
	ginc.Request = httptest.NewRequest("GET", "/api/v1/namespaces/mine/writable", nil)
	httpbase.SetCurrentUser(ginc, "current-user")

	mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
	mockOrgComp.EXPECT().ListCurrentUserWritableNamespaces(mock.Anything, "current-user").Return([]types.WritableNamespace{
		{Path: "write-org", Type: "organization", Name: "Write Org", UUID: "write-org-uuid"},
	}, nil).Once()
	h := &OrganizationHandler{c: mockOrgComp}
	h.ListCurrentUserWritableNamespaces(ginc)

	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), "write-org")
}

type orgsResponse struct {
	Code int              `json:"code"`
	Msg  string           `json:"msg"`
	Data orgsResponseData `json:"data"`
}

type orgsResponseData struct {
	Orgs  []types.Organization `json:"data"`
	Total int                  `json:"total"`
}

func TestOrganizationHandler_GetByUUID(t *testing.T) {
	t.Run("get organization by uuid successfully", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		ginc.Request = httptest.NewRequest("GET", "/api/v1/organization/uuid/test-uuid-123", nil)
		ginc.Params = gin.Params{{Key: "uuid", Value: "test-uuid-123"}}

		dborg := types.Organization{
			Name:           "org1",
			Nickname:       "Organization 1",
			IsHierarchical: true,
		}
		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		mockOrgComp.EXPECT().GetByUUID(mock.Anything, "test-uuid-123").Return(&dborg, nil)
		h := &OrganizationHandler{
			c:              mockOrgComp,
			isHierarchical: true,
		}
		h.GetByUUID(ginc)
		require.Equal(t, 200, response.Code)
		var r types.Response
		err := json.Unmarshal(response.Body.Bytes(), &r)
		require.Nil(t, err)
		require.NotNil(t, r.Data)
	})

	t.Run("reject mismatched organization mode", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		ginc.Request = httptest.NewRequest("GET", "/api/v1/organization/uuid/hierarchy-uuid", nil)
		ginc.Params = gin.Params{{Key: "uuid", Value: "hierarchy-uuid"}}

		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		mockOrgComp.EXPECT().GetByUUID(mock.Anything, "hierarchy-uuid").Return(&types.Organization{
			Name:           "hierarchy-org",
			IsHierarchical: true,
		}, nil)
		h := &OrganizationHandler{c: mockOrgComp, isHierarchical: false}
		h.GetByUUID(ginc)

		require.Equal(t, 400, response.Code)
		var result httpbase.R
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.Equal(t, "ORG-ERR-4", result.Code)
	})

	t.Run("get organization by uuid with empty uuid", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		ginc.Request = httptest.NewRequest("GET", "/api/v1/organization/uuid/", nil)
		ginc.Params = gin.Params{{Key: "uuid", Value: ""}}

		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		h := &OrganizationHandler{
			c: mockOrgComp,
		}
		h.GetByUUID(ginc)
		require.Equal(t, 400, response.Code)
	})

	t.Run("get organization by uuid not found", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		ginc.Request = httptest.NewRequest("GET", "/api/v1/organization/uuid/non-existent-uuid", nil)
		ginc.Params = gin.Params{{Key: "uuid", Value: "non-existent-uuid"}}

		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		mockOrgComp.EXPECT().GetByUUID(mock.Anything, "non-existent-uuid").Return(nil, errorx.ErrDatabaseNoRows)
		h := &OrganizationHandler{
			c: mockOrgComp,
		}
		h.GetByUUID(ginc)
		require.Equal(t, 404, response.Code)
	})

	t.Run("get organization by uuid with server error", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		ginc.Request = httptest.NewRequest("GET", "/api/v1/organization/uuid/test-uuid-error", nil)
		ginc.Params = gin.Params{{Key: "uuid", Value: "test-uuid-error"}}

		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		mockOrgComp.EXPECT().GetByUUID(mock.Anything, "test-uuid-error").Return(nil, errors.New("internal server error"))
		h := &OrganizationHandler{
			c: mockOrgComp,
		}
		h.GetByUUID(ginc)
		require.Equal(t, 500, response.Code)
	})
}
