package handler

import (
	"encoding/json"
	"io"
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
	"opencsg.com/csghub-server/common/errorx"
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

func TestMCPHandler_ProxyToApi_UsesHTTPEndpointAsIs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var hitPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(backend.Close)

	mockSpaceComp := apicomp.NewMockSpaceComponent(t)
	mockMCPResComp := gwmockcomp.NewMockMCPResourceComponent(t)
	svcName := "u-wanghj-file-parser-14k"
	currentUser := "testuser"
	mockSpaceComp.EXPECT().
		GetMCPServiceBySvcName(mock.Anything, svcName, currentUser).
		Return(&comType.MCPService{
			SvcName:  svcName,
			Endpoint: backend.URL,
		}, nil).
		Once()

	handler, err := NewTestMCPProxyHandler(mockSpaceComp, mockMCPResComp)
	require.NoError(t, err)

	router := gin.New()
	router.Any("/v1/mcp/:servicename/*any", func(c *gin.Context) {
		httpbase.SetCurrentUser(c, currentUser)
		httpbase.SetAuthType(c, httpbase.AuthTypeJwt)
		handler.ProxyToApi("")(c)
	})
	gateway := httptest.NewServer(router)
	t.Cleanup(gateway.Close)

	resp, err := http.Get(gateway.URL + "/v1/mcp/" + svcName + "/mcp")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "/mcp", hitPath)
	require.Equal(t, "ok", string(body))
}

func TestMCPHandler_ProxyToApi_SetsProxyHostHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var hitPath, hitHost string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		hitHost = r.Host
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(backend.Close)

	mockSpaceComp := apicomp.NewMockSpaceComponent(t)
	mockMCPResComp := gwmockcomp.NewMockMCPResourceComponent(t)
	svcName := "u-wanghj-file-parser-14k"
	currentUser := "testuser"
	proxyHost := "test-svc.spaces.remote.internal"
	mockSpaceComp.EXPECT().
		GetMCPServiceBySvcName(mock.Anything, svcName, currentUser).
		Return(&comType.MCPService{
			SvcName:   svcName,
			Endpoint:  backend.URL,
			ProxyHost: proxyHost,
		}, nil).
		Once()

	handler, err := NewTestMCPProxyHandler(mockSpaceComp, mockMCPResComp)
	require.NoError(t, err)

	router := gin.New()
	router.Any("/v1/mcp/:servicename/*any", func(c *gin.Context) {
		httpbase.SetCurrentUser(c, currentUser)
		httpbase.SetAuthType(c, httpbase.AuthTypeJwt)
		handler.ProxyToApi("")(c)
	})
	gateway := httptest.NewServer(router)
	t.Cleanup(gateway.Close)

	resp, err := http.Get(gateway.URL + "/v1/mcp/" + svcName + "/mcp")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "/mcp", hitPath)
	require.Equal(t, proxyHost, hitHost)
	require.Equal(t, "ok", string(body))
}

