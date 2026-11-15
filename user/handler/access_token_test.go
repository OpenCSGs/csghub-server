package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockapicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
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
		refreshReq := &types.RefreshTokenReq{
			Username:  "u",
			TokenName: "my-token",
			App:       "git",
		}
		tester.mocks.token.EXPECT().RefreshToken(tester.Gctx(), refreshReq).
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
		refreshReq := &types.RefreshTokenReq{
			Username:     "u",
			TokenName:    "my-token",
			App:          "git",
			NewExpiredAt: time.Time{},
		}
		tester.mocks.token.EXPECT().RefreshToken(tester.Gctx(), refreshReq).
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

	t.Run("multiple quotas are passed to component", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.CreateAPIKey
		})

		body := types.CreateAPIKeyRequest{
			KeyName: "my-key",
			Quotas: []types.UpdateAPIKeyQuotaItem{
				{QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee, Quota: 100},
				{QuotaType: types.AccountingQuotaTypeDaily, ValueType: types.AccountingQuotaValueTypeToken, Quota: 50},
			},
		}

		expectedReq := types.CreateUserTokenRequest{
			Username:    "u",
			OpUUID:      "user-uuid",
			NSUUID:      "namespace-uuid",
			TokenName:   "my-key",
			Application: types.AccessTokenAppAIGateway,
			Quotas:      body.Quotas,
		}

		tester.mocks.token.EXPECT().Create(tester.Gctx(), &expectedReq).
			Return(&database.AccessToken{ID: 1, Name: "my-key"}, nil).Once()

		tester.WithUserAndUUID().
			WithParam("uuid", "namespace-uuid").
			WithBody(t, body).
			Execute()

		tester.ResponseEqCode(t, 200)
	})

	t.Run("invalid quota type returns 400", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.CreateAPIKey
		})

		body := types.CreateAPIKeyRequest{
			KeyName: "my-key",
			Quotas: []types.UpdateAPIKeyQuotaItem{
				{QuotaType: types.AccountingQuotaType("bogus"), ValueType: types.AccountingQuotaValueTypeFee, Quota: 100},
			},
		}

		tester.WithUserAndUUID().
			WithParam("uuid", "namespace-uuid").
			WithBody(t, body).
			Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("negative quota returns 400", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.CreateAPIKey
		})

		body := types.CreateAPIKeyRequest{
			KeyName: "my-key",
			Quotas: []types.UpdateAPIKeyQuotaItem{
				{QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee, Quota: -1},
			},
		}

		tester.WithUserAndUUID().
			WithParam("uuid", "namespace-uuid").
			WithBody(t, body).
			Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("unenforceable quota combination returns 400", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.CreateAPIKey
		})

		// monthly+request passes the enum check but is never enforced by the
		// quota middleware or usage accounting, so it must be rejected.
		body := types.CreateAPIKeyRequest{
			KeyName: "my-key",
			Quotas: []types.UpdateAPIKeyQuotaItem{
				{QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeRequest, Quota: 1000},
			},
		}

		tester.WithUserAndUUID().
			WithParam("uuid", "namespace-uuid").
			WithBody(t, body).
			Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("per-minute fee combination returns 400", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.CreateAPIKey
		})

		body := types.CreateAPIKeyRequest{
			KeyName: "my-key",
			Quotas: []types.UpdateAPIKeyQuotaItem{
				{QuotaType: types.AccountingQuotaTypePerMinute, ValueType: types.AccountingQuotaValueTypeFee, Quota: 100},
			},
		}

		tester.WithUserAndUUID().
			WithParam("uuid", "namespace-uuid").
			WithBody(t, body).
			Execute()

		tester.ResponseEqCode(t, 400)
	})
}

func TestAccessTokenHandler_GetAPIKeyQuotas(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing token value returns 400", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.GetAPIKeyQuotas
		})

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("get quotas successfully returns 200", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.GetAPIKeyQuotas
		})

		quotas := []database.AccountAccessTokenQuota{
			{
				ID:          1,
				APIKey:      "test-api-key",
				QuotaType:   types.AccountingQuotaTypeMonthly,
				ValueType:   types.AccountingQuotaValueTypeFee,
				Usage:       50.0,
				Quota:       100.0,
				PeriodStart: 1704067200,
				PeriodEnd:   1735689600,
			},
		}

		tester.mocks.token.EXPECT().GetAPIKeyQuotas(tester.Gctx().Request.Context(), "test-api-key").
			Return(quotas, nil).Once()

		tester.WithParam("token_value", "test-api-key").
			Execute()

		tester.ResponseEq(t, 200, "OK", quotas)
	})

	t.Run("get empty quotas returns 200", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.GetAPIKeyQuotas
		})

		tester.mocks.token.EXPECT().GetAPIKeyQuotas(tester.Gctx().Request.Context(), "test-api-key").
			Return([]database.AccountAccessTokenQuota{}, nil).Once()

		tester.WithParam("token_value", "test-api-key").
			Execute()

		tester.ResponseEq(t, 200, "OK", []database.AccountAccessTokenQuota{})
	})

	t.Run("internal server error returns 500", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
			return h.GetAPIKeyQuotas
		})

		tester.mocks.token.EXPECT().GetAPIKeyQuotas(tester.Gctx().Request.Context(), "test-api-key").
			Return(nil, errorx.ErrInternalServerError).Once()

		tester.WithParam("token_value", "test-api-key").
			Execute()

		tester.ResponseEqCode(t, 500)
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

