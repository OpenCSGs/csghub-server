package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/trace"
)

type stubReverseProxy struct {
	lastRequest *http.Request
}

func (s *stubReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, api, svcHost string) {
	s.lastRequest = r
	w.WriteHeader(http.StatusOK)
}

func TestCSGBotProxyHandler_Proxy_AddsCSGHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/api/v1/csgbot/chat", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set(trace.HeaderRequestID, "req-123")
	httpbase.SetCurrentUser(c, "test-user")

	mockUserComponent := mockcomponent.NewMockUserComponent(t)
	mockUserComponent.EXPECT().
		GetUserByName(mock.Anything, "test-user").
		Return(&database.User{
			ID:       42,
			UUID:     "user-uuid",
			Username: "test-user",
		}, nil)

	mockUserSvc := mockrpc.NewMockUserSvcClient(t)
	mockUserSvc.EXPECT().
		GetOrCreateFirstAvaiTokens(mock.Anything, "test-user", "test-user", string(types.AccessTokenAppGit), "csgbot").
		Return("test-token", nil)

	rp := &stubReverseProxy{}
	handler := &CSGBotProxyHandler{
		rp:   rp,
		user: mockUserComponent,
		usc:  mockUserSvc,
	}

	handler.Proxy(c)

	assert.NotNil(t, rp.lastRequest)
	assert.Equal(t, "user-uuid", rp.lastRequest.Header.Get("X-CSG-User-UUID"))
	assert.Equal(t, "test-user", rp.lastRequest.Header.Get("X-CSG-User-Name"))
	assert.Equal(t, "test-token", rp.lastRequest.Header.Get("X-CSG-User-Token"))
	assert.Equal(t, "req-123", rp.lastRequest.Header.Get("X-CSG-Request-Id"))
}
