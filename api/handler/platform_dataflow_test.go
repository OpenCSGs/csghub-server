package handler

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type PlatformDataflowTester struct {
	*testutil.GinTester
	handler  *PlatformDataflowHandler
	mockComp *mockcomponent.MockPlatformDataflowComponent
}

func NewPlatformDataflowTester(t *testing.T) *PlatformDataflowTester {
	tester := &PlatformDataflowTester{GinTester: testutil.NewGinTester()}
	tester.mockComp = mockcomponent.NewMockPlatformDataflowComponent(t)

	tester.handler = &PlatformDataflowHandler{
		component: tester.mockComp,
	}
	tester.WithParam("uuid", "test-ns-uuid")
	return tester
}

func (t *PlatformDataflowTester) WithHandleFunc(fn func(h *PlatformDataflowHandler) gin.HandlerFunc) *PlatformDataflowTester {
	t.Handler(fn(t.handler))
	return t
}

func (t *PlatformDataflowTester) WithUserAndUUID() *PlatformDataflowTester {
	t.Gctx().Set("currentUser", "testuser")
	t.Gctx().Set("currentUserUUID", "test-user-uuid")
	return t
}

func TestPlatformDataflowHandler_CreateJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.CreateJob
		})
		tester.WithUserAndUUID()

		req := &types.DataflowArgoJobReq{
			ResourceId:  1,
			JobID:       "job-1",
			JobName:     "test-job",
			StorageSize: "10Gi",
			Entrypoint:  "python script.py",
			DagTasks:    []types.ArgoDagTask{{ID: "task1", Name: "task1", Template: "main"}},
		}
		resp := &types.DataflowArgoJobResp{
			ID:         1,
			ArgoTaskID: "argo-task-1",
			JobID:      "job-1",
			JobName:    "test-job",
			Status:     "Pending",
		}

		tester.mockComp.EXPECT().CreateJob(tester.Ctx(), &types.DataflowArgoJobReq{
			OpUserUUID:  "test-user-uuid",
			Username:    "testuser",
			NSUUID:      "test-ns-uuid",
			ResourceId:  1,
			JobID:       "job-1",
			JobName:     "test-job",
			StorageSize: "10Gi",
			Entrypoint:  "python script.py",
			DagTasks:    []types.ArgoDagTask{{ID: "task1", Name: "task1", Template: "main"}},
		}).Return(resp, nil)

		tester.WithBody(t, req).Execute()
		tester.ResponseEq(t, 201, "Created", resp)
	})

	t.Run("missing_ns_uuid", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.CreateJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("uuid", "")

		req := &types.DataflowArgoJobReq{}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("invalid_request_body", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.CreateJob
		})
		tester.WithUserAndUUID()

		tester.Gctx().Request.Body = io.NopCloser(bytes.NewBuffer([]byte("{invalid json")))
		tester.Gctx().Request.Header = map[string][]string{"Content-Type": {"application/json"}}

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("component_error", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.CreateJob
		})
		tester.WithUserAndUUID()

		req := &types.DataflowArgoJobReq{
			ResourceId:  1,
			JobID:       "job-1",
			JobName:     "test-job",
			StorageSize: "10Gi",
			Entrypoint:  "python script.py",
			DagTasks:    []types.ArgoDagTask{{ID: "task1", Name: "task1", Template: "main"}},
		}

		tester.mockComp.EXPECT().CreateJob(tester.Ctx(), mock.Anything).Return(nil, errors.New("some error"))

		tester.WithBody(t, req).Execute()
		tester.ResponseEqCode(t, 500)
	})
}

