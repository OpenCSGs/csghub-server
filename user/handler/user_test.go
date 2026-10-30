package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockapicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	usercomponent "opencsg.com/csghub-server/user/component"
)

// TestUserHandler_RejectsLastOrganizationAdmin verifies deletion and account closure return lastOrgAdmin.
func TestUserHandler_RejectsLastOrganizationAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expectedErr := errorx.LastOrgAdmin(
		errors.New("cannot delete the last administrator of an organization"),
		errorx.Ctx().Set("username", "u"),
	)
	expectedCustomErr := expectedErr.(errorx.CustomError)
	expectedResponse := httpbase.R{
		Code:    expectedCustomErr.Code(),
		Msg:     expectedCustomErr.Error(),
		Context: expectedCustomErr.Context(),
	}

	t.Run("delete user", func(t *testing.T) {
		tester := testutil.NewGinTester()
		mockUserComponent := component.NewMockUserComponent(t)
		handler := &UserHandler{c: mockUserComponent}
		tester.Handler(handler.Delete)
		mockUserComponent.EXPECT().CheckOperatorAndUser(tester.Ctx(), "u", "u").Return(false, nil)
		mockUserComponent.EXPECT().CheckIfUserIsLastOrgAdmin(tester.Ctx(), "u").Return(true, nil)

		tester.WithUser().WithParam("username", "u").Execute()

		tester.ResponseEqSimple(t, http.StatusConflict, expectedResponse)
	})

	t.Run("close account", func(t *testing.T) {
		tester := testutil.NewGinTester()
		mockUserComponent := component.NewMockUserComponent(t)
		handler := &UserHandler{c: mockUserComponent}
		tester.Handler(handler.CloseAccount)
		mockUserComponent.EXPECT().CheckIfUserIsLastOrgAdmin(tester.Gctx(), "u").Return(true, nil)

		tester.WithUser().WithParam("username", "u").Execute()

		tester.ResponseEqSimple(t, http.StatusConflict, expectedResponse)
	})
}

// TestUserHandler_RejectsAdministratorDeletion stops before scheduling a workflow for an administrator target.
func TestUserHandler_RejectsAdministratorDeletion(t *testing.T) {
	tester := testutil.NewGinTester()
	userComponent := component.NewMockUserComponent(t)
	tester.Handler((&UserHandler{c: userComponent}).Delete)
	userComponent.EXPECT().CheckOperatorAndUser(tester.Ctx(), "u", "target").Return(false, errorx.ErrAdminUserCannotBeDeleted).Once()
	tester.WithUser().WithParam("username", "target").Execute()
	custom := errorx.ErrAdminUserCannotBeDeleted.(errorx.CustomError)
	tester.ResponseEqSimple(t, http.StatusBadRequest, httpbase.R{Code: custom.Code(), Msg: custom.Error()})
}

func TestUserHandler_ResetUserTags_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userUUID := "test-user-uuid"
	tagIDs := []int64{1, 2, 3}
	mockUserComponent := component.NewMockUserComponent(t)
	mockUserComponent.EXPECT().ResetUserTags(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	handler := UserHandler{
		c: mockUserComponent,
	}
	body, _ := json.Marshal(tagIDs)
	req, err := http.NewRequest("POST", "/user/tags", strings.NewReader(string(body)))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	ctx.Set(httpbase.CurrentUserUUIDCtxVar, userUUID)
	handler.ResetUserTags(ctx)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserHandler_ResetUserTags_Failure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userUUID := "test-user-uuid"
	tagIDs := []int64{1, 2, 3}

	mockUserComponent := component.NewMockUserComponent(t)
	mockUserComponent.EXPECT().ResetUserTags(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("failed to reset user tags"))

	handler := UserHandler{
		c: mockUserComponent,
	}

	body, err := json.Marshal(tagIDs)
	assert.NoError(t, err)
	req, err := http.NewRequest("POST", "/user/tags", strings.NewReader(string(body)))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	ctx.Set(httpbase.CurrentUserUUIDCtxVar, userUUID)

	handler.ResetUserTags(ctx)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUserHandler_ResetUserTags_UserNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userUUID := "non-existent-user"
	tagIDs := []int64{1, 2, 3}

	mockUserComponent := component.NewMockUserComponent(t)
	mockUserComponent.EXPECT().ResetUserTags(mock.Anything, mock.Anything, mock.Anything).Return(errorx.ErrUserNotFound)

	handler := UserHandler{
		c: mockUserComponent,
	}

	body, _ := json.Marshal(tagIDs)
	req, err := http.NewRequest("POST", "/user/tags", strings.NewReader(string(body)))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	ctx.Set(httpbase.CurrentUserUUIDCtxVar, userUUID)

	handler.ResetUserTags(ctx)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUserHandler_GetAdminUserUUIDs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		mockUserComponent := component.NewMockUserComponent(t)
		mockUserComponent.EXPECT().
			GetAdminUserUUIDs(mock.Anything).
			Return([]string{"admin-uuid", "super-user-uuid"}, nil)
		handler := UserHandler{c: mockUserComponent}
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/user/admin_uuids", nil)

		handler.GetAdminUserUUIDs(ctx)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{
			"msg": "OK",
			"data": {
				"data": ["admin-uuid", "super-user-uuid"],
				"total": 2
			}
		}`, recorder.Body.String())
	})

	t.Run("component error", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		mockUserComponent := component.NewMockUserComponent(t)
		mockUserComponent.EXPECT().
			GetAdminUserUUIDs(mock.Anything).
			Return(nil, errors.New("component error"))
		handler := UserHandler{c: mockUserComponent}
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/user/admin_uuids", nil)

		handler.GetAdminUserUUIDs(ctx)

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	})
}

func TestUserHandler_GetAdminEmails(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		mockUserComponent := component.NewMockUserComponent(t)
		mockUserComponent.EXPECT().
			GetAdminEmails(mock.Anything).
			Return([]string{"admin@example.com", "super-user@example.com"}, nil)
		handler := UserHandler{c: mockUserComponent}
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/user/admin_emails", nil)

		handler.GetAdminEmails(ctx)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{
			"msg": "OK",
			"data": {
				"data": ["admin@example.com", "super-user@example.com"],
				"total": 2
			}
		}`, recorder.Body.String())
	})

	t.Run("component error", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		mockUserComponent := component.NewMockUserComponent(t)
		mockUserComponent.EXPECT().
			GetAdminEmails(mock.Anything).
			Return(nil, errors.New("component error"))
		handler := UserHandler{c: mockUserComponent}
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/user/admin_emails", nil)

		handler.GetAdminEmails(ctx)

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	})
}