// TestAccessTokenHandlerListNeverDisclosesStoredValues checks the display boundary
// independently from runtime credential resolution and existing create/refresh tests.
func TestAccessTokenHandlerListNeverDisclosesStoredValues(t *testing.T) {
	tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc { return h.GetUserTokensDisplay })
	tester.mocks.token.EXPECT().GetTokens(tester.Gctx(), &types.GetAccessTokenRequest{Username: "u", Application: types.AccessTokenAppGit}).Return([]types.CheckAccessTokenResp{{Token: "sk-original-secret-yz", TokenName: "sample", Application: types.AccessTokenAppGit}}, nil)
	tester.WithUser().WithQuery("app", "git").Execute()
	tester.ResponseEq(t, 200, "OK", []types.CheckAccessTokenResp{{Token: "sk********yz", TokenName: "sample", Application: types.AccessTokenAppGit}})
	require.NotContains(t, tester.Response().Body.String(), "original-secret")
}

// TestAccessTokenRuntimeResolverKeepsOriginalBearer verifies that actual business
// execution retains the existing response independently of display-only reads.
func TestAccessTokenRuntimeResolverKeepsOriginalBearer(t *testing.T) {
	tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc { return h.GetOrCreateFirstAvaiTokens })
	tester.mocks.token.EXPECT().GetOrCreateFirstAvaiToken(tester.Gctx(), "u", "git", "runtime").Return("original-secret-value", nil)
	tester.WithUser().WithQuery("app", "git").WithQuery("token_name", "runtime").Execute()
	tester.ResponseEq(t, 200, "OK", "original-secret-value")
}

// TestAccessTokenListCompatibility checks that display projection does not change
// legacy plaintext reads, application filtering, or missing-token behavior.
func TestAccessTokenListCompatibility(t *testing.T) {
	for _, app := range []types.AccessTokenApp{types.AccessTokenAppGit, types.AccessTokenAppStarship, types.AccessTokenAppMirror} {
		for _, masked := range []bool{false, true} {
			t.Run(string(app)+"/masked="+fmt.Sprint(masked), func(t *testing.T) {
				tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
					if masked {
						return h.GetUserTokensDisplay
					}
					return h.GetUserTokens
				})
				tester.mocks.token.EXPECT().GetTokens(tester.Gctx(), &types.GetAccessTokenRequest{Username: "u", Application: app}).Return([]types.CheckAccessTokenResp{{Token: "original-secret-value", Application: app}}, nil)
				tester.WithUser().WithQuery("app", string(app)).Execute()
				value := "original-secret-value"
				if masked {
					value = "or********ue"
				}
				tester.ResponseEq(t, 200, "OK", []types.CheckAccessTokenResp{{Token: value, Application: app}})
			})
		}
	}
	t.Run("display requires authentication", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc { return h.GetUserTokensDisplay })
		tester.Execute()
		tester.ResponseEqCode(t, 401)
	})
	t.Run("missing credentials remain absent", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc { return h.GetUserTokensDisplay })
		tester.mocks.token.EXPECT().GetTokens(tester.Gctx(), &types.GetAccessTokenRequest{Username: "u", Application: types.AccessTokenAppGit}).Return([]types.CheckAccessTokenResp{}, nil)
		tester.WithUser().WithQuery("app", "git").Execute()
		tester.ResponseEq(t, 200, "OK", []types.CheckAccessTokenResp{})
	})
}

