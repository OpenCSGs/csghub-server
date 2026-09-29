package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	deploycommon "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type RProxyTester struct {
	ctx     *gin.Context
	handler *RProxyHandler
	mocks   struct {
		space *mockcomponent.MockSpaceComponent
		repo  *mockcomponent.MockRepoComponent
	}
}

func NewRProxyTester(t *testing.T) *RProxyTester {
	r := &RProxyTester{}
	r.ctx, _ = gin.CreateTestContext(nil)
	r.ctx.Request, _ = http.NewRequest("GET", "/test", nil)
	r.mocks.space = mockcomponent.NewMockSpaceComponent(t)
	r.mocks.repo = mockcomponent.NewMockRepoComponent(t)

	r.handler = &RProxyHandler{
		spaceComp: r.mocks.space,
		repoComp:  r.mocks.repo,
	}

	return r
}

type customUIProxyComponentStub struct {
	info    *component.AgentCustomUIProxyInfo
	matched bool
	err     error
}

type rproxyDeployComponentStub struct {
	deploy *database.Deploy
	err    error
}

func (s *rproxyDeployComponentStub) GetDeployBySvcName(context.Context, string) (*database.Deploy, error) {
	return s.deploy, s.err
}

func (s *customUIProxyComponentStub) Resolve(context.Context, string) (*component.AgentCustomUIProxyInfo, bool, error) {
	return s.info, s.matched, s.err
}

func (t *RProxyTester) WithAuthType(authType httpbase.AuthType) *RProxyTester {
	httpbase.SetAuthType(t.ctx, authType)
	return t
}

func (t *RProxyTester) WithUser(username string) *RProxyTester {
	httpbase.SetCurrentUser(t.ctx, username)
	return t
}