func TestUserHandler_Casdoor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
		for _, secret := range []string{"signed_token", "starship_token", "csghub_token"} {
			assert.NotContains(t, logs.String(), secret)
		}
		assert.Contains(t, logs.String(), "generate login redirect url")
	})
	const (
		mockSigninSuccessRedirectURL       = "http://localhost:8080/signin/success"
		mockSigninFailureRedirectURL       = "http://localhost:8080/signin/failure"
		mockCodeSoulerVScodeRedirectURL    = "vscode://open"
		mockCodeSoulerJetbrainsRedirectURL = "jetbrains://open"
		mockLiteRedirectURL                = "http://127.0.0.1:11437/api/cloud/auth/callback"
	)

	cfg := &config.Config{
		APIServer: struct {
			Port         int    `env:"STARHUB_SERVER_SERVER_PORT" default:"8080"`
			PublicDomain string `env:"STARHUB_SERVER_PUBLIC_DOMAIN" default:"http://localhost:8080"`
			SSHDomain    string `env:"STARHUB_SERVER_SSH_DOMAIN" default:"ssh://git@localhost:2222"`
		}{
			PublicDomain: "http://localhost:8080",
		},
		User: struct {
			Host                           string `env:"OPENCSG_USER_SERVER_HOST" default:"http://localhost"`
			Port                           int    `env:"OPENCSG_USER_SERVER_PORT" default:"8088"`
			SigninSuccessRedirectURL       string `env:"OPENCSG_USER_SERVER_SIGNIN_SUCCESS_REDIRECT_URL" default:"http://localhost:3000/server/callback"`
			CodeSoulerVScodeRedirectURL    string `env:"OPENCSG_USER_SERVER_CODESOULER_VSCODE_REDIRECT_URL" default:"http://127.0.0.1:37678/callback"`
			CodeSoulerJetBrainsRedirectURL string `env:"OPENCSG_USER_SERVER_CODESOULER_JETBRAINS_REDIRECT_URL" default:"http://127.0.0.1:37679/callback"`
			LiteRedirectURL                string `env:"OPENCSG_USER_SERVER_LITE_REDIRECT_URL" default:"http://127.0.0.1:11437/api/cloud/auth/callback"`
			AwardSelfRegisterCredit        int    `env:"OPENCSG_USER_SERVER_AWARD_SELF_REGISTER_CREDIT" default:"5000"`
			NamespaceLookupMaxUUIDs        int    `env:"OPENCSG_USER_SERVER_NAMESPACE_LOOKUP_MAX_UUIDS" default:"100"`
		}{
			SigninSuccessRedirectURL:       mockSigninSuccessRedirectURL,
			CodeSoulerVScodeRedirectURL:    mockCodeSoulerVScodeRedirectURL,
			CodeSoulerJetBrainsRedirectURL: mockCodeSoulerJetbrainsRedirectURL,
			LiteRedirectURL:                mockLiteRedirectURL,
			AwardSelfRegisterCredit:        5000,
			NamespaceLookupMaxUUIDs:        100,
		},
		ServerFailureRedirectURL: mockSigninFailureRedirectURL,
	}

	t.Run("success signin with casdoor state", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/casdoor?code=123&state=casdoor", nil)

		mockUserComp := component.NewMockUserComponent(t)
		mockUserComp.EXPECT().Signin(mock.Anything, "123", CASDOOR).Return(&types.JWTClaims{CurrentUser: "testuser"}, "signed_token", nil)

		h := &UserHandler{
			c:                        mockUserComp,
			signinSuccessRedirectURL: mockSigninSuccessRedirectURL,
			signinFailureRedirectURL: mockSigninFailureRedirectURL,
			config:                   cfg,
		}

		h.Casdoor(c)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "http://localhost:8080/signin/success?jwt=signed_token", w.Header().Get("Location"))
		// Verify Content-Length is either empty or set to 0
		contentLength := w.Header().Get("Content-Length")
		assert.True(t, contentLength == "" || contentLength == "0", "Content-Length should be either empty or 0, got %s", contentLength)
		mockUserComp.AssertExpectations(t)
	})

	t.Run("success signin with vscode state", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/casdoor?code=123&state=vscode", nil)

		mockUserComp := component.NewMockUserComponent(t)
		mockUserComp.On("Signin", mock.Anything, "123", "vscode").Return(&types.JWTClaims{CurrentUser: "testuser"}, "signed_token", nil)
		mockAccessTokenComp := new(component.MockAccessTokenComponent)
		mockAccessTokenComp.On("GetOrCreateFirstAvaiToken", mock.Anything, "testuser", string(types.AccessTokenAppStarship), "codesouler-vscode").Return("starship_token", nil)

		h := &UserHandler{
			c:                              mockUserComp,
			atc:                            mockAccessTokenComp,
			signinSuccessRedirectURL:       mockSigninSuccessRedirectURL,
			signinFailureRedirectURL:       mockSigninFailureRedirectURL,
			codeSoulerVScodeRedirectURL:    mockCodeSoulerVScodeRedirectURL,
			codeSoulerJetbrainsRedirectURL: mockCodeSoulerJetbrainsRedirectURL,
			config:                         cfg,
		}

		h.Casdoor(c)

		assert.Equal(t, http.StatusFound, w.Code)
		expectedURL := fmt.Sprintf("%s?apikey=%s&portal_url=%s&jwt=%s", mockCodeSoulerVScodeRedirectURL, "starship_token", mockSigninSuccessRedirectURL, "signed_token")
		assert.Equal(t, expectedURL, w.Header().Get("Location"))
		// Verify Content-Length is either empty or set to 0
		contentLength := w.Header().Get("Content-Length")
		assert.True(t, contentLength == "" || contentLength == "0", "Content-Length should be either empty or 0, got %s", contentLength)
		mockUserComp.AssertExpectations(t)
		mockAccessTokenComp.AssertExpectations(t)
	})

	t.Run("success signin with jetbrains state", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/casdoor?code=123&state=jetbrains", nil)

		mockUserComp := component.NewMockUserComponent(t)
		mockUserComp.On("Signin", mock.Anything, "123", "jetbrains").Return(&types.JWTClaims{CurrentUser: "testuser"}, "signed_token", nil)
		mockAccessTokenComp := new(component.MockAccessTokenComponent)
		mockAccessTokenComp.On("GetOrCreateFirstAvaiToken", mock.Anything, "testuser", string(types.AccessTokenAppStarship), "codesouler-jetbrains").Return("starship_token", nil)

		h := &UserHandler{
			c:                              mockUserComp,
			atc:                            mockAccessTokenComp,
			signinSuccessRedirectURL:       mockSigninSuccessRedirectURL,
			signinFailureRedirectURL:       mockSigninFailureRedirectURL,
			codeSoulerVScodeRedirectURL:    mockCodeSoulerVScodeRedirectURL,
			codeSoulerJetbrainsRedirectURL: mockCodeSoulerJetbrainsRedirectURL,
			config:                         cfg,
		}

		h.Casdoor(c)

		assert.Equal(t, http.StatusFound, w.Code)
		expectedURL := fmt.Sprintf("%s?apikey=%s&portal_url=%s&jwt=%s", mockCodeSoulerJetbrainsRedirectURL, "starship_token", mockSigninSuccessRedirectURL, "signed_token")
		assert.Equal(t, expectedURL, w.Header().Get("Location"))
		// Verify Content-Length is either empty or set to 0
		contentLength := w.Header().Get("Content-Length")
		assert.True(t, contentLength == "" || contentLength == "0", "Content-Length should be either empty or 0, got %s", contentLength)
		mockUserComp.AssertExpectations(t)
		mockAccessTokenComp.AssertExpectations(t)
	})

	t.Run("success signin with lite state", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/casdoor?code=123&state=lite", nil)

		mockUserComp := component.NewMockUserComponent(t)
		mockUserComp.On("Signin", mock.Anything, "123", LITE).Return(&types.JWTClaims{CurrentUser: "testuser"}, "signed_token", nil)
		mockAccessTokenComp := new(component.MockAccessTokenComponent)
		mockAccessTokenComp.On("GetOrCreateFirstAvaiToken", mock.Anything, "testuser", string(types.AccessTokenAppCSGHub), "csglite").Return("csghub_token", nil)

		h := &UserHandler{
			c:                              mockUserComp,
			atc:                            mockAccessTokenComp,
			signinSuccessRedirectURL:       mockSigninSuccessRedirectURL,
			signinFailureRedirectURL:       mockSigninFailureRedirectURL,
			codeSoulerVScodeRedirectURL:    mockCodeSoulerVScodeRedirectURL,
			codeSoulerJetbrainsRedirectURL: mockCodeSoulerJetbrainsRedirectURL,
			liteRedirectURL:                mockLiteRedirectURL,
			config:                         cfg,
		}

		h.Casdoor(c)

		assert.Equal(t, http.StatusFound, w.Code)
		expectedURL := fmt.Sprintf("%s?token=%s&portal_url=%s&jwt=%s", mockLiteRedirectURL, "csghub_token", mockSigninSuccessRedirectURL, "signed_token")
		assert.Equal(t, expectedURL, w.Header().Get("Location"))
		contentLength := w.Header().Get("Content-Length")
		assert.True(t, contentLength == "" || contentLength == "0", "Content-Length should be either empty or 0, got %s", contentLength)
		mockUserComp.AssertExpectations(t)
		mockAccessTokenComp.AssertExpectations(t)
	})

	t.Run("success signin with flows state", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		state := "http://langflow.com/api/v1/callback/opencsg?url=http://langflow.com/flows"
		c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/casdoor?code=123&state=%s", state), nil)

		mockUserComp := component.NewMockUserComponent(t)
		mockUserComp.On("Signin", mock.Anything, "123", state).Return(&types.JWTClaims{CurrentUser: "testuser"}, "signed_token", nil)

		h := &UserHandler{
			c:                        mockUserComp,
			signinSuccessRedirectURL: mockSigninSuccessRedirectURL,
			signinFailureRedirectURL: mockSigninFailureRedirectURL,
			config:                   cfg,
		}

		h.Casdoor(c)

		assert.Equal(t, http.StatusFound, w.Code)
		expectedURL := fmt.Sprintf("http://langflow.com/api/v1/callback/opencsg?jwt_token=signed_token&url=%s", url.QueryEscape("http://langflow.com/flows"))
		assert.Equal(t, expectedURL, w.Header().Get("Location"))
		// Verify Content-Length is either empty or set to 0
		contentLength := w.Header().Get("Content-Length")
		assert.True(t, contentLength == "" || contentLength == "0", "Content-Length should be either empty or 0, got %s", contentLength)
		mockUserComp.AssertExpectations(t)
	})

	t.Run("signin failure", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/casdoor?code=123&state=", nil)

		mockUserComp := component.NewMockUserComponent(t)
		mockUserComp.On("Signin", mock.Anything, "123", "").Return(nil, "", errors.New("signin error"))

		h := &UserHandler{
			c:                        mockUserComp,
			signinSuccessRedirectURL: mockSigninSuccessRedirectURL,
			signinFailureRedirectURL: mockSigninFailureRedirectURL,
			config:                   cfg,
		}

		h.Casdoor(c)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Contains(t, w.Header().Get("Location"), mockSigninFailureRedirectURL)
		assert.Contains(t, w.Header().Get("Location"), "error_code=500")
		// Verify Content-Length is either empty or set to 0
		contentLength := w.Header().Get("Content-Length")
		assert.True(t, contentLength == "" || contentLength == "0", "Content-Length should be either empty or 0, got %s", contentLength)
		mockUserComp.AssertExpectations(t)
	})

	t.Run("invalid flows state", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/casdoor?code=123&state=flows%2Finvalid", nil)

		mockUserComp := component.NewMockUserComponent(t)
		mockUserComp.EXPECT().Signin(mock.Anything, "123", "flows/invalid").Return(&types.JWTClaims{CurrentUser: "testuser"}, "signed_token", nil)

		h := &UserHandler{
			c:                        mockUserComp,
			signinSuccessRedirectURL: mockSigninSuccessRedirectURL,
			signinFailureRedirectURL: mockSigninFailureRedirectURL,
			config:                   cfg,
		}

		h.Casdoor(c)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Contains(t, w.Header().Get("Location"), mockSigninFailureRedirectURL)
		assert.Contains(t, w.Header().Get("Location"), "error_code=500")
		// Verify Content-Length is either empty or set to 0
		contentLength := w.Header().Get("Content-Length")
		assert.True(t, contentLength == "" || contentLength == "0", "Content-Length should be either empty or 0, got %s", contentLength)
		mockUserComp.AssertExpectations(t)
	})
}

