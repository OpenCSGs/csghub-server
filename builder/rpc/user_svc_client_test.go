package rpc

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/common/types"
)

func setupTestClient(server *httptest.Server) *UserSvcHttpClient {
	return &UserSvcHttpClient{
		hc: &HttpClient{
			endpoint: server.URL,
			hc:       server.Client(),
			logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		},
	}
}

func TestGetMemberRole_Success(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/organization/test-org/members/test-user?current_user=test-user", r.URL.String())
		assert.Equal(t, http.MethodGet, r.Method)

		resp := httpbase.R{
			Data: "admin",
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	// 执行测试
	role, err := client.GetMemberRole(context.Background(), "test-org", "test-user")

	// 验证结果
	assert.NoError(t, err)
	assert.Equal(t, membership.RoleAdmin, role)
}

func TestGetMemberRole_Failure(t *testing.T) {
	// 创建模拟服务器返回错误
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestClient(server)

	// 执行测试
	role, err := client.GetMemberRole(context.Background(), "test-org", "test-user")

	// 验证结果
	assert.Error(t, err)
	assert.Equal(t, membership.RoleUnknown, role)
}

func TestGetNameSpaceInfo_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/namespace/test-namespace", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		resp := httpbase.R{
			Data: &Namespace{Path: "test-namespace"},
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	ns, err := client.GetNameSpaceInfo(context.Background(), "test-namespace")

	assert.NoError(t, err)
	assert.NotNil(t, ns)
	assert.Equal(t, "test-namespace", ns.Path)
}

func TestGetUserInfo_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/user/test-user?current_user=visitor", r.URL.String())
		assert.Equal(t, http.MethodGet, r.Method)

		resp := httpbase.R{
			Data: &User{Username: "test-user"},
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	user, err := client.GetUserInfo(context.Background(), "test-user", "visitor")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "test-user", user.Username)
}

func TestGetOrCreateFirstAvaiTokens_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/user/test-user/tokens/first?current_user=visitor&app=test-app&token_name=test-token", r.URL.String())
		assert.Equal(t, http.MethodGet, r.Method)

		resp := httpbase.R{
			Data: "test-token-value",
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	token, err := client.GetOrCreateFirstAvaiTokens(context.Background(), "test-user", "visitor", "test-app", "test-token")

	assert.NoError(t, err)
	assert.Equal(t, "test-token-value", token)
}

func TestVerifyByAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/token/test-token", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		resp := httpbase.R{
			Data: &types.CheckAccessTokenResp{UserUUID: "123", Username: "test-user"},
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	result, err := client.VerifyByAccessToken(context.Background(), "test-token")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "123", result.UserUUID)
	assert.Equal(t, "test-user", result.Username)
}

func TestGetUserByName_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/user/test-user", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		resp := httpbase.R{
			Data: &types.User{Username: "test-user", Email: "test@example.com"},
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	user, err := client.GetUserByName(context.Background(), "test-user")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "test-user", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
}

func TestGetUserByUUID_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/user/test-uuid", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "uuid", r.URL.Query().Get("type"))

		resp := httpbase.R{
			Data: &types.User{
				UUID:     "test-uuid",
				Username: "test-user",
				Email:    "test@example.com",
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	user, err := client.GetUserByUUID(context.Background(), "test-uuid")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "test-uuid", user.UUID)
	assert.Equal(t, "test-user", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
}

func TestGetUserByUUID_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestClient(server)

	user, err := client.GetUserByUUID(context.Background(), "test-uuid")

	assert.Error(t, err)
	assert.Nil(t, user)
}

func TestFindByUUIDs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/users/by-uuids", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "uuids=uuid1&uuids=uuid2", r.URL.RawQuery)

		resp := struct {
			Msg  string        `json:"msg"`
			Data []*types.User `json:"data"`
		}{
			Data: []*types.User{
				{UUID: "uuid1", Username: "user1"},
				{UUID: "uuid2", Username: "user2"},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	result, err := client.FindByUUIDs(context.Background(), []string{"uuid1", "uuid2"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Equal(t, "user1", result["uuid1"].Username)
	assert.Equal(t, "user2", result["uuid2"].Username)
}

func TestFindByUUIDs_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestClient(server)

	result, err := client.FindByUUIDs(context.Background(), []string{"uuid1", "uuid2"})

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestFindByUUIDs_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/users/by-uuids", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		resp := struct {
			Msg  string        `json:"msg"`
			Data []*types.User `json:"data"`
		}{
			Data: []*types.User{},
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	result, err := client.FindByUUIDs(context.Background(), []string{"uuid1"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestFindByUUIDs_FiltersInvalidUsers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/users/by-uuids", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		resp := struct {
			Msg  string        `json:"msg"`
			Data []*types.User `json:"data"`
		}{
			Data: []*types.User{
				{UUID: "uuid1", Username: "user1"},
				{UUID: "", Username: "user2"}, // Empty UUID should be filtered
				{UUID: "uuid3", Username: "user3"},
				nil, // Nil user should be filtered
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	result, err := client.FindByUUIDs(context.Background(), []string{"uuid1", "uuid2", "uuid3"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2) // Only uuid1 and uuid3 should be in the result
	assert.Equal(t, "user1", result["uuid1"].Username)
	assert.Equal(t, "user3", result["uuid3"].Username)
	assert.Nil(t, result["uuid2"]) // uuid2 should not be in result
}

func TestGetUserUUIDs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/user/user_uuids", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "10", r.URL.Query().Get("per"))
		assert.Equal(t, "1", r.URL.Query().Get("page"))

		resp := struct {
			Data struct {
				UserUUIDs []string `json:"data"`
				Total     int      `json:"total"`
			} `json:"data"`
		}{
			Data: struct {
				UserUUIDs []string `json:"data"`
				Total     int      `json:"total"`
			}{
				UserUUIDs: []string{"uuid1", "uuid2", "uuid3"},
				Total:     100,
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	uuids, total, err := client.GetUserUUIDs(context.Background(), 10, 1)

	assert.NoError(t, err)
	assert.Len(t, uuids, 3)
	assert.Equal(t, 100, total)
	assert.Contains(t, uuids, "uuid1")
}

func TestGetEmails_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/internal/user/emails", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "20", r.URL.Query().Get("per"))
		assert.Equal(t, "1", r.URL.Query().Get("page"))

		resp := struct {
			Msg   string   `json:"msg"`
			Data  []string `json:"data"`
			Total int      `json:"total"`
		}{
			Data:  []string{"test1@example.com", "test2@example.com"},
			Total: 50,
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	emails, total, err := client.GetEmails(context.Background(), 20, 1)

	assert.NoError(t, err)
	assert.Len(t, emails, 2)
	assert.Equal(t, 50, total)
	assert.Contains(t, emails, "test1@example.com")
}

func TestGetMemberRole_DataConversionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := httpbase.R{
			Data: 123, // 返回非字符串类型，应该导致类型转换错误
		}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestClient(server)

	role, err := client.GetMemberRole(context.Background(), "test-org", "test-user")

	assert.Error(t, err)
	assert.Equal(t, membership.RoleUnknown, role)
}
