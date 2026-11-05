package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
)

type fakeUserPriorityResolver struct {
	priority   int
	restricted bool
	err        error
}

func (f *fakeUserPriorityResolver) UserPriority(_ context.Context, userUUID string) (int, bool, error) {
	if f.err != nil {
		return 0, false, f.err
	}
	if userUUID == "" {
		return 0, true, nil
	}
	return f.priority, f.restricted, nil
}

func TestUserTagsMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// runWithAuth builds a gin engine whose first handler emulates the
	// authenticator's context values, then the middleware under test.
	runWithAuth := func(t *testing.T, authType httpbase.AuthType, userUUID string, resolver UserPriorityResolver) *int {
		t.Helper()
		r := gin.New()
		r.Use(func(c *gin.Context) {
			httpbase.SetAuthType(c, authType)
			if userUUID != "" {
				httpbase.SetCurrentUserUUID(c, userUUID)
			}
			c.Next()
		})
		r.Use(NewUserTagsMiddleware(resolver))
		var got *int
		r.GET("/probe", func(c *gin.Context) {
			got = httpbase.GetCurrentUserPriority(c)
			c.Status(http.StatusOK)
		})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))
		require.Equal(t, http.StatusOK, w.Code)
		return got
	}

	t.Run("restricted user gets the resolved priority", func(t *testing.T) {
		got := runWithAuth(t, httpbase.AuthTypeUserOrgApiKey, "user-uuid-1",
			&fakeUserPriorityResolver{priority: 0, restricted: true})
		require.NotNil(t, got)
		require.Equal(t, 0, *got)
	})

	t.Run("intermediate tier is preserved", func(t *testing.T) {
		got := runWithAuth(t, httpbase.AuthTypeJwt, "user-uuid-1",
			&fakeUserPriorityResolver{priority: 50, restricted: true})
		require.NotNil(t, got)
		require.Equal(t, 50, *got)
	})

	t.Run("user without routing tags stays unrestricted", func(t *testing.T) {
		got := runWithAuth(t, httpbase.AuthTypeUserOrgApiKey, "user-uuid-1",
			&fakeUserPriorityResolver{restricted: false})
		require.Nil(t, got)
	})

	t.Run("system api key is not resolved", func(t *testing.T) {
		got := runWithAuth(t, httpbase.AuthTypeSystemApiKey, "platform",
			&fakeUserPriorityResolver{priority: 0, restricted: true})
		require.Nil(t, got)
	})

	t.Run("anonymous caller is not resolved", func(t *testing.T) {
		got := runWithAuth(t, "", "",
			&fakeUserPriorityResolver{priority: 0, restricted: true})
		require.Nil(t, got)
	})

	t.Run("resolver error leaves the caller unrestricted and does not fail the request", func(t *testing.T) {
		got := runWithAuth(t, httpbase.AuthTypeJwt, "user-uuid-1",
			&fakeUserPriorityResolver{err: errors.New("db down")})
		require.Nil(t, got)
	})
}

// Compile-time guard: the DB resolver satisfies the middleware's contract.
var _ UserPriorityResolver = (*dbUserPriorityResolver)(nil)