func TestUserHandler_CasdoorFailureDoesNotLogOAuthCredentials(t *testing.T) {
	var logs bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/casdoor?code=code-secret&state=state-secret", nil)
	comp := component.NewMockUserComponent(t)
	comp.EXPECT().Signin(mock.Anything, "code-secret", "state-secret").Return(nil, "", errors.New("signin failed"))
	h := &UserHandler{c: comp, signinFailureRedirectURL: "https://example.com/failure", config: &config.Config{}}
	h.Casdoor(c)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.NotContains(t, logs.String(), "code-secret")
	assert.NotContains(t, logs.String(), "state-secret")
	assert.Contains(t, logs.String(), "code_fingerprint")
}

func TestHandleConflictCustomError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		customErr      errorx.CustomError
		redirectURL    string
		expectedStatus int
		expectedURL    string
		expectRedirect bool
	}{
		{
			name:           "username conflict - successful redirect",
			customErr:      errorx.UsernameExists("testuser").(errorx.CustomError),
			redirectURL:    "https://example.com/error",
			expectedStatus: http.StatusFound,
			expectedURL:    "https://example.com/error?error_code=409&error_message_code=USER-ERR-12&field_name=username&field_value=testuser",
			expectRedirect: true,
		},
		{
			name:           "email conflict - successful redirect",
			customErr:      errorx.EmailExists("test@example.com").(errorx.CustomError),
			redirectURL:    "https://example.com/error",
			expectedStatus: http.StatusFound,
			expectedURL:    "https://example.com/error?error_code=409&error_message_code=USER-ERR-13&field_name=email&field_value=test%40example.com",
			expectRedirect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)

			result := handleConflictCustomError(ctx, tt.customErr, tt.redirectURL)

			assert.True(t, result, "handleConflictCustomError should return true for valid conflicts")
			assert.Equal(t, tt.expectedStatus, w.Code, "HTTP status code should be 301 Moved Permanently")

			assert.Equal(t, tt.expectedURL, w.Header().Get("Location"), "Redirect URL should match expected URL")
		})
	}
}

