package handler

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
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
			authType:       httpbase.AuthTypeApiKey,
			expectedAllow:  false,
			expectedError:  true,
			expectSpaceGet: true,
			expectRepoCall: false,
			repoAllow:      false,
			repoError:      false,
		},
		{
			name:           "Non-MCP space with MultiSyncToken auth",
			hasSpace:       true,
			spaceSDK:       "gradio",
			authType:       httpbase.AuthTypeMultiSyncToken,
			expectedAllow:  false,
			expectedError:  true,
			expectSpaceGet: true,
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
			authType:       httpbase.AuthTypeApiKey,
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