func TestRProxyHandler_CheckAccessPermission(t *testing.T) {
	tests := []struct {
		name           string
		hasSpace       bool
		spaceSDK       string
		authType       httpbase.AuthType
		expectedAllow  bool
		expectedError  bool
		expectSpaceGet bool
		expectRepoCall bool
		repoAllow      bool
		repoError      bool
	}{
		{
			name:           "Non-MCP space with JWT auth",
			hasSpace:       true,
			spaceSDK:       "gradio",
			authType:       httpbase.AuthTypeJwt,
			expectedAllow:  true,
			expectedError:  false,
			expectSpaceGet: true,
			expectRepoCall: true,
			repoAllow:      true,
			repoError:      false,
		},
		{
			name:           "Non-MCP space with AccessToken auth",
			hasSpace:       true,
			spaceSDK:       "gradio",
			authType:       httpbase.AuthTypeAccessToken,
			expectedAllow:  true,
			expectedError:  false,
			expectSpaceGet: true,
			expectRepoCall: true,
			repoAllow:      true,
			repoError:      false,
		},
		{
			name:           "Non-MCP space with ApiKey auth",
			hasSpace:       true,
			spaceSDK:       "gradio",
			authType:       httpbase.AuthTypeSystemApiKey,
			expectedAllow:  false,
			expectedError:  true,
			expectSpaceGet: true,
			expectRepoCall: false,
			repoAllow:      false,
			repoError:      false,
		},
		{
			// AuthTypeUserOrgApiKey and AuthTypeMultiSyncToken are rejected
			// before any component call, so no space lookup happens.
			name:           "Non-MCP space with MultiSyncToken auth",
			hasSpace:       true,
			spaceSDK:       "gradio",
			authType:       httpbase.AuthTypeMultiSyncToken,
			expectedAllow:  false,
			expectedError:  true,
			expectSpaceGet: false,
			expectRepoCall: false,
			repoAllow:      false,
			repoError:      false,
		},
		{
			name:           "MCP space with JWT auth",
			hasSpace:       true,
			spaceSDK:       types.MCPSERVER.Name,
			authType:       httpbase.AuthTypeJwt,
			expectedAllow:  true,
			expectedError:  false,
			expectSpaceGet: true,
			expectRepoCall: true,
			repoAllow:      true,
			repoError:      false,
		},
		{
			name:           "MCP space with AccessToken auth",
			hasSpace:       true,
			spaceSDK:       types.MCPSERVER.Name,
			authType:       httpbase.AuthTypeAccessToken,
			expectedAllow:  true,
			expectedError:  false,
			expectSpaceGet: true,
			expectRepoCall: true,
			repoAllow:      true,
			repoError:      false,
		},
		{
			name:           "MCP space with ApiKey auth",
			hasSpace:       true,
			spaceSDK:       types.MCPSERVER.Name,
			authType:       httpbase.AuthTypeSystemApiKey,
			expectedAllow:  true,
			expectedError:  false,
			expectSpaceGet: true,
			expectRepoCall: true,
			repoAllow:      true,
			repoError:      false,
		},
		{
			name:           "Non-space case",
			hasSpace:       false,
			spaceSDK:       "",
			authType:       httpbase.AuthTypeJwt,
			expectedAllow:  true,
			expectedError:  false,
			expectSpaceGet: false,
			expectRepoCall: true,
			repoAllow:      true,
			repoError:      false,
		},
		{
			name:           "Space get error",
			hasSpace:       true,
			spaceSDK:       "gradio",
			authType:       httpbase.AuthTypeJwt,
			expectedAllow:  false,
			expectedError:  true,
			expectSpaceGet: false,
			expectRepoCall: false,
			repoAllow:      false,
			repoError:      false,
		},
		{
			name:           "Repo allow access error",
			hasSpace:       true,
			spaceSDK:       "gradio",
			authType:       httpbase.AuthTypeJwt,
			expectedAllow:  false,
			expectedError:  true,
			expectSpaceGet: true,
			expectRepoCall: true,
			repoAllow:      false,
			repoError:      true,
		},
		{
			name:           "Public MCP space",
			hasSpace:       true,
			spaceSDK:       types.MCPSERVER.Name,
			expectedAllow:  true,
			expectedError:  false,
			expectSpaceGet: true,
			expectRepoCall: true,
			repoAllow:      true,
			repoError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := NewRProxyTester(t).WithAuthType(tt.authType).WithUser("testuser")

			deploy := &database.Deploy{}
			if tt.hasSpace {
				deploy.SpaceID = 1
				deploy.RepoID = 1
				if tt.expectSpaceGet {
					tester.mocks.space.EXPECT().GetByID(tester.ctx.Request.Context(), int64(1)).Return(&database.Space{Sdk: tt.spaceSDK}, nil)
					if tt.expectRepoCall {
						if tt.repoError {
							if tt.spaceSDK == types.MCPSERVER.Name {
								tester.mocks.repo.EXPECT().AllowAccessEndpoint(tester.ctx.Request.Context(), "testuser", deploy).Return(false, errors.New("repo access error"))
							} else {
								tester.mocks.repo.EXPECT().AllowAccessByRepoID(tester.ctx.Request.Context(), int64(1), "testuser").Return(false, errors.New("repo access error"))
							}
						} else {
							if tt.spaceSDK == types.MCPSERVER.Name {
								tester.mocks.repo.EXPECT().AllowAccessEndpoint(tester.ctx.Request.Context(), "testuser", deploy).Return(tt.repoAllow, nil)
							} else {
								tester.mocks.repo.EXPECT().AllowAccessByRepoID(tester.ctx.Request.Context(), int64(1), "testuser").Return(tt.repoAllow, nil)
							}
						}
					}
				} else if tt.expectedError && tt.repoError == false && tt.authType == httpbase.AuthTypeMultiSyncToken {
					// AuthTypeUserOrgApiKey and AuthTypeMultiSyncToken are
					// rejected before any component call, so no space lookup
					// happens and no mock expectation is needed.
				} else {
					tester.mocks.space.EXPECT().GetByID(tester.ctx.Request.Context(), int64(1)).Return(nil, errors.New("space get error"))
				}
			} else {
				if tt.expectRepoCall {
					tester.mocks.repo.EXPECT().AllowAccessEndpoint(tester.ctx.Request.Context(), "testuser", deploy).Return(tt.repoAllow, nil)
				}
			}

			allow, err := tester.handler.checkAccessPermission(tester.ctx, deploy, "testuser")

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedAllow, allow)
		})
	}
}