func TestHandleConflictCustomError_InvalidErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		customErr    errorx.CustomError
		redirectURL  string
		expectReturn bool
	}{
		{
			name:         "non-conflict error - should return false",
			customErr:    errorx.NewCustomError("USER-ERR", 1, nil, nil), // Some other error
			redirectURL:  "https://example.com/error",
			expectReturn: false,
		},
		{
			name:         "username conflict but no username in context",
			customErr:    errorx.NewCustomError("USER-ERR", 18, nil, map[string]interface{}{}), // UsernameExists but no username
			redirectURL:  "https://example.com/error",
			expectReturn: false,
		},
		{
			name:         "email conflict but no email in context",
			customErr:    errorx.NewCustomError("USER-ERR", 19, nil, map[string]interface{}{}), // EmailExists but no email
			redirectURL:  "https://example.com/error",
			expectReturn: false,
		},
		{
			name:         "username conflict but username is not string",
			customErr:    errorx.NewCustomError("USER-ERR", 18, nil, map[string]interface{}{"username": 123}), // UsernameExists but username is int
			redirectURL:  "https://example.com/error",
			expectReturn: false,
		},
		{
			name:         "email conflict but email is not string",
			customErr:    errorx.NewCustomError("USER-ERR", 19, nil, map[string]interface{}{"email": 123}), // EmailExists but email is int
			redirectURL:  "https://example.com/error",
			expectReturn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)

			result := handleConflictCustomError(ctx, tt.customErr, tt.redirectURL)

			assert.Equal(t, tt.expectReturn, result, "handleConflictCustomError should return expected result")

		})
	}
}

