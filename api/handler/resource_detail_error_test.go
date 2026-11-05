package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/errorx"
)

func TestResourceDetailErrorStatus(t *testing.T) {
	for _, resource := range []struct {
		name             string
		permissionError  error
		permissionStatus int
		handler          func(*testing.T, error) gin.HandlerFunc
	}{
		{name: "Model", permissionError: errorx.ErrForbidden, permissionStatus: http.StatusForbidden, handler: func(t *testing.T, err error) gin.HandlerFunc {
			comp := mockcomponent.NewMockModelComponent(t)
			comp.EXPECT().Show(mock.Anything, "aiwizards", "juliadoc-mcp", "", false, true).Return(nil, err)
			return (&ModelHandler{model: comp}).Show
		}},
		{name: "Dataset", permissionError: errorx.ErrForbidden, permissionStatus: http.StatusForbidden, handler: func(t *testing.T, err error) gin.HandlerFunc {
			comp := mockcomponent.NewMockDatasetComponent(t)
			comp.EXPECT().Show(mock.Anything, "aiwizards", "juliadoc-mcp", "", false, true).Return(nil, err)
			return (&DatasetHandler{dataset: comp}).Show
		}},
		{name: "Space", permissionError: errorx.ErrUnauthorized, permissionStatus: http.StatusUnauthorized, handler: func(t *testing.T, err error) gin.HandlerFunc {
			comp := mockcomponent.NewMockSpaceComponent(t)
			comp.EXPECT().Show(mock.Anything, "aiwizards", "juliadoc-mcp", "", false).Return(nil, err)
			return (&SpaceHandler{space: comp}).Show
		}},
		{name: "Code", permissionError: errorx.ErrForbidden, permissionStatus: http.StatusForbidden, handler: func(t *testing.T, err error) gin.HandlerFunc {
			comp := mockcomponent.NewMockCodeComponent(t)
			comp.EXPECT().Show(mock.Anything, "aiwizards", "juliadoc-mcp", "", false, true).Return(nil, err)
			return (&CodeHandler{code: comp}).Show
		}},
		{name: "Skill", permissionError: errorx.ErrForbidden, permissionStatus: http.StatusForbidden, handler: func(t *testing.T, err error) gin.HandlerFunc {
			comp := mockcomponent.NewMockSkillComponent(t)
			comp.EXPECT().Show(mock.Anything, "aiwizards", "juliadoc-mcp", "", false, true).Return(nil, err)
			return (&SkillHandler{skill: comp}).Show
		}},
		{name: "Prompt", permissionError: errorx.ErrUnauthorized, permissionStatus: http.StatusUnauthorized, handler: func(t *testing.T, err error) gin.HandlerFunc {
			comp := mockcomponent.NewMockPromptComponent(t)
			comp.EXPECT().Show(mock.Anything, "aiwizards", "juliadoc-mcp", "", false, true).Return(nil, err)
			return (&PromptHandler{prompt: comp}).PromptDetail
		}},
		{name: "MCPServer", permissionError: errorx.ErrForbidden, permissionStatus: http.StatusForbidden, handler: func(t *testing.T, err error) gin.HandlerFunc {
			comp := mockcomponent.NewMockMCPServerComponent(t)
			comp.EXPECT().Show(mock.Anything, "aiwizards", "juliadoc-mcp", "", false, true).Return(nil, err)
			return (&MCPServerHandler{mcpComp: comp}).Show
		}},
	} {
		t.Run(resource.name, func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				err    error
				status int
			}{
				{"missing_resource", fmt.Errorf("lookup failed: %w", errorx.RepoNotFound(sql.ErrNoRows, nil)), http.StatusNotFound},
				{"missing_resource_with_db_error", fmt.Errorf("lookup failed: %w", errorx.RepoNotFound(errorx.HandleDBError(sql.ErrNoRows, nil), nil)), http.StatusNotFound},
				{"secondary_custom_no_rows", errorx.HandleDBError(sql.ErrNoRows, nil), http.StatusInternalServerError},
				{"database_failure", errors.New("database unavailable"), http.StatusInternalServerError},
				{"secondary_lookup_missing", fmt.Errorf("related data: %w", sql.ErrNoRows), http.StatusInternalServerError},
				{"permission_denied", resource.permissionError, resource.permissionStatus},
			} {
				t.Run(tc.name, func(t *testing.T) {
					router := gin.New()
					router.GET("/:namespace/:name", resource.handler(t, tc.err))
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/aiwizards/juliadoc-mcp?need_multi_sync=true", nil))
					require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
					if tc.name == "missing_resource" {
						require.Contains(t, recorder.Body.String(), "REPO-ERR-3")
					}
				})
			}
		})
	}
}
