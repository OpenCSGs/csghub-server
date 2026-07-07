package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
)

func TestBindResourceApplicationIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("overwrites client identity", func(t *testing.T) {
		router := gin.New()
		router.Use(func(ctx *gin.Context) {
			httpbase.SetCurrentUser(ctx, "authenticated-user")
			httpbase.SetCurrentUserUUID(ctx, "authenticated-uuid")
		})
		router.POST("/notifications", BindResourceApplicationIdentity(), func(ctx *gin.Context) {
			body, err := io.ReadAll(ctx.Request.Body)
			require.NoError(t, err)

			var messageReq types.MessageRequest
			require.NoError(t, json.Unmarshal(body, &messageReq))
			var resourceReq types.ResourceApplicationNotificationReq
			require.NoError(t, json.Unmarshal([]byte(messageReq.Parameters), &resourceReq))
			require.Equal(t, "authenticated-uuid", resourceReq.UserUUID)
			require.Equal(t, "authenticated-user", resourceReq.UserName)
			require.Equal(t, "A100", resourceReq.ResourceSKU)
			ctx.Status(http.StatusOK)
		})

		body := marshalMessageRequest(t, types.MessageScenarioResourceApplication,
			`{"user_uuid":"spoofed-uuid","user_name":"spoofed-user","resource_sku":"A100"}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/notifications", bytes.NewReader(body))

		router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("rejects missing authenticated user UUID", func(t *testing.T) {
		router := gin.New()
		router.POST("/notifications", BindResourceApplicationIdentity(), func(ctx *gin.Context) {
			ctx.Status(http.StatusOK)
		})

		body := marshalMessageRequest(t, types.MessageScenarioResourceApplication,
			`{"user_uuid":"spoofed-uuid","resource_sku":"A100"}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/notifications", bytes.NewReader(body))

		router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("preserves other scenarios", func(t *testing.T) {
		router := gin.New()
		router.POST("/notifications", BindResourceApplicationIdentity(), func(ctx *gin.Context) {
			body, err := io.ReadAll(ctx.Request.Body)
			require.NoError(t, err)
			var messageReq types.MessageRequest
			require.NoError(t, json.Unmarshal(body, &messageReq))
			require.JSONEq(t, `{"status":"success"}`, messageReq.Parameters)
			ctx.Status(http.StatusOK)
		})

		body := marshalMessageRequest(t, types.MessageScenario("test-scenario"), `{"status":"success"}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/notifications", bytes.NewReader(body))

		router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
	})
}

func marshalMessageRequest(t *testing.T, scenario types.MessageScenario, parameters string) []byte {
	t.Helper()
	body, err := json.Marshal(types.MessageRequest{
		Scenario:   scenario,
		Parameters: parameters,
		Priority:   types.MessagePriorityNormal,
	})
	require.NoError(t, err)
	return body
}
