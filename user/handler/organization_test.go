package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockapicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	usercomponent "opencsg.com/csghub-server/user/component"
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

// TestOrganizationHandler_Index preserves the public organization list response and pagination.
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

// TestOrganizationHandler_QueryRequestContext preserves administrator grants and ordinary query behavior without Gin fallback.
func TestOrganizationHandler_QueryRequestContext(t *testing.T) {
	for _, authorized := range []bool{false, true} {
		for _, canceled := range []bool{false, true} {
			for _, operation := range []string{"index", "get"} {
				t.Run(fmt.Sprintf("%s/admin=%t/canceled=%t", operation, authorized, canceled), func(t *testing.T) {
					requestContext, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
					t.Cleanup(cancel)
					type requestKey struct{}
					requestContext = context.WithValue(requestContext, requestKey{}, "request-value")
					if authorized {
						requestContext = types.WithAdminAuthorization(requestContext)
					}
					if canceled {
						cancel()
					}
					ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
					ctx.Request = httptest.NewRequest(http.MethodGet, "/organizations?search=acme&per=7&page=2&org_type=company&verify_status=approved&tag=AI", nil).WithContext(requestContext)
					org := types.Organization{Name: "acme"}
					component := mockcomp.NewMockOrganizationComponent(t)
					checkContext := func(actual context.Context) {
						assertVerificationRequestContext(t, actual, requestContext, authorized)
						require.Equal(t, "request-value", actual.Value(requestKey{}))
					}
					h := &OrganizationHandler{c: component}
					if operation == "index" {
						component.EXPECT().Index(requestContext, "acme", 7, 2, "company", "approved", "AI").Run(func(actual context.Context, _ string, _, _ int, _, _, _ string) {
							checkContext(actual)
						}).Return([]types.Organization{org}, 1, nil).Once()
						h.Index(ctx)
					} else {
						ctx.Params = gin.Params{{Key: "namespace", Value: "acme"}}
						component.EXPECT().Get(requestContext, "acme").Run(func(actual context.Context, _ string) { checkContext(actual) }).Return(&org, nil).Once()
						h.Get(ctx)
					}
					require.Equal(t, http.StatusOK, ctx.Writer.Status())
				})
			}
		}
	}
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

// TestOrganizationHandler_CreateAdminContext preserves the request grant, audit identity and actual operator.
func TestOrganizationHandler_CreateAdminContext(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext := types.WithAdminAuthorization(context.Background())
	ctx.Request = httptest.NewRequest("POST", "/organizations", bytes.NewBufferString(`{"name":"acme","username":"forged"}`)).WithContext(requestContext)
	ctx.Request.Header.Set("Content-Type", "application/json")
	httpbase.SetCurrentUser(ctx, "operator")
	httpbase.SetCurrentUserUUID(ctx, "operator-uuid")
	httpbase.SetIPAddress(ctx, "192.0.2.1")
	httpbase.SetAccessToken(ctx, "test-token")
	httpbase.SetAuthType(ctx, httpbase.AuthTypeJwt)
	requestContext = ctx.Request.Context()
	require.True(t, types.IsAdminAuthorized(requestContext))
	require.Equal(t, "operator", requestContext.Value("currentUser"))
	require.Equal(t, "operator-uuid", requestContext.Value("currentUserUUID"))
	require.Equal(t, "192.0.2.1", requestContext.Value("ip_address"))
	require.Equal(t, "test-token", requestContext.Value("accessToken"))
	require.Equal(t, "JWT", requestContext.Value("authType"))
	sensitive := mockapicomp.NewMockSensitiveComponent(t)
	sensitive.EXPECT().CheckRequestV2(ctx, mock.Anything).Return(true, nil).Once()
	orgs := mockcomp.NewMockOrganizationComponent(t)
	orgs.EXPECT().Create(requestContext, mock.MatchedBy(func(req *types.CreateOrgReq) bool { return req.Username == "operator" && req.Name == "acme" })).Return(&types.Organization{Name: "acme"}, nil).Once()
	h := &OrganizationHandler{c: orgs, sc: sensitive}
	h.Create(ctx)
	require.Equal(t, 200, ctx.Writer.Status())
}

// organizationVerificationContextMock embeds the private component method and mocks the two HTTP operations.
type organizationVerificationContextMock struct {
	usercomponent.OrganizationVerifyComponent
	mock.Mock
}

// Get records the request context and target organization supplied by the verification handler.
func (m *organizationVerificationContextMock) Get(ctx context.Context, path string) (*database.OrganizationVerify, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(*database.OrganizationVerify), args.Error(1)
}

// Update records the request context and unchanged verification update arguments.
func (m *organizationVerificationContextMock) Update(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*database.OrganizationVerify, error) {
	args := m.Called(ctx, id, status, reason)
	return args.Get(0).(*database.OrganizationVerify), args.Error(1)
}

// TestOrganizationHandler_VerificationRequestContext covers admin and ordinary verification reads and updates.
func TestOrganizationHandler_VerificationRequestContext(t *testing.T) {
	for _, authorized := range []bool{false, true} {
		for _, canceled := range []bool{false, true} {
			for _, operation := range []string{"get", "update"} {
				t.Run(fmt.Sprintf("%s/admin=%t/canceled=%t", operation, authorized, canceled), func(t *testing.T) {
					requestContext, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
					t.Cleanup(cancel)
					if authorized {
						requestContext = types.WithAdminAuthorization(requestContext)
					}
					if canceled {
						cancel()
					}
					ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
					ctx.Request = httptest.NewRequest(http.MethodPut, "/organization/verify/42", strings.NewReader(`{"status":"approved"}`)).WithContext(requestContext)
					ctx.Request.Header.Set("Content-Type", "application/json")
					verification := &database.OrganizationVerify{ID: 42, Name: "target-org", Status: types.VerifyStatusApproved}
					component := &organizationVerificationContextMock{}
					component.Test(t)
					t.Cleanup(func() { component.AssertExpectations(t) })
					checkContext := func(args mock.Arguments) {
						assertVerificationRequestContext(t, args.Get(0).(context.Context), requestContext, authorized)
					}
					h := &OrganizationHandler{ov: component}
					if operation == "get" {
						ctx.Params = gin.Params{{Key: "namespace", Value: "target-org"}}
						component.On("Get", requestContext, "target-org").Run(checkContext).Return(verification, nil).Once()
						h.GetVerify(ctx)
					} else {
						ctx.Params = gin.Params{{Key: "id", Value: "42"}}
						component.On("Update", requestContext, int64(42), types.VerifyStatusApproved, "").Run(checkContext).Return(verification, nil).Once()
						h.UpdateVerify(ctx)
					}
					require.Equal(t, http.StatusOK, ctx.Writer.Status())
				})
			}
		}
	}
}

// TestOrganizationHandler_Choices preserves selector envelopes, query validation, and edition-mode errors.
func TestOrganizationHandler_Choices(t *testing.T) {
	t.Run("list projection", func(t *testing.T) {
		component := mockcomp.NewMockOrganizationComponent(t)
		req := types.OrganizationChoiceListRequest{Search: "org", Per: 10, Page: 2, OrgType: "school", VerifyStatus: "verified", Tag: "industry"}
		component.EXPECT().OrganizationChoices(mock.Anything, req).Return([]types.OrganizationChoice{{Name: "org", Nickname: "Organization", Logo: "logo.png"}}, 11, nil).Once()
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/organizations?search=org&per=10&page=2&org_type=school&verify_status=verified&tag=industry", nil)
		(&OrganizationHandler{c: component}).OrganizationChoices(ctx)
		require.Equal(t, http.StatusOK, response.Code)
		require.JSONEq(t, `{"msg":"OK","data":{"data":[{"path":"org","name":"Organization","logo":"logo.png","uuid":"00000000-0000-0000-0000-000000000000"}],"total":11}}`, response.Body.String())
	})
	for _, tc := range []struct {
		name   string
		mode   bool
		err    error
		status int
	}{
		{name: "recipient namespace", status: http.StatusOK},
		{name: "incompatible mode", mode: true, status: http.StatusBadRequest},
		{name: "missing organization", err: errorx.ErrDatabaseNoRows, status: http.StatusNotFound},
		{name: "store failure", err: errors.New("store unavailable"), status: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			component := mockcomp.NewMockOrganizationComponent(t)
			component.EXPECT().OrganizationChoice(mock.Anything, "org").Return(&types.OrganizationChoice{Name: "org", IsHierarchical: tc.mode,
				Namespace: &types.NamespaceChoice{Path: "org", Type: "organization", UUID: "org-ns"}}, tc.err).Once()
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/organization/org", nil)
			ctx.AddParam("namespace", "org")
			(&OrganizationHandler{c: component}).OrganizationChoice(ctx)
			require.Equal(t, tc.status, response.Code)
			if tc.status == http.StatusOK {
				require.JSONEq(t, `{"msg":"OK","data":{"path":"org","name":"","logo":"","uuid":"00000000-0000-0000-0000-000000000000","namespace":{"Path":"org","Type":"organization","UUID":"org-ns"}}}`, response.Body.String())
			}
		})
	}
	t.Run("invalid pagination", func(t *testing.T) {
		component := mockcomp.NewMockOrganizationComponent(t)
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/organizations?page=invalid", nil)
		(&OrganizationHandler{c: component}).OrganizationChoices(ctx)
		require.Equal(t, http.StatusBadRequest, response.Code)
	})
}
