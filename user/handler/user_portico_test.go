package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

func TestUserHandler_PorticoSignin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("redirect to sso authorization url", func(t *testing.T) {
		tester := testutil.NewGinTester()
		mockUserComponent := component.NewMockUserComponent(t)
		handler := &UserHandler{c: mockUserComponent}
		tester.Handler(handler.PorticoSignin)
		authURL := "http://portico.example.com/authorize?client_id=x&state=ab"
		mockUserComponent.EXPECT().BuildSigninRedirect(mock.Anything, "vscode").Return(authURL, "ab", nil)

		tester.WithQuery("state", "vscode").Execute()

		require.Equal(t, http.StatusFound, tester.Response().Code)
		assert.Equal(t, authURL, tester.Response().Header().Get("Location"))

		// the sign-in state must be bound to the initiating browser via a
		// http-only, same-site cookie carrying the __Host- prefix
		cookies := tester.Response().Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, porticoSigninCookieName, cookies[0].Name)
		assert.Equal(t, "ab", cookies[0].Value)
		assert.True(t, cookies[0].HttpOnly)
		assert.True(t, cookies[0].Secure)
		assert.Equal(t, "/", cookies[0].Path)
		assert.Empty(t, cookies[0].Domain)
		assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
	})

	t.Run("redirect to error page on failure", func(t *testing.T) {
		tester := testutil.NewGinTester()
		mockUserComponent := component.NewMockUserComponent(t)
		failureURL := "http://hub.example.com/signin/failure"
		handler := &UserHandler{c: mockUserComponent, signinFailureRedirectURL: failureURL}
		tester.Handler(handler.PorticoSignin)
		mockUserComponent.EXPECT().BuildSigninRedirect(mock.Anything, "casdoor").Return("", "", errors.New("redis down"))

		tester.Execute()

		require.Equal(t, http.StatusFound, tester.Response().Code)
		assert.Equal(t, fmt.Sprintf("%s?error_code=500&error_message=%s", failureURL, url.QueryEscape("failed to signin: redis down")), tester.Response().Header().Get("Location"))
	})
}

