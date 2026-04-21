package handler

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AccessTokenTester struct {
	*testutil.GinTester
	handler *AccessTokenHandler
	mocks   struct {
		token *mockcomp.MockAccessTokenComponent
	}
}

func NewAccessTokenTester(t *testing.T) *AccessTokenTester {
	tester := &AccessTokenTester{GinTester: testutil.NewGinTester()}
	tester.mocks.token = mockcomp.NewMockAccessTokenComponent(t)
	tester.handler = &AccessTokenHandler{
		c: tester.mocks.token,
	}
	return tester
}

func (t *AccessTokenTester) WithHandleFunc(fn func(h *AccessTokenHandler) gin.HandlerFunc) *AccessTokenTester {
	t.Handler(fn(t.handler))
	return t
}

func (t *AccessTokenTester) WithUserUUID() *AccessTokenTester {
	t.Gctx().Set(httpbase.CurrentUserUUIDCtxVar, "user-uuid")
	return t
}

func (t *AccessTokenTester) WithUserAndUUID() *AccessTokenTester {
	t.GinTester.WithUser()
	t.Gctx().Set(httpbase.CurrentUserUUIDCtxVar, "user-uuid")
	return t
}

func TestAccessTokenHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("token not found returns 404", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.Get
		})

		tester.mocks.token.EXPECT().Check(tester.Gctx(), &types.CheckAccessTokenReq{
			Token:       "invalid-token-value",
			Application: "",
		}).Return(types.CheckAccessTokenResp{}, errorx.ErrNotFound)

		tester.WithParam("token_value", "invalid-token-value").
			Execute()

		tester.ResponseEqCode(t, 404)
	})

	t.Run("token found returns 200", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.Get
		})

		expectedResp := types.CheckAccessTokenResp{
			Token:       "valid-token-value",
			TokenName:   "my-token",
			Application: "git",
			Permission:  "read",
			Username:    "testuser",
			UserUUID:    "user-uuid-123",
			ExpireAt:    time.Now().Add(time.Hour),
		}

		tester.mocks.token.EXPECT().Check(tester.Gctx(), &types.CheckAccessTokenReq{
			Token:       "valid-token-value",
			Application: "",
		}).Return(expectedResp, nil)

		tester.WithParam("token_value", "valid-token-value").
			Execute()

		tester.ResponseEq(t, 200, "OK", expectedResp)
	})

	t.Run("internal server error returns 500", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.Get
		})

		tester.mocks.token.EXPECT().Check(tester.Gctx(), &types.CheckAccessTokenReq{
			Token:       "some-token",
			Application: "",
		}).Return(types.CheckAccessTokenResp{}, errorx.NewCustomError("DB-ERR", 1, nil, nil))

		tester.WithParam("token_value", "some-token").
			Execute()

		tester.ResponseEqCode(t, 500)
	})

	t.Run("token found with app query param", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.Get
		})

		expectedResp := types.CheckAccessTokenResp{
			Token:       "starship-token-value",
			TokenName:   "starship-token",
			Application: "starship",
			Username:    "testuser",
			UserUUID:    "user-uuid-456",
		}

		tester.mocks.token.EXPECT().Check(tester.Gctx(), &types.CheckAccessTokenReq{
			Token:       "starship-token-value",
			Application: "starship",
		}).Return(expectedResp, nil)

		tester.WithParam("token_value", "starship-token-value").
			WithQuery("app", "starship").
			Execute()

		tester.ResponseEq(t, 200, "OK", expectedResp)
	})
}

func TestAccessTokenHandler_DeleteAppToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("token not found returns 404", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.DeleteAppToken
		})

		tester.mocks.token.EXPECT().Delete(tester.Gctx(), &types.DeleteUserTokenRequest{
			Username:    "u",
			TokenName:   "my-token",
			Application: "git",
		}).Return(errorx.ErrNotFound)

		tester.WithUser().
			WithParam("app", "git").
			WithParam("token_name", "my-token").
			Execute()

		tester.ResponseEqCode(t, 404)
	})

	t.Run("success returns 200", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.DeleteAppToken
		})

		tester.mocks.token.EXPECT().Delete(tester.Gctx(), &types.DeleteUserTokenRequest{
			Username:    "u",
			TokenName:   "my-token",
			Application: "git",
		}).Return(nil)

		tester.WithUser().
			WithParam("app", "git").
			WithParam("token_name", "my-token").
			Execute()

		tester.ResponseEqCode(t, 200)
	})
}

func TestAccessTokenHandler_Refresh(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("token not found returns 404", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.Refresh
		})

		tester.mocks.token.EXPECT().RefreshToken(tester.Gctx(), "u", "my-token", "git", time.Time{}).
			Return(types.CheckAccessTokenResp{}, errorx.ErrNotFound)

		tester.WithUser().
			WithParam("app", "git").
			WithParam("token_name", "my-token").
			Execute()

		tester.ResponseEqCode(t, 404)
	})

	t.Run("success returns 200", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.Refresh
		})

		expiredAt := time.Now().Add(time.Hour)
		expectedResp := types.CheckAccessTokenResp{
			Token:       "new-token-value",
			TokenName:   "my-token",
			Application: "git",
			Permission:  "read",
			Username:    "u",
			UserUUID:    "user-uuid",
			ExpireAt:    expiredAt,
		}

		tester.mocks.token.EXPECT().RefreshToken(tester.Gctx(), "u", "my-token", "git", time.Time{}).
			Return(expectedResp, nil)

		tester.WithUser().
			WithParam("app", "git").
			WithParam("token_name", "my-token").
			Execute()

		tester.ResponseEq(t, 200, "OK", expectedResp)
	})
}

func TestAccessTokenHandler_CreateAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing namespace uuid returns 400", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.CreateAPIKey
		})

		tester.WithUserAndUUID().
			Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("invalid request body returns 400", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.CreateAPIKey
		})

		tester.WithUserAndUUID().
			WithParam("uuid", "namespace-uuid").
			Execute()

		tester.ResponseEqCode(t, 400)
	})
}

func TestAccessTokenHandler_GetAPIKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing namespace uuid returns 400", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.GetAPIKeys
		})

		tester.WithUserAndUUID().
			Execute()

		tester.ResponseEqCode(t, 400)
	})
}

func TestAccessTokenHandler_UpdateAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid id returns 400", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.UpdateAPIKey
		})

		tester.WithUserAndUUID().
			WithParam("uuid", "namespace-uuid").
			WithParam("id", "invalid").
			Execute()

		tester.ResponseEqCode(t, 400)
	})
}

func TestAccessTokenHandler_DeleteAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid id returns 400", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.DeleteAPIKey
		})

		tester.WithUserAndUUID().
			WithParam("uuid", "namespace-uuid").
			WithParam("id", "invalid").
			Execute()

		tester.ResponseEqCode(t, 400)
	})
}