// TestAccessTokenBuiltinRuntimeDisplay verifies separate JSON responses for raw
// resolution and masked display, with no custom display response header.
func TestAccessTokenBuiltinRuntimeDisplay(t *testing.T) {
	for _, masked := range []bool{false, true} {
		for _, present := range []bool{false, true} {
			t.Run(fmt.Sprintf("masked=%t/present=%t", masked, present), func(t *testing.T) {
				tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc {
					if masked {
						return h.GetBuiltinKeysDisplay
					}
					return h.GetBuiltinKeys
				})
				req := &types.GetAccessTokenRequest{Username: "u", OpUUID: "user-uuid", NSUUID: "namespace-uuid", Application: types.AccessTokenAppAIGateway}
				var key *types.CheckAccessTokenResp
				if present {
					value := "original-gateway-key"
					if masked {
						value = "or********ey"
					}
					key = &types.CheckAccessTokenResp{ID: 1, Token: value, NSUUID: req.NSUUID}
				}
				if masked {
					tester.mocks.token.EXPECT().GetBuiltinAPIKeyDisplay(context.Background(), req).Return(key, nil).Once()
				} else {
					tester.mocks.token.EXPECT().GetOrCreateBuiltinAPIKey(context.Background(), req).Return(key, nil).Once()
				}
				tester.WithUserAndUUID().WithParam("uuid", req.NSUUID).Execute()
				tester.ResponseEq(t, 200, "OK", key)
				require.Empty(t, tester.Response().Header().Get("X-Credential-Display"))
			})
		}
	}
}

// TestAccessTokenBuiltinDisplayErrors preserves namespace validation and propagates
// authorization or storage errors without disclosing a credential.
func TestAccessTokenBuiltinDisplayErrors(t *testing.T) {
	t.Run("missing namespace", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc { return h.GetBuiltinKeysDisplay })
		tester.WithUserAndUUID().Execute()
		tester.ResponseEqCode(t, 400)
	})
	t.Run("component error", func(t *testing.T) {
		tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc { return h.GetBuiltinKeysDisplay })
		tester.mocks.token.EXPECT().GetBuiltinAPIKeyDisplay(context.Background(), &types.GetAccessTokenRequest{Username: "u", OpUUID: "user-uuid", NSUUID: "namespace-uuid", Application: types.AccessTokenAppAIGateway}).Return(nil, fmt.Errorf("lookup unavailable")).Once()
		tester.WithUserAndUUID().WithParam("uuid", "namespace-uuid").Execute()
		tester.ResponseEqCode(t, 500)
	})
}

// TestCreateAPIKeyLimitReached verifies that a wrapped quota error returns a stable client error code.
func TestCreateAPIKeyLimitReached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tester := NewAccessTokenTester(t).WithHandleFunc(func(h *AccessTokenHandler) gin.HandlerFunc { return h.CreateAPIKey })
	tester.mocks.token.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("create token: %w", errorx.ErrAPIKeyLimitReached)).Once()
	tester.WithUserAndUUID().WithParam("uuid", "namespace-uuid").WithBody(t, types.CreateAPIKeyRequest{
		KeyName: "new-key",
		Quotas:  []types.UpdateAPIKeyQuotaItem{{QuotaType: types.AccountingQuotaTypeUnlimited, ValueType: types.AccountingQuotaValueTypeFee}},
	}).Execute()
	tester.ResponseEqCode(t, 400)
	require.Contains(t, tester.Response().Body.String(), `"code":"USER-ERR-31"`)
}

// TestCreateLegacyAPIKeyLimitReached verifies the same client error on both legacy creation routes.
func TestCreateLegacyAPIKeyLimitReached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name    string
		handler func(*AccessTokenHandler) gin.HandlerFunc
	}{
		{"app token", func(h *AccessTokenHandler) gin.HandlerFunc { return h.CreateAppToken }},
		{"deprecated user token", func(h *AccessTokenHandler) gin.HandlerFunc { return h.Create }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tester := NewAccessTokenTester(t).WithHandleFunc(tc.handler)
			sensitive := mockapicomp.NewMockSensitiveComponent(t)
			sensitive.EXPECT().CheckRequestV2(mock.Anything, mock.Anything).Return(true, nil).Once()
			tester.handler.sc = sensitive
			tester.mocks.token.EXPECT().Create(mock.Anything, mock.MatchedBy(func(req *types.CreateUserTokenRequest) bool {
				return req.Username == "u" && req.Application == types.AccessTokenAppAIGateway && req.NSUUID == ""
			})).Return(nil, fmt.Errorf("create token: %w", errorx.ErrAPIKeyLimitReached)).Once()
			tester.WithUser().WithParam("app", "aigateway").WithParam("token_name", "legacy-key").WithParam("username", "u").
				WithBody(t, types.CreateUserTokenRequest{TokenName: "legacy-key", Application: types.AccessTokenAppAIGateway}).Execute()
			tester.ResponseEqCode(t, 400)
			require.Contains(t, tester.Response().Body.String(), `"code":"USER-ERR-31"`)
		})
	}
}
