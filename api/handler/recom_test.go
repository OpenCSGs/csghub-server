package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// TestRecomHandler_SetOpWeightPinsRepositoryType keeps the server-selected type independent of client input.
func TestRecomHandler_SetOpWeightPinsRepositoryType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type requestKey struct{}
	for _, repoType := range []types.RepositoryType{
		types.ModelRepo, types.DatasetRepo, types.CodeRepo, types.SpaceRepo, types.MCPServerRepo, types.SkillRepo,
	} {
		for _, weight := range []int64{-7, 0, 50} {
			t.Run(fmt.Sprintf("%s/weight=%d", repoType, weight), func(t *testing.T) {
				spoofType := types.DatasetRepo
				if repoType == types.DatasetRepo {
					spoofType = types.ModelRepo
				}
				requestCtx := types.WithAdminAuthorization(context.WithValue(context.Background(), requestKey{}, "request-value"))
				response := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(response)
				body := fmt.Sprintf(`{"repo_id":41,"weight":%d,"repo_type":%q,"repository_type":%q}`, weight, spoofType, spoofType)
				ctx.Request = httptest.NewRequest(http.MethodPost, "/recommendation?repo_type="+string(spoofType), strings.NewReader(body)).WithContext(requestCtx)
				ctx.Request.Header.Set("Content-Type", "application/json")
				ctx.Request.Header.Set("X-Repository-Type", string(spoofType))
				ctx.Set("repo_type", spoofType)
				ctx.Params = gin.Params{{Key: "repo_type", Value: string(spoofType)}}
				component := mockcomponent.NewMockRecomComponent(t)
				component.On("SetOpWeight", requestCtx, repoType, int64(41), weight).Return(nil).Once()
				h := &RecomHandler{c: component}

				h.SetOpWeight(repoType)(ctx)

				require.Equal(t, http.StatusOK, response.Code)
				var result httpbase.R
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
				require.Equal(t, "OK", result.Msg)
				require.Empty(t, result.Code)
				require.Nil(t, result.Data)
				require.Equal(t, "request-value", ctx.Request.Context().Value(requestKey{}))
				require.True(t, types.IsAdminAuthorized(ctx.Request.Context()))
			})
		}
	}
}

// TestRecomHandler_SetOpWeightBadRequest rejects invalid JSON before any repository operation.
func TestRecomHandler_SetOpWeightBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ name, body string }{
		{"missing repository ID", `{"weight":50}`},
		{"zero repository ID", `{"repo_id":0,"weight":50}`},
		{"null repository ID", `{"repo_id":null,"weight":50}`},
		{"string repository ID", `{"repo_id":"41","weight":50}`},
		{"fractional repository ID", `{"repo_id":41.5,"weight":50}`},
		{"string weight", `{"repo_id":41,"weight":"50"}`},
		{"fractional weight", `{"repo_id":41,"weight":50.5}`},
		{"malformed JSON", `{"repo_id":41,`},
		{"empty body", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/recommendation", strings.NewReader(tc.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			component := mockcomponent.NewMockRecomComponent(t)
			h := &RecomHandler{c: component}

			h.SetOpWeight(types.ModelRepo)(ctx)

			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Contains(t, response.Body.String(), "unmarshal body failed")
			component.AssertNotCalled(t, "SetOpWeight", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// TestRecomHandler_SetOpWeightErrors distinguishes module-boundary denial from database failures.
func TestRecomHandler_SetOpWeightErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"different repository type", errorx.ErrForbidden, http.StatusForbidden},
		{"repository type denial with message", errorx.ErrForbiddenMsg("repository type does not match the administrator submodule"), http.StatusForbidden},
		{"wrapped repository type denial", fmt.Errorf("repository type mismatch: %w", errorx.ErrForbidden), http.StatusForbidden},
		{"repository query failure", errors.New("failed to find repository"), http.StatusInternalServerError},
		{"weight write failure", errors.New("failed to write recommendation weight"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/recommendation", strings.NewReader(`{"repo_id":41,"weight":50}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			component := mockcomponent.NewMockRecomComponent(t)
			component.On("SetOpWeight", ctx.Request.Context(), types.ModelRepo, int64(41), int64(50)).Return(tc.err).Once()
			h := &RecomHandler{c: component}

			h.SetOpWeight(types.ModelRepo)(ctx)

			require.Equal(t, tc.status, response.Code)
			var result httpbase.R
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			require.NotEmpty(t, result.Msg)
		})
	}
}
