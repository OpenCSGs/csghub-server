package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

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

func TestUserHandler_Casdoor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const (
		mockSigninSuccessRedirectURL       = "http://localhost:8080/signin/success"
		mockSigninFailureRedirectURL       = "http://localhost:8080/signin/failure"
		mockCodeSoulerVScodeRedirectURL    = "vscode://open"
		mockCodeSoulerJetbrainsRedirectURL = "jetbrains://open"
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
		}{
			SigninSuccessRedirectURL:       mockSigninSuccessRedirectURL,
			CodeSoulerVScodeRedirectURL:    mockCodeSoulerVScodeRedirectURL,
			CodeSoulerJetBrainsRedirectURL: mockCodeSoulerJetbrainsRedirectURL,
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
