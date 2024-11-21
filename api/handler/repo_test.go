package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func TestRepoHandler_LastCommit(t *testing.T) {
	t.Run("forbidden", func(t *testing.T) {
		comp := mockcomponent.NewMockRepoComponent(t)
		h := &RepoHandler{comp}

		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		ginc.AddParam("namespace", "user_name_1")
		ginc.AddParam("name", "repo_name_1")

		//user does not have permission to access repo
		comp.EXPECT().LastCommit(mock.Anything, mock.Anything).Return(nil, component.ErrForbidden).Once()
		h.LastCommit(ginc)
		require.Equal(t, http.StatusForbidden, response.Code)
	})

	t.Run("server error", func(t *testing.T) {
		comp := mockcomponent.NewMockRepoComponent(t)
		h := &RepoHandler{comp}

		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		ginc.AddParam("namespace", "user_name_1")
		ginc.AddParam("name", "repo_name_1")

		commit := &types.Commit{}

		comp.EXPECT().LastCommit(mock.Anything, mock.Anything).Return(commit, errors.New("custome error")).Once()
		h.LastCommit(ginc)
		require.Equal(t, http.StatusInternalServerError, response.Code)
	})

	t.Run("success", func(t *testing.T) {
		comp := mockcomponent.NewMockRepoComponent(t)
		h := &RepoHandler{comp}

		response := httptest.NewRecorder()
		ginc, _ := gin.CreateTestContext(response)
		ginc.AddParam("namespace", "user_name_1")
		ginc.AddParam("name", "repo_name_1")

		commit := &types.Commit{}
		commit.AuthorName = "user_name_1"
		commit.ID = uuid.New().String()

		comp.EXPECT().LastCommit(mock.Anything, mock.Anything).Return(commit, nil).Once()
		h.LastCommit(ginc)
		require.Equal(t, http.StatusOK, response.Code)

		var r = struct {
			Code int           `json:"code,omitempty"`
			Msg  string        `json:"msg"`
			Data *types.Commit `json:"data,omitempty"`
		}{}
		err := json.Unmarshal(response.Body.Bytes(), &r)
		require.Empty(t, err)
		require.Equal(t, commit.ID, r.Data.ID)
	})
}