func TestRProxyHandler_CustomUI(t *testing.T) {
	if !rproxyCustomUIEnabled {
		t.Skip("custom UI routing is available in EE and SaaS")
	}
	var upstreamPath, upstreamAgentBaseURL, upstreamAuthMode, upstreamAgentName, upstreamAuthorization, upstreamCookie string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAgentBaseURL = r.Header.Get(types.CSGHubAgentBaseURLHeader)
		upstreamAuthMode = r.Header.Get(types.CSGHubAgentAuthModeHeader)
		upstreamAgentName = r.Header.Get(types.CSGBotHeaderAgentName)
		upstreamAuthorization = r.Header.Get("Authorization")
		upstreamCookie = r.Header.Get("Cookie")
		_, _ = w.Write([]byte("space frontend"))
	}))
	defer upstream.Close()

	repos := mockcomponent.NewMockRepoComponent(t)

	cfg := &config.Config{}
	cfg.AIGateway.PublicAIGatewayURL = "https://gateway.example.com/v1/"
	recorder := &rproxyTestResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "http://my-agent.space.opencsg.com/assets/app.js", nil)
	ctx.Request.Header.Set("Authorization", "Bearer browser-token")
	ctx.Request.Header.Set("Cookie", "session=browser-session")
	ctx.Request.Header.Set(types.CSGHubAgentBaseURLHeader, "https://attacker.example/target")
	ctx.Request.Header.Set(types.CSGHubAgentAuthModeHeader, types.CSGHubAgentAuthModeUserToken)
	ctx.Request.Header.Set(types.CSGBotHeaderAgentName, "attacker-agent")
	handler := &RProxyHandler{
		repoComp:     repos,
		rproxyDeploy: &rproxyDeployComponentStub{deploy: &database.Deploy{Type: types.SandboxType, Status: deploycommon.Running}},
		cfg:          cfg,
		customUIComp: &customUIProxyComponentStub{matched: true, info: &component.AgentCustomUIProxyInfo{
			AgentContentID: "my-agent", AgentName: "generic-assistant-lj1", ShareName: "s-a-11",
			SpaceSvcName: "ui-space", SpaceEndpoint: upstream.URL, Available: true,
		}},
	}

	handler.Proxy(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "/assets/app.js", upstreamPath)
	require.Equal(t, "https://gateway.example.com/v1/shared/sandboxes/s-a-11", upstreamAgentBaseURL)
	require.Equal(t, types.CSGHubAgentAuthModeAnonymous, upstreamAuthMode)
	require.Equal(t, "generic-assistant-lj1", upstreamAgentName)
	require.Empty(t, upstreamAuthorization)
	require.Empty(t, upstreamCookie)
	body, err := io.ReadAll(recorder.Body)
	require.NoError(t, err)
	require.Equal(t, "space frontend", string(body))

	configRecorder := &rproxyTestResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	configCtx, _ := gin.CreateTestContext(configRecorder)
	configCtx.Request = httptest.NewRequest(http.MethodGet, "http://my-agent.space.opencsg.com/__csgclaw/config", nil)
	handler.Proxy(configCtx)
	require.Equal(t, http.StatusOK, configRecorder.Code)
	require.Equal(t, "space frontend", configRecorder.Body.String())

	endpointRecorder := &rproxyTestResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	endpointCtx, _ := gin.CreateTestContext(endpointRecorder)
	endpointCtx.Request = httptest.NewRequest(http.MethodGet, "http://localhost/endpoint/my-agent/assets/app.js", nil)
	handler.Proxy(endpointCtx)
	require.Equal(t, http.StatusOK, endpointRecorder.Code)
	require.Equal(t, "/assets/app.js", upstreamPath)

	endpointConfigRecorder := &rproxyTestResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	endpointConfigCtx, _ := gin.CreateTestContext(endpointConfigRecorder)
	endpointConfigCtx.Request = httptest.NewRequest(http.MethodGet, "http://localhost/endpoint/my-agent/__csgclaw/config", nil)
	handler.Proxy(endpointConfigCtx)
	require.Equal(t, http.StatusOK, endpointConfigRecorder.Code)
	require.Equal(t, "space frontend", endpointConfigRecorder.Body.String())
}

