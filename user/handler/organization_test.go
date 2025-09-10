package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/api/httpbase"
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
