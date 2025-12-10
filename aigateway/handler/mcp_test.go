package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	gwmockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	apicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	gwcomp "opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	comType "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewTestMCPProxyHandler(mockSpaceComp component.SpaceComponent, mockMCPResComp gwcomp.MCPResourceComponent) (MCPProxyHandler, error) {
	return &MCPProxyHandlerImpl{
		spaceComp:  mockSpaceComp,
		mcpResComp: mockMCPResComp,
	}, nil
}

func TestMCPHandler_ResourceList(t *testing.T) {
	mockSpaceComp := apicomp.NewMockSpaceComponent(t)
	mockMCPResComp := gwmockcomp.NewMockMCPResourceComponent(t)

	handler, err := NewTestMCPProxyHandler(mockSpaceComp, mockMCPResComp)
	require.Nil(t, err)

	filter := new(comType.MCPFilter)
	filter.Username = "testuser"
	filter.Page = 1
	filter.Per = 10

	mcps := []database.MCPResource{
		{
			ID:   1,
			Name: "mcp1",
		},
	}

	mockMCPResComp.EXPECT().List(mock.Anything, filter).Return(mcps, 1, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
		URL: &url.URL{
			RawQuery: "per=10&page=1",
		},
	}
	httpbase.SetCurrentUser(c, "testuser")

	handler.Resources(c)

	assert.Equal(t, http.StatusOK, w.Code)

	type Resp struct {
		Total int                    `json:"total"`
		Data  []database.MCPResource `json:"data"`
	}
	var response Resp
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.Nil(t, err)
	require.Equal(t, 1, response.Total)
	require.Equal(t, mcps, response.Data)
}