func TestUserHandler_Portico(t *testing.T) {
	gin.SetMode(gin.TestMode)

	nonce := strings.Repeat("ab", 16)

	t.Run("redirect to custom flow url", func(t *testing.T) {
		tester := testutil.NewGinTester()
		mockUserComponent := component.NewMockUserComponent(t)
		flowCallback, err := url.Parse("http://flow.example.com/cb")
		require.NoError(t, err)
		handler := &UserHandler{
			c:             mockUserComponent,
			flowRedirects: map[string]*url.URL{"flow": flowCallback},
		}
		tester.Handler(handler.Portico)
		state := nonce + "http://flow.example.com/cb"
		mockUserComponent.EXPECT().Signin(mock.Anything, "auth-code", state).Return(&types.JWTClaims{CurrentUser: "alice"}, "signed-jwt", nil)

		tester.WithQuery("code", "auth-code").WithQuery("state", state).
			WithHeader("Cookie", porticoSigninCookieName+"="+state).Execute()

		require.Equal(t, http.StatusFound, tester.Response().Code)
		assert.Equal(t, "http://flow.example.com/cb?jwt_token=signed-jwt", tester.Response().Header().Get("Location"))
	})

	t.Run("redirect to portal with jwt", func(t *testing.T) {
		tester := testutil.NewGinTester()
		mockUserComponent := component.NewMockUserComponent(t)
		successURL := "http://hub.example.com/signin/success"
		handler := &UserHandler{c: mockUserComponent, signinSuccessRedirectURL: successURL}
		tester.Handler(handler.Portico)
		state := nonce + "casdoor"
		mockUserComponent.EXPECT().Signin(mock.Anything, "auth-code", state).Return(&types.JWTClaims{CurrentUser: "alice"}, "signed-jwt", nil)

		tester.WithQuery("code", "auth-code").WithQuery("state", state).
			WithHeader("Cookie", porticoSigninCookieName+"="+state).Execute()

		require.Equal(t, http.StatusFound, tester.Response().Code)
		assert.Equal(t, fmt.Sprintf("%s?jwt=signed-jwt", successURL), tester.Response().Header().Get("Location"))
	})

	t.Run("invalid state redirects to error page", func(t *testing.T) {
		tester := testutil.NewGinTester()
		mockUserComponent := component.NewMockUserComponent(t)
		failureURL := "http://hub.example.com/signin/failure"
		handler := &UserHandler{c: mockUserComponent, signinFailureRedirectURL: failureURL}
		tester.Handler(handler.Portico)

		tester.WithQuery("code", "auth-code").WithQuery("state", "casdoor").Execute()

		require.Equal(t, http.StatusFound, tester.Response().Code)
		assert.Equal(t, fmt.Sprintf("%s?error_code=500&error_message=%s", failureURL, url.QueryEscape("failed to signin: invalid portico signin state: casdoor")), tester.Response().Header().Get("Location"))
	})

	t.Run("missing browser binding cookie redirects to error page", func(t *testing.T) {
		tester := testutil.NewGinTester()
		mockUserComponent := component.NewMockUserComponent(t)
		failureURL := "http://hub.example.com/signin/failure"
		handler := &UserHandler{c: mockUserComponent, signinFailureRedirectURL: failureURL}
		tester.Handler(handler.Portico)
		state := nonce + "casdoor"

		tester.WithQuery("code", "auth-code").WithQuery("state", state).Execute()

		require.Equal(t, http.StatusFound, tester.Response().Code)
		assert.Equal(t, fmt.Sprintf("%s?error_code=500&error_message=%s", failureURL, url.QueryEscape("failed to signin: portico sign-in state is not bound to this browser")), tester.Response().Header().Get("Location"))
	})

	t.Run("mismatched browser binding cookie redirects to error page", func(t *testing.T) {
		tester := testutil.NewGinTester()
		mockUserComponent := component.NewMockUserComponent(t)
		failureURL := "http://hub.example.com/signin/failure"
		handler := &UserHandler{c: mockUserComponent, signinFailureRedirectURL: failureURL}
		tester.Handler(handler.Portico)
		state := nonce + "casdoor"

		tester.WithQuery("code", "auth-code").WithQuery("state", state).
			WithHeader("Cookie", porticoSigninCookieName+"="+nonce+"http://attacker.example.com/cb").Execute()

		require.Equal(t, http.StatusFound, tester.Response().Code)
		assert.Equal(t, fmt.Sprintf("%s?error_code=500&error_message=%s", failureURL, url.QueryEscape("failed to signin: portico sign-in state is not bound to this browser")), tester.Response().Header().Get("Location"))
	})

	t.Run("forged same-name domain cookie does not satisfy binding", func(t *testing.T) {
		tester := testutil.NewGinTester()
		mockUserComponent := component.NewMockUserComponent(t)
		failureURL := "http://hub.example.com/signin/failure"
		handler := &UserHandler{c: mockUserComponent, signinFailureRedirectURL: failureURL}
		tester.Handler(handler.Portico)
		state := nonce + "casdoor"

		// A same-site subdomain can set a cookie with Domain=<apex> and the
		// plain, non-prefixed name; the victim's browser then sends that cookie
		// to the callback host. It must not satisfy the __Host- binding, whose
		// cookie name can only be set by the callback host itself over HTTPS.
		tester.WithQuery("code", "auth-code").WithQuery("state", state).
			WithHeader("Cookie", "portico_signin_state="+state).Execute()

		require.Equal(t, http.StatusFound, tester.Response().Code)
		assert.Equal(t, fmt.Sprintf("%s?error_code=500&error_message=%s", failureURL, url.QueryEscape("failed to signin: portico sign-in state is not bound to this browser")), tester.Response().Header().Get("Location"))
	})
}

func TestUserHandler_Casdoor_StillUsesFullStateAsRoutingState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tester := testutil.NewGinTester()
	mockUserComponent := component.NewMockUserComponent(t)
	successURL := "http://hub.example.com/signin/success"
	handler := &UserHandler{c: mockUserComponent, signinSuccessRedirectURL: successURL}
	tester.Handler(handler.Casdoor)
	mockUserComponent.EXPECT().Signin(mock.Anything, "auth-code", "casdoor").Return(&types.JWTClaims{CurrentUser: "alice"}, "signed-jwt", nil)

	tester.WithQuery("code", "auth-code").WithQuery("state", "casdoor").Execute()

	require.Equal(t, http.StatusFound, tester.Response().Code)
	assert.Equal(t, fmt.Sprintf("%s?jwt=signed-jwt", successURL), tester.Response().Header().Get("Location"))
}
