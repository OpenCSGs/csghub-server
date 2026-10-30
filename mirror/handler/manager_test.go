package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/component"
)

func TestManagerHandlerDeprecatedWorkerEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &ManagerHandler{}

	tests := []struct {
		name string
		run  func(*gin.Context)
	}{
		{name: "stop worker by id", run: handler.StopWorkerByID},
		{name: "sync now", run: handler.SyncNow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			tt.run(c)

			require.Equal(t, http.StatusGone, w.Code)
		})
	}
}

// cancelTestManager isolates the cancellation contract without implementing unrelated manager methods.
type cancelTestManager struct {
	component.ManagerComponent
	cancel func(context.Context, int64) (bool, error)
}

// Cancel delegates the test's single manager operation.
func (m *cancelTestManager) Cancel(ctx context.Context, id int64) (bool, error) {
	return m.cancel(ctx, id)
}

// TestManagerHandlerCancelAdminContext preserves task validation, authorization, audit identity and cancellation errors.
func TestManagerHandlerCancelAdminContext(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		found      bool
		err        error
		status     int
	}{
		{name: "valid task", body: `{"task_id":123}`, found: true, status: http.StatusOK},
		{name: "missing task ID", body: `{}`, status: http.StatusBadRequest},
		{name: "task not found", body: `{"task_id":123}`, err: errors.New("task missing"), status: http.StatusNotFound},
		{name: "cancellation failed", body: `{"task_id":123}`, found: true, err: errors.New("cancel failed"), status: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requestContext := types.WithAdminAuthorization(context.Background())
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/lfs_sync/cancel", strings.NewReader(tc.body)).WithContext(requestContext)
			ctx.Request.Header.Set("Content-Type", "application/json")
			httpbase.SetCurrentUser(ctx, "operator")
			httpbase.SetCurrentUserUUID(ctx, "operator-uuid")
			httpbase.SetIPAddress(ctx, "192.0.2.1")
			httpbase.SetAccessToken(ctx, "test-token")
			httpbase.SetAuthType(ctx, httpbase.AuthTypeJwt)
			requestContext = ctx.Request.Context()
			called := false
			h := &ManagerHandler{managerComponent: &cancelTestManager{cancel: func(got context.Context, id int64) (bool, error) {
				called = true
				require.Equal(t, requestContext, got)
				require.True(t, types.IsAdminAuthorized(got))
				require.Equal(t, "operator", got.Value("currentUser"))
				require.Equal(t, "operator-uuid", got.Value("currentUserUUID"))
				require.Equal(t, "192.0.2.1", got.Value("ip_address"))
				require.Equal(t, "test-token", got.Value("accessToken"))
				require.Equal(t, "JWT", got.Value("authType"))
				require.Equal(t, int64(123), id)
				return tc.found, tc.err
			}}}
			h.Cancel(ctx)
			require.Equal(t, tc.status, ctx.Writer.Status())
			require.Equal(t, tc.status != http.StatusBadRequest, called)
		})
	}
}