func TestMCPHandler_ProxyToApi_AuthCases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcName := "u-wanghj-file-parser-14k"

	t.Run("anonymous denied", func(t *testing.T) {
		mockSpaceComp := apicomp.NewMockSpaceComponent(t)
		mockMCPResComp := gwmockcomp.NewMockMCPResourceComponent(t)
		handler, err := NewTestMCPProxyHandler(mockSpaceComp, mockMCPResComp)
		require.NoError(t, err)

		router := gin.New()
		router.Any("/v1/mcp/:servicename/*any", handler.ProxyToApi(""))
		gateway := httptest.NewServer(router)
		t.Cleanup(gateway.Close)

		resp, err := http.Get(gateway.URL + "/v1/mcp/" + svcName + "/mcp")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("user org api key rejected", func(t *testing.T) {
		mockSpaceComp := apicomp.NewMockSpaceComponent(t)
		mockMCPResComp := gwmockcomp.NewMockMCPResourceComponent(t)
		handler, err := NewTestMCPProxyHandler(mockSpaceComp, mockMCPResComp)
		require.NoError(t, err)

		router := gin.New()
		router.Any("/v1/mcp/:servicename/*any", func(c *gin.Context) {
			httpbase.SetCurrentUser(c, "apikey-user")
			httpbase.SetAuthType(c, httpbase.AuthTypeUserOrgApiKey)
			handler.ProxyToApi("")(c)
		})
		gateway := httptest.NewServer(router)
		t.Cleanup(gateway.Close)

		resp, err := http.Get(gateway.URL + "/v1/mcp/" + svcName + "/mcp")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("forbidden for private non-member", func(t *testing.T) {
		mockSpaceComp := apicomp.NewMockSpaceComponent(t)
		mockMCPResComp := gwmockcomp.NewMockMCPResourceComponent(t)
		mockSpaceComp.EXPECT().
			GetMCPServiceBySvcName(mock.Anything, svcName, "outsider").
			Return(nil, errorx.ErrForbiddenMsg("no permission")).
			Once()
		handler, err := NewTestMCPProxyHandler(mockSpaceComp, mockMCPResComp)
		require.NoError(t, err)

		router := gin.New()
		router.Any("/v1/mcp/:servicename/*any", func(c *gin.Context) {
			httpbase.SetCurrentUser(c, "outsider")
			httpbase.SetAuthType(c, httpbase.AuthTypeAccessToken)
			handler.ProxyToApi("")(c)
		})
		gateway := httptest.NewServer(router)
		t.Cleanup(gateway.Close)

		resp, err := http.Get(gateway.URL + "/v1/mcp/" + svcName + "/mcp")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("owner allowed for private", func(t *testing.T) {
		var hit bool
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		t.Cleanup(backend.Close)

		mockSpaceComp := apicomp.NewMockSpaceComponent(t)
		mockMCPResComp := gwmockcomp.NewMockMCPResourceComponent(t)
		mockSpaceComp.EXPECT().
			GetMCPServiceBySvcName(mock.Anything, svcName, "owner").
			Return(&comType.MCPService{SvcName: svcName, Endpoint: backend.URL}, nil).
			Once()
		handler, err := NewTestMCPProxyHandler(mockSpaceComp, mockMCPResComp)
		require.NoError(t, err)

		router := gin.New()
		router.Any("/v1/mcp/:servicename/*any", func(c *gin.Context) {
			httpbase.SetCurrentUser(c, "owner")
			httpbase.SetAuthType(c, httpbase.AuthTypeJwt)
			handler.ProxyToApi("")(c)
		})
		gateway := httptest.NewServer(router)
		t.Cleanup(gateway.Close)

		resp, err := http.Get(gateway.URL + "/v1/mcp/" + svcName + "/mcp")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.True(t, hit)
	})

	t.Run("authenticated user allowed for public", func(t *testing.T) {
		var hit bool
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		t.Cleanup(backend.Close)

		mockSpaceComp := apicomp.NewMockSpaceComponent(t)
		mockMCPResComp := gwmockcomp.NewMockMCPResourceComponent(t)
		mockSpaceComp.EXPECT().
			GetMCPServiceBySvcName(mock.Anything, svcName, "anyuser").
			Return(&comType.MCPService{SvcName: svcName, Endpoint: backend.URL}, nil).
			Once()
		handler, err := NewTestMCPProxyHandler(mockSpaceComp, mockMCPResComp)
		require.NoError(t, err)

		router := gin.New()
		router.Any("/v1/mcp/:servicename/*any", func(c *gin.Context) {
			httpbase.SetCurrentUser(c, "anyuser")
			httpbase.SetAuthType(c, httpbase.AuthTypeAccessToken)
			handler.ProxyToApi("")(c)
		})
		gateway := httptest.NewServer(router)
		t.Cleanup(gateway.Close)

		resp, err := http.Get(gateway.URL + "/v1/mcp/" + svcName + "/mcp")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.True(t, hit)
	})
}
