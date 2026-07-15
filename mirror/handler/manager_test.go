package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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