func TestExtractConflictInfo(t *testing.T) {
	tests := []struct {
		name          string
		customErr     errorx.CustomError
		expectedField string
		expectedValue string
		expectedOk    bool
	}{
		{
			name:          "username conflict with valid username",
			customErr:     errorx.UsernameExists("testuser").(errorx.CustomError),
			expectedField: "username",
			expectedValue: "testuser",
			expectedOk:    true,
		},
		{
			name:          "email conflict with valid email",
			customErr:     errorx.EmailExists("test@example.com").(errorx.CustomError),
			expectedField: "email",
			expectedValue: "test@example.com",
			expectedOk:    true,
		},
		{
			name:          "username conflict with special characters",
			customErr:     errorx.UsernameExists("user@domain.com").(errorx.CustomError),
			expectedField: "username",
			expectedValue: "user@domain.com",
			expectedOk:    true,
		},
		{
			name:          "email conflict with special characters",
			customErr:     errorx.EmailExists("user+tag@domain.com").(errorx.CustomError),
			expectedField: "email",
			expectedValue: "user+tag@domain.com",
			expectedOk:    true,
		},
		{
			name:          "non-conflict error",
			customErr:     errorx.NewCustomError("USER-ERR", 1, nil, nil),
			expectedField: "",
			expectedValue: "",
			expectedOk:    false,
		},
		{
			name:          "username conflict but no username in context",
			customErr:     errorx.NewCustomError("USER-ERR", 18, nil, map[string]interface{}{}),
			expectedField: "",
			expectedValue: "",
			expectedOk:    false,
		},
		{
			name:          "email conflict but no email in context",
			customErr:     errorx.NewCustomError("USER-ERR", 19, nil, map[string]interface{}{}),
			expectedField: "",
			expectedValue: "",
			expectedOk:    false,
		},
		{
			name:          "username conflict but username is not string",
			customErr:     errorx.NewCustomError("USER-ERR", 18, nil, map[string]interface{}{"username": 123}),
			expectedField: "",
			expectedValue: "",
			expectedOk:    false,
		},
		{
			name:          "email conflict but email is not string",
			customErr:     errorx.NewCustomError("USER-ERR", 19, nil, map[string]interface{}{"email": 123}),
			expectedField: "",
			expectedValue: "",
			expectedOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, value, ok := extractConflictInfo(tt.customErr)

			assert.Equal(t, tt.expectedField, field, "Field should match expected value")
			assert.Equal(t, tt.expectedValue, value, "Value should match expected value")
			assert.Equal(t, tt.expectedOk, ok, "Ok should match expected value")
		})
	}
}

func TestHandleExportUserInfo(t *testing.T) {
	r := gin.Default()
	mockUserComp := component.NewMockUserComponent(t)
	h := &UserHandler{
		c: mockUserComp,
	}

	ch := make(chan types.UserIndexResp)
	go func() {
		defer close(ch)
		ch <- types.UserIndexResp{
			Users: []*types.User{
				{
					Username: "testuser",
					Email:    "test@example.com",
				},
			},
		}
	}()
	mockUserComp.EXPECT().StreamExportUsers(mock.Anything, mock.Anything).Return(ch, nil)
	r.GET("/users/stream-export", h.ExportUserInfo)

	req, _ := http.NewRequest("GET",
		"/users/stream-export?verify_status=approved&search=test_search&labels=vip&labels=basic",
		nil,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Status code should be 200 OK")
}

func TestRedirectWithoutBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		code           int
		url            string
		expectedStatus int
		expectedHeader string
	}{
		{
			name:           "redirect with 301 status",
			code:           http.StatusMovedPermanently,
			url:            "https://example.com/new-location",
			expectedStatus: http.StatusMovedPermanently,
			expectedHeader: "https://example.com/new-location",
		},
		{
			name:           "redirect with 302 status",
			code:           http.StatusFound,
			url:            "https://example.com/temporary-redirect",
			expectedStatus: http.StatusFound,
			expectedHeader: "https://example.com/temporary-redirect",
		},
		{
			name:           "redirect with 307 status",
			code:           http.StatusTemporaryRedirect,
			url:            "https://example.com/temp-redirect",
			expectedStatus: http.StatusTemporaryRedirect,
			expectedHeader: "https://example.com/temp-redirect",
		},
		{
			name:           "redirect with 308 status",
			code:           http.StatusPermanentRedirect,
			url:            "https://example.com/perm-redirect",
			expectedStatus: http.StatusPermanentRedirect,
			expectedHeader: "https://example.com/perm-redirect",
		},
		{
			name:           "redirect with query parameters",
			code:           http.StatusFound,
			url:            "https://example.com/callback?code=123&state=abc",
			expectedStatus: http.StatusFound,
			expectedHeader: "https://example.com/callback?code=123&state=abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)

			redirectWithoutBody(ctx, tt.code, tt.url)

			// Verify the status code
			assert.Equal(t, tt.expectedStatus, w.Code, "HTTP status code should match")

			// Verify the Location header
			assert.Equal(t, tt.expectedHeader, w.Header().Get("Location"), "Location header should match")

			// Verify Content-Length is either empty or set to 0
			contentLength := w.Header().Get("Content-Length")
			assert.True(t, contentLength == "" || contentLength == "0", "Content-Length should be either empty or 0, got %s", contentLength)

			// Verify that there is no response body
			bodyBytes, err := io.ReadAll(w.Body)
			assert.NoError(t, err, "Should be able to read response body")
			assert.Empty(t, bodyBytes, "Response body should be empty")
			assert.Equal(t, 0, len(bodyBytes), "Response body length should be 0")

			// Verify that the response was aborted (no further processing)
			assert.True(t, ctx.IsAborted(), "Context should be aborted")
		})
	}
}