func TestPlatformDataflowHandler_DeleteJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.DeleteJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")

		tester.mockComp.EXPECT().DeleteJob(tester.Ctx(), &types.DataflowDeleteReq{
			OpUserUUID: "test-user-uuid",
			Username:   "testuser",
			NSUUID:     "test-ns-uuid",
			ArgoTaskID: "argo-task-1",
		}).Return(nil)

		tester.Execute()
		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("missing_task_id", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.DeleteJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "")

		tester.Execute()
		tester.ResponseEqCode(t, 400)
	})

	t.Run("missing_ns_uuid", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.DeleteJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")
		tester.WithParam("uuid", "")

		tester.Execute()
		tester.ResponseEqCode(t, 400)
	})

	t.Run("component_error", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.DeleteJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")

		tester.mockComp.EXPECT().DeleteJob(tester.Ctx(), mock.Anything).Return(errors.New("some error"))

		tester.Execute()
		tester.ResponseEqCode(t, 500)
	})
}

func TestPlatformDataflowHandler_GetJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.GetJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")

		resp := &types.DataflowArgoJobResp{
			ID:         1,
			ArgoTaskID: "argo-task-1",
			JobID:      "job-1",
			JobName:    "test-job",
			Status:     "Running",
		}

		tester.mockComp.EXPECT().GetJob(tester.Ctx(), &types.DataflowArgoJobReq{
			OpUserUUID: "test-user-uuid",
			Username:   "testuser",
			NSUUID:     "test-ns-uuid",
			ArgoTaskID: "argo-task-1",
		}).Return(resp, nil)

		tester.Execute()
		tester.ResponseEq(t, 200, tester.OKText, resp)
	})

	t.Run("missing_task_id", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.GetJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "")

		tester.Execute()
		tester.ResponseEqCode(t, 400)
	})

	t.Run("missing_ns_uuid", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.GetJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")
		tester.WithParam("uuid", "")

		tester.Execute()
		tester.ResponseEqCode(t, 400)
	})

	t.Run("component_error", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.GetJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")

		tester.mockComp.EXPECT().GetJob(tester.Ctx(), mock.Anything).Return(nil, errors.New("some error"))

		tester.Execute()
		tester.ResponseEqCode(t, 500)
	})
}

// TestAcademyProxyOrdinaryRequests preserves anonymous access and authenticated business identity.
func TestAcademyProxyOrdinaryRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, username string
		super          bool
	}{
		{name: "anonymous"},
		{name: "ordinary user", username: "learner"},
		{name: "legacy global grant", username: "learner", super: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			headers := make(chan http.Header, 1)
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers <- r.Header.Clone()
				w.WriteHeader(http.StatusNoContent)
			}))
			defer target.Close()
			rp, err := proxy.NewReverseProxy(target.URL)
			require.NoError(t, err)
			users := mockcomponent.NewMockUserComponent(t)
			usc := mockrpc.NewMockUserSvcClient(t)
			if tc.username != "" {
				user := &database.User{ID: 42, Username: "learner", UUID: "verified-uuid", Email: "learner@example.com"}
				if tc.super {
					user.RoleAssignments = []database.UserRole{{RoleCode: "custom", Role: &database.Role{IsAdmin: true,
						Permissions: []database.AdminRolePermission{{PermissionCode: types.AdminPermissionAll}}}}}
				}
				users.EXPECT().GetUserByName(mock.Anything, tc.username).Return(user, nil)
				usc.EXPECT().GetOrCreateFirstAvaiTokens(mock.Anything, tc.username, tc.username, string(types.AccessTokenAppGit), "dataflow").Return("trusted-token", nil)
			}
			h := &DataflowProxyHandler{rp: rp, user: users, usc: usc, forwardType: ForwardTypeAcademy}
			engine := gin.New()
			engine.Use(func(ctx *gin.Context) { httpbase.SetCurrentUser(ctx, tc.username) })
			engine.Any("/api/v1/academy/*any", h.Proxy)
			server := httptest.NewServer(engine)
			defer server.Close()
			request, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/academy/courses?per=10", nil)
			require.NoError(t, err)
			request.Header.Set("Authorization", "Bearer original-jwt")
			for _, key := range []string{"isadmin", "User-Id", "User-Name", "User-Email", "User-Token", "X-Academy-User-Uuid"} {
				request.Header.Set(key, "spoofed")
			}
			response, err := server.Client().Do(request)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusNoContent, response.StatusCode)
			got := <-headers
			require.Equal(t, "Bearer original-jwt", got.Get("Authorization"))
			require.Equal(t, strconv.FormatBool(tc.super), got.Get("isadmin"))
			if tc.username == "" {
				for _, key := range []string{"User-Id", "User-Name", "User-Email", "User-Token", "X-Academy-User-Uuid"} {
					require.Empty(t, got.Get(key), key)
				}
			} else {
				require.Equal(t, "42", got.Get("User-Id"))
				require.Equal(t, "learner", got.Get("User-Name"))
				require.Equal(t, "learner@example.com", got.Get("User-Email"))
				require.Equal(t, "trusted-token", got.Get("User-Token"))
				require.Equal(t, "verified-uuid", got.Get("X-Academy-User-Uuid"))
			}
		})
	}
}