func TestRProxyHandler_PrivateCustomUIIsDenied(t *testing.T) {
	if !rproxyCustomUIEnabled {
		t.Skip("custom UI routing is available in EE and SaaS")
	}
	repos := mockcomponent.NewMockRepoComponent(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "http://private-agent.space.opencsg.com/", nil)
	(&RProxyHandler{repoComp: repos, rproxyDeploy: &rproxyDeployComponentStub{deploy: &database.Deploy{Type: types.SandboxType, Status: deploycommon.Running}}, customUIComp: &customUIProxyComponentStub{matched: true, err: errorx.ErrForbidden}}).Proxy(ctx)
	require.Equal(t, http.StatusForbidden, ctx.Writer.Status())
}

func TestRProxyHandler_PrivateCustomUIIsDeniedBeforeSpaceLookup(t *testing.T) {
	if !rproxyCustomUIEnabled {
		t.Skip("custom UI routing is available in EE and SaaS")
	}
	repos := mockcomponent.NewMockRepoComponent(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "http://private-agent.space.opencsg.com/", nil)
	(&RProxyHandler{repoComp: repos, rproxyDeploy: &rproxyDeployComponentStub{deploy: &database.Deploy{Type: types.SandboxType, Status: deploycommon.Running}}, customUIComp: &customUIProxyComponentStub{matched: true, err: errorx.ErrForbidden}}).Proxy(ctx)
	require.Equal(t, http.StatusForbidden, ctx.Writer.Status())
}

func TestRProxyHandler_ExistingDeployDoesNotCheckAgentForCustomUI(t *testing.T) {
	if !rproxyCustomUIEnabled {
		t.Skip("custom UI routing is available in EE and SaaS")
	}
	repos := mockcomponent.NewMockRepoComponent(t)
	deploy := &database.Deploy{Type: types.SpaceType, Status: deploycommon.Running}
	repos.EXPECT().AllowAccessEndpoint(mock.Anything, mock.Anything, deploy).Return(false, nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "http://ordinary-space.space.opencsg.com/", nil)
	(&RProxyHandler{repoComp: repos, rproxyDeploy: &rproxyDeployComponentStub{deploy: deploy}}).Proxy(ctx)
	require.Equal(t, http.StatusForbidden, ctx.Writer.Status())
}

func TestRProxyHandler_OrdinaryDeployStripsAgentContextHeaders(t *testing.T) {
	var upstreamBaseURL, upstreamAuthMode, upstreamAgentName string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamBaseURL = r.Header.Get(types.CSGHubAgentBaseURLHeader)
		upstreamAuthMode = r.Header.Get(types.CSGHubAgentAuthModeHeader)
		upstreamAgentName = r.Header.Get(types.CSGBotHeaderAgentName)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	repos := mockcomponent.NewMockRepoComponent(t)
	deploy := &database.Deploy{ID: 1, Type: types.ServerlessType, Status: deploycommon.Running, Endpoint: upstream.URL}
	repos.EXPECT().AllowAccessEndpoint(mock.Anything, mock.Anything, deploy).Return(true, nil)
	recorder := &rproxyTestResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "http://ordinary-service.space.opencsg.com/", nil)
	ctx.Request.Header.Set(types.CSGHubAgentBaseURLHeader, "https://attacker.example/target")
	ctx.Request.Header.Set(types.CSGHubAgentAuthModeHeader, types.CSGHubAgentAuthModeUserToken)
	ctx.Request.Header.Set(types.CSGBotHeaderAgentName, "attacker-agent")

	handler := &RProxyHandler{repoComp: repos, rproxyDeploy: &rproxyDeployComponentStub{deploy: deploy}, cfg: &config.Config{}}
	handler.Proxy(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, upstreamBaseURL)
	require.Empty(t, upstreamAuthMode)
	require.Empty(t, upstreamAgentName)
}

type rproxyTestResponseWriter struct {
	*httptest.ResponseRecorder
}

func (w *rproxyTestResponseWriter) CloseNotify() <-chan bool {
	return make(chan bool)
}