// TestUserHandler_RequestAdminAuthorization verifies user handlers forward request grants without relying on Gin context fallback.
func TestUserHandler_RequestAdminAuthorization(t *testing.T) {
	for _, authorized := range []bool{false, true} {
		for _, operation := range []string{"get", "internal get", "index", "update"} {
			t.Run(fmt.Sprintf("%s/authorized=%t", operation, authorized), func(t *testing.T) {
				requestContext := context.Background()
				if authorized {
					requestContext = types.WithAdminAuthorization(requestContext)
				}
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = httptest.NewRequest(http.MethodPut, "/user/target?page=2&per=20", strings.NewReader(`{"name":"Updated"}`)).WithContext(requestContext)
				ctx.Request.Header.Set("Content-Type", "application/json")
				ctx.Params = gin.Params{{Key: "username", Value: "target"}, {Key: "id", Value: "target-uuid"}}
				httpbase.SetCurrentUser(ctx, "operator")
				if operation == "internal get" {
					httpbase.SetAuthType(ctx, httpbase.AuthTypeSystemApiKey)
				}
				requestContext = ctx.Request.Context()
				users := component.NewMockUserComponent(t)
				h := &UserHandler{c: users}
				switch operation {
				case "get":
					users.EXPECT().Get(requestContext, "target", "operator", false).Return(&types.User{Username: "target"}, nil).Once()
					h.Get(ctx)
				case "internal get":
					users.EXPECT().GetInternal(requestContext, "target", false).Return(&types.User{Username: "target"}, nil).Once()
					h.Get(ctx)
				case "index":
					users.EXPECT().Index(requestContext, mock.MatchedBy(func(req types.UserListReq) bool {
						return req.VisitorName == "operator" && req.Page == 2 && req.Per == 20
					})).Return(nil, 0, nil).Once()
					h.Index(ctx)
				case "update":
					sensitive := mockapicomp.NewMockSensitiveComponent(t)
					sensitive.EXPECT().CheckRequestV2(ctx, mock.Anything).Return(true, nil).Once()
					h.sc = sensitive
					users.EXPECT().UpdateByUUID(requestContext, mock.MatchedBy(func(req *types.UpdateUserRequest) bool {
						return req.UUID != nil && *req.UUID == "target-uuid" && req.OpUser == "operator"
					})).Return(nil).Once()
					h.Update(ctx)
				}
				require.Equal(t, http.StatusOK, ctx.Writer.Status())
			})
		}
	}
}

// userVerificationContextMock embeds the component's private method and mocks only the tested HTTP operations.
type userVerificationContextMock struct {
	usercomponent.UserVerifyComponent
	mock.Mock
}

// Get records the request context and target UUID supplied by the verification handler.
func (m *userVerificationContextMock) Get(ctx context.Context, uuid string) (*database.UserVerify, error) {
	args := m.Called(ctx, uuid)
	return args.Get(0).(*database.UserVerify), args.Error(1)
}

// Update records the request context and unchanged verification update arguments.
func (m *userVerificationContextMock) Update(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*database.UserVerify, error) {
	args := m.Called(ctx, id, status, reason)
	return args.Get(0).(*database.UserVerify), args.Error(1)
}

// assertVerificationRequestContext checks authorization and cancellation without enabling Gin context fallback.
func assertVerificationRequestContext(t *testing.T, actual, expected context.Context, authorized bool) {
	t.Helper()
	require.Same(t, expected, actual)
	require.Equal(t, authorized, types.IsAdminAuthorized(actual))
	expectedDeadline, expectedHasDeadline := expected.Deadline()
	deadline, hasDeadline := actual.Deadline()
	require.True(t, expectedHasDeadline)
	require.Equal(t, expectedHasDeadline, hasDeadline)
	require.Equal(t, expectedDeadline, deadline)
	require.Equal(t, expected.Done(), actual.Done())
	require.Equal(t, expected.Err(), actual.Err())
}