// TestAcademyProxyRejectsLegacyAdminPaths blocks old management URLs and normalization bypasses.
func TestAcademyProxyRejectsLegacyAdminPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &DataflowProxyHandler{forwardType: ForwardTypeAcademy}
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		httpbase.SetCurrentUser(ctx, "global-admin")
		ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
	})
	engine.Any("/api/v1/academy/*any", h.Proxy)
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace} {
		for _, suffix := range []string{
			"/admin", "/admin/", "/admin/courses", "/admin/collections/1", "/admin/users/u/permissions",
			"//admin/courses", "/courses/../admin/users", "/%61dmin/courses",
			"/%2561dmin/courses", "/%252e%252e/admin/users", "/%2e/admin/courses",
			"/courses/%2e%2e/admin/users", "/%2fadmin/courses", "/admin%2fcourses",
			"/courses//1", "/courses/./1", "/courses/../me", "/courses%5c..%5cadmin/users",
		} {
			t.Run(method+suffix, func(t *testing.T) {
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, httptest.NewRequest(method, "/api/v1/academy"+suffix, nil))
				require.Contains(t, []int{http.StatusBadRequest, http.StatusForbidden}, response.Code)
			})
		}
	}
}

// TestAcademyProxyMappedRequests verifies identity, payload preservation, and safe fixed target IDs.
func TestAcademyProxyMappedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, path           string
		authorized, loggedIn bool
		want                 int
	}{
		{name: "authorized module", path: "course-1", authorized: true, loggedIn: true, want: http.StatusNoContent},
		{name: "encoded scalar ID", path: "course%2d1", authorized: true, loggedIn: true, want: http.StatusNoContent},
		{name: "no authorization", path: "course-1", loggedIn: true, want: http.StatusForbidden},
		{name: "no identity", path: "course-1", authorized: true, want: http.StatusForbidden},
		{name: "dot", path: ".", authorized: true, loggedIn: true, want: http.StatusBadRequest},
		{name: "parent", path: "..", authorized: true, loggedIn: true, want: http.StatusBadRequest},
		{name: "double encoded separator", path: "a%252fb", authorized: true, loggedIn: true, want: http.StatusBadRequest},
		{name: "backslash", path: "a%5cb", authorized: true, loggedIn: true, want: http.StatusBadRequest},
		{name: "encoded separator", path: "a%2fb", authorized: true, loggedIn: true, want: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			received := make(chan map[string]string, 1)
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				received <- map[string]string{"path": r.URL.Path, "query": r.URL.RawQuery, "body": string(body),
					"method": r.Method, "admin": r.Header.Get("isadmin"), "uuid": r.Header.Get("X-Academy-User-Uuid"),
					"user": r.Header.Get("User-Name"), "authorization": r.Header.Get("Authorization")}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer target.Close()
			rp, err := proxy.NewReverseProxy(target.URL)
			require.NoError(t, err)
			users := mockcomponent.NewMockUserComponent(t)
			usc := mockrpc.NewMockUserSvcClient(t)
			if tc.want == http.StatusNoContent {
				users.EXPECT().GetUserByName(mock.Anything, "module-admin").Return(&database.User{Username: "module-admin", UUID: "verified-uuid"}, nil)
				usc.EXPECT().GetOrCreateFirstAvaiTokens(mock.Anything, "module-admin", "module-admin", string(types.AccessTokenAppGit), "dataflow").Return("trusted-token", nil)
			}
			h := &DataflowProxyHandler{rp: rp, user: users, usc: usc, forwardType: ForwardTypeAcademy}
			engine := gin.New()
			engine.Use(func(ctx *gin.Context) {
				if tc.loggedIn {
					httpbase.SetCurrentUser(ctx, "module-admin")
				}
				if tc.authorized {
					ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
				}
			})
			engine.PUT("/api/v1/admin/academy/course/courses/:id", h.AcademyProxyToAPI("/api/v1/academy/admin/courses/%s", "id"))
			server := httptest.NewServer(engine)
			defer server.Close()
			request, err := http.NewRequest(http.MethodPut, server.URL+"/api/v1/admin/academy/course/courses/"+tc.path+"?page=2&search=a%2Fb", bytes.NewBufferString(`{"name":"course"}`))
			require.NoError(t, err)
			request.Header.Set("Authorization", "Bearer original-jwt")
			request.Header.Set("User-Name", "spoofed")
			request.Header.Set("X-Academy-User-Uuid", "spoofed")
			response, err := server.Client().Do(request)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, tc.want, response.StatusCode)
			if tc.want == http.StatusNoContent {
				require.Equal(t, map[string]string{"path": "/api/v1/academy/admin/courses/course-1", "query": "page=2&search=a%2Fb",
					"body": `{"name":"course"}`, "method": http.MethodPut, "admin": "true", "uuid": "verified-uuid",
					"user": "module-admin", "authorization": "Bearer original-jwt"}, <-received)
			} else {
				require.Empty(t, received)
			}
		})
	}
}

