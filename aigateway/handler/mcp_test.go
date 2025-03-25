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
	apicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	comType "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewTestMCPProxyHandler(mockSpaceComp component.SpaceComponent) (MCPProxyHandler, error) {
	return &MCPProxyHandlerImpl{
		spaceComp: mockSpaceComp,
	}, nil
}

func TestOpenAIHandler_List(t *testing.T) {
	mockSpaceComp := apicomp.NewMockSpaceComponent(t)

	handler, err := NewTestMCPProxyHandler(mockSpaceComp)
	require.Nil(t, err)

	repoFilter := new(comType.RepoFilter)
	repoFilter.Username = "testuser"
	repoFilter.SpaceSDK = comType.MCPSERVER.Name

	mcps := []*comType.MCPService{
		{
			ID:   1,
			Name: "mcp1",
		},
	}

	mockSpaceComp.EXPECT().MCPIndex(mock.Anything, repoFilter, 10, 1).Return(mcps, 1, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
		URL: &url.URL{
			RawQuery: "per=10&page=1",
		},
	}
	httpbase.SetCurrentUser(c, "testuser")

	handler.List(c)

	assert.Equal(t, http.StatusOK, w.Code)

	type Resp struct {
		Total int                   `json:"total"`
		Data  []*comType.MCPService `json:"data"`
	}
	var response Resp
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.Nil(t, err)
	require.Equal(t, 1, response.Total)
	require.Equal(t, mcps, response.Data)
}