// TestUserHandler_VerificationRequestContext covers admin and ordinary verification reads and updates.
func TestUserHandler_VerificationRequestContext(t *testing.T) {
	for _, authorized := range []bool{false, true} {
		for _, canceled := range []bool{false, true} {
			for _, operation := range []string{"get", "update"} {
				t.Run(fmt.Sprintf("%s/admin=%t/canceled=%t", operation, authorized, canceled), func(t *testing.T) {
					requestContext, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
					t.Cleanup(cancel)
					if authorized {
						requestContext = types.WithAdminAuthorization(requestContext)
					}
					if canceled {
						cancel()
					}
					ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
					ctx.Request = httptest.NewRequest(http.MethodPut, "/user/verify/42", strings.NewReader(`{"status":"approved"}`)).WithContext(requestContext)
					ctx.Request.Header.Set("Content-Type", "application/json")
					verification := &database.UserVerify{ID: 42, UUID: "target-user", Status: types.VerifyStatusApproved}
					component := &userVerificationContextMock{}
					component.Test(t)
					t.Cleanup(func() { component.AssertExpectations(t) })
					checkContext := func(args mock.Arguments) {
						assertVerificationRequestContext(t, args.Get(0).(context.Context), requestContext, authorized)
					}
					h := &UserHandler{uv: component}
					if operation == "get" {
						ctx.Params = gin.Params{{Key: "id", Value: "target-user"}}
						component.On("Get", requestContext, "target-user").Run(checkContext).Return(verification, nil).Once()
						h.GetVerify(ctx)
					} else {
						ctx.Params = gin.Params{{Key: "id", Value: "42"}}
						component.On("Update", requestContext, int64(42), types.VerifyStatusApproved, "").Run(checkContext).Return(verification, nil).Once()
						h.UpdateVerify(ctx)
					}
					require.Equal(t, http.StatusOK, ctx.Writer.Status())
				})
			}
		}
	}
}