// TestAcademyProxyModuleIdentity maps only locally authorized academy requests to the legacy service flag.
func TestAcademyProxyModuleIdentity(t *testing.T) {
	for _, service := range []ForwardType{ForwardTypeAcademy, ForwardTypeDataflow, ForwardTypeSupplydemand} {
		for _, authorized := range []bool{false, true} {
			t.Run(string(service)+strconv.FormatBool(authorized), func(t *testing.T) {
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, strconv.FormatBool(service == ForwardTypeAcademy && authorized), r.Header.Get("isadmin"))
					require.Equal(t, "operator", r.Header.Get("User-Name"))
					w.WriteHeader(204)
				}))
				defer target.Close()
				rp, err := proxy.NewReverseProxy(target.URL)
				require.NoError(t, err)
				users := mockcomponent.NewMockUserComponent(t)
				rpc := mockrpc.NewMockUserSvcClient(t)
				users.EXPECT().GetUserByName(mock.Anything, "operator").Return(&database.User{Username: "operator", UUID: "verified-uuid"}, nil)
				rpc.EXPECT().GetOrCreateFirstAvaiTokens(mock.Anything, "operator", "operator", string(types.AccessTokenAppGit), "dataflow").Return("user-token", nil)
				h := &DataflowProxyHandler{rp: rp, user: users, usc: rpc, forwardType: service}
				r := gin.New()
				r.Use(func(c *gin.Context) {
					httpbase.SetCurrentUser(c, "operator")
					if authorized {
						c.Request = c.Request.WithContext(types.WithAdminAuthorization(c.Request.Context()))
					}
				})
				r.GET("/api/v1/academy/*any", h.Proxy)
				server := httptest.NewServer(r)
				defer server.Close()
				req, err := http.NewRequest("GET", server.URL+"/api/v1/academy/test", nil)
				require.NoError(t, err)
				req.Header.Set("isadmin", "true")
				resp, err := server.Client().Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				require.Equal(t, 204, resp.StatusCode)
			})
		}
	}
}
