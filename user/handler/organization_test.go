package handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestOrganizationHandler_Index(t *testing.T) {
	response := httptest.NewRecorder()
	ginc, _ := gin.CreateTestContext(response)
	httpbase.SetCurrentUser(ginc, "user1")
	ginc.Request = httptest.NewRequest("GET", "/api/v1/organizations?search=org1&per=10&page=1", nil)

	dborgs := []types.Organization{
		{
			Name: "org1",
		},
	}
	mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
	mockOrgComp.EXPECT().Index(mock.Anything, "user1", "org1", 10, 1, "", "").Return(dborgs, 1, nil)
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
			Name:     "org1",
			Nickname: "Organization 1",
		}
		mockOrgComp := mockcomp.NewMockOrganizationComponent(t)
		mockOrgComp.EXPECT().GetByUUID(mock.Anything, "test-uuid-123").Return(&dborg, nil)
		h := &OrganizationHandler{
			c: mockOrgComp,
		}
		h.GetByUUID(ginc)
		require.Equal(t, 200, response.Code)
		var r types.Response
		err := json.Unmarshal(response.Body.Bytes(), &r)
		require.Nil(t, err)
		require.NotNil(t, r.Data)
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