// TestJWTHandlerRefreshAdmin binds refresh to the authenticated JWT and preserves the component's expiry.
func TestJWTHandlerRefreshAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expiry := time.Now().UTC().Add(15 * time.Minute).Truncate(time.Second)
	for _, tc := range []struct {
		name, uuid, authorization, body string
		authType                        httpbase.AuthType
		componentErr                    error
		want                            int
		call                            bool
	}{
		{name: "current administrator", uuid: "self", authorization: "Bearer old-jwt", body: `{"uuid":"self","old_token":"old-jwt"}`, authType: httpbase.AuthTypeJwt, want: http.StatusOK, call: true},
		{name: "missing identity", authorization: "Bearer old-jwt", body: `{"uuid":"self","old_token":"old-jwt"}`, authType: httpbase.AuthTypeJwt, want: http.StatusUnauthorized},
		{name: "API key cannot refresh", uuid: "self", authorization: "Bearer old-jwt", body: `{"uuid":"self","old_token":"old-jwt"}`, authType: httpbase.AuthTypeSystemApiKey, want: http.StatusUnauthorized},
		{name: "PAT cannot refresh", uuid: "self", authorization: "Bearer old-jwt", body: `{"uuid":"self","old_token":"old-jwt"}`, authType: httpbase.AuthTypeAccessToken, want: http.StatusUnauthorized},
		{name: "missing bearer", uuid: "self", body: `{"uuid":"self","old_token":"old-jwt"}`, authType: httpbase.AuthTypeJwt, want: http.StatusUnauthorized},
		{name: "other user", uuid: "self", authorization: "Bearer old-jwt", body: `{"uuid":"other","old_token":"old-jwt"}`, authType: httpbase.AuthTypeJwt, want: http.StatusForbidden},
		{name: "other token", uuid: "self", authorization: "Bearer old-jwt", body: `{"uuid":"self","old_token":"other-jwt"}`, authType: httpbase.AuthTypeJwt, want: http.StatusForbidden},
		{name: "missing old token", uuid: "self", authorization: "Bearer old-jwt", body: `{"uuid":"self"}`, authType: httpbase.AuthTypeJwt, want: http.StatusBadRequest},
		{name: "invalid body", uuid: "self", authorization: "Bearer old-jwt", body: `{`, authType: httpbase.AuthTypeJwt, want: http.StatusBadRequest},
		{name: "expired token", uuid: "self", authorization: "Bearer old-jwt", body: `{"uuid":"self","old_token":"old-jwt"}`, authType: httpbase.AuthTypeJwt, componentErr: errorx.ErrInvalidJWT, want: http.StatusUnauthorized, call: true},
		{name: "component unavailable", uuid: "self", authorization: "Bearer old-jwt", body: `{"uuid":"self","old_token":"old-jwt"}`, authType: httpbase.AuthTypeJwt, componentErr: errors.New("unavailable"), want: http.StatusInternalServerError, call: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := component.NewMockJwtComponent(t)
			if tc.call {
				c.EXPECT().RefreshToken(mock.Anything, types.RefreshJWTReq{UUID: "self", OldToken: "old-jwt"}).Run(func(ctx context.Context, _ types.RefreshJWTReq) {
					require.False(t, types.IsAdminAuthorized(ctx))
					require.Equal(t, "self", ctx.Value(httpbase.CurrentUserUUIDCtxVar))
				}).Return(&types.JWTClaims{RegisteredClaims: jwtlib.RegisteredClaims{ExpiresAt: jwtlib.NewNumericDate(expiry)}}, "new-jwt", tc.componentErr)
			}
			engine := gin.New()
			engine.Use(func(ctx *gin.Context) {
				httpbase.SetCurrentUserUUID(ctx, tc.uuid)
				httpbase.SetAuthType(ctx, tc.authType)
			})
			engine.POST("/api/v1/admin/common/jwt/token", (&JWTHandler{c: c}).RefreshAdmin)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/common/jwt/token", strings.NewReader(tc.body))
			request.Header.Set("Authorization", tc.authorization)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, tc.want, response.Code, response.Body.String())
			if tc.want == http.StatusOK {
				var result struct {
					Data types.CreateJWTResp `json:"data"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
				require.Equal(t, "new-jwt", result.Data.Token)
				require.Equal(t, expiry, result.Data.ExpireAt)
			}
		})
	}
}

// TestUserHandler_UserChoicesResponse restricts a single-submodule administrator to selector fields.
// The permission gate is simulated here; downstream route tests independently cover module isolation.
func TestUserHandler_UserChoicesResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, permission := range []types.AdminPermission{
		types.AdminAssetModel, types.AdminAssetDataset, types.AdminComputeNode,
		types.AdminAcademyUserPermission, types.AdminAccountOrganization, types.AdminAccountingComputeVoucher,
	} {
		t.Run(string(permission), func(t *testing.T) {
			userComponent := component.NewMockUserComponent(t)
			h := &UserHandler{c: userComponent}
			userComponent.EXPECT().UserChoices(mock.MatchedBy(func(ctx context.Context) bool {
				return types.IsAdminAuthorized(ctx)
			}), types.UserListReq{VisitorName: "module-admin", Search: "alice", Page: 2, Per: 10, ExactMatch: true}).
				Return([]types.UserChoice{{ID: 42, UUID: "alice-uuid", Username: "alice", Nickname: "Alice", Avatar: "avatar.png"}}, 11, nil).Once()
			engine := gin.New()
			engine.GET("/users", func(ctx *gin.Context) {
				httpbase.SetCurrentUser(ctx, "module-admin")
				if ctx.GetHeader("X-Test-Permission") != string(permission) {
					ctx.AbortWithStatus(http.StatusForbidden)
					return
				}
				ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
				ctx.Next()
			}, h.UserChoices)
			request := httptest.NewRequest(http.MethodGet, "/users?page=2&per=10&search=alice&exact_match=true", nil)
			request.Header.Set("X-Test-Permission", string(permission))
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code)
			require.JSONEq(t, `{"msg":"OK","data":{"data":[{"id":42,"uuid":"alice-uuid","username":"alice","nickname":"Alice","avatar":"avatar.png"}],"total":11}}`, response.Body.String())
		})
	}
}

// TestUserHandler_UserListValidation keeps ordinary and selector validation consistent before component calls.
func TestUserHandler_UserListValidation(t *testing.T) {
	for _, query := range []struct{ key, value string }{
		{"exact_match", "invalid"}, {"page", "invalid"}, {"per", "invalid"},
	} {
		for _, choices := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/choices=%t", query.key, choices), func(t *testing.T) {
				userComponent := component.NewMockUserComponent(t)
				h := &UserHandler{c: userComponent}
				tester := testutil.NewGinTester()
				if choices {
					tester.Handler(h.UserChoices)
				} else {
					tester.Handler(h.Index)
				}
				tester.WithQuery(query.key, query.value).Execute()
				tester.ResponseEqCode(t, http.StatusBadRequest)
			})
		}
	}
}

// TestUserHandler_AccountUserListResponse preserves full profile fields at the authorized account list entry.
func TestUserHandler_AccountUserListResponse(t *testing.T) {
	tester := testutil.NewGinTester()
	userComponent := component.NewMockUserComponent(t)
	ctx := types.WithAdminAuthorization(tester.Ctx())
	tester.Gctx().Request = tester.Gctx().Request.WithContext(ctx)
	users := []*types.User{{ID: 42, Username: "alice", Email: "private@example.com", Phone: "123456", Roles: []string{"personal_user"}}}
	userComponent.EXPECT().Index(ctx, types.UserListReq{VisitorName: "u", Per: 10, Page: 1}).Return(users, 1, nil).Once()
	tester.Handler((&UserHandler{c: userComponent}).Index)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEq(t, http.StatusOK, "OK", gin.H{"data": users, "total": 1})
}

// TestUserHandler_UserChoiceResponse proves a voucher administrator receives no account details for another user.
func TestUserHandler_UserChoiceResponse(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "another user", status: http.StatusOK}, {name: "missing user", err: errorx.ErrDatabaseNoRows, status: http.StatusNotFound},
		{name: "store failure", err: errors.New("store unavailable"), status: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userComponent := component.NewMockUserComponent(t)
			userComponent.EXPECT().UserChoice(mock.MatchedBy(func(ctx context.Context) bool { return types.IsAdminAuthorized(ctx) }), "alice").
				Return(&types.UserChoiceDetail{UserChoice: types.UserChoice{ID: 42, UUID: "user-uuid", Username: "alice", Nickname: "Alice", Avatar: "avatar.png"},
					Namespaces: []types.NamespaceChoice{{Path: "alice", Type: "user", UUID: "personal-ns"}}}, tc.err).Once()
			engine := gin.New()
			engine.GET("/admin/accounting/compute_voucher/user/:username", func(ctx *gin.Context) {
				httpbase.SetCurrentUser(ctx, "voucher-only-admin")
				httpbase.SetAuthType(ctx, httpbase.AuthTypeSystemApiKey)
				ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
				ctx.Next()
			}, (&UserHandler{c: userComponent}).UserChoice)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/accounting/compute_voucher/user/alice?type=uuid&full=true", nil))
			require.Equal(t, tc.status, response.Code)
			if tc.status == http.StatusOK {
				require.JSONEq(t, `{"msg":"OK","data":{"id":42,"uuid":"user-uuid","username":"alice","nickname":"Alice","avatar":"avatar.png","namespaces":[{"Path":"alice","Type":"user","UUID":"personal-ns"}]}}`, response.Body.String())
			}
		})
	}
}
