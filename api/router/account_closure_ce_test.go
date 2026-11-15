//go:build !ee && !saas

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserRoutes_LoginUser_CloseAccount_PassesThrough(t *testing.T) {
	router := newUserRoutesTestRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/user/testuser/close_account", nil)
	req.Header.Set("X-Test-User", "testuser")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code,
		"logged-in user DELETE close_account should reach the proxy (ownership validated downstream)")
}
