package handler

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func TestModelHandler_OptimizeEngineArgs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.OptimizeEngineArgs
		})
		tester.WithUser()

		req := &types.OptimizeEngineArgsReq{
			RuntimeFrameworkID: 1,
			ResourceID:         1,
			EngineArgs:         `{"gpu-memory-utilization":"0.85"}`,
			EnablePD:           false,
		}

		tester.mocks.repo.EXPECT().AllowReadAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(true, nil)
		tester.mocks.model.EXPECT().OptimizeEngineArgs(tester.Ctx(), "u", "r", "u", *req).Return(
			&types.OptimizeEngineArgsResp{
				EngineArgs: map[string]string{
					"gpu-memory-utilization": "0.92",
					"max-model-len":          "32768",
				},
			}, nil,
		)

		tester.WithBody(t, req).Execute()

		tester.ResponseEqSimple(t, 200, gin.H{
			"data": &types.OptimizeEngineArgsResp{
				EngineArgs: map[string]string{
					"gpu-memory-utilization": "0.92",
					"max-model-len":          "32768",
				},
			},
			"msg": "OK",
		})
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.OptimizeEngineArgs
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().AllowReadAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(false, nil)

		tester.WithBody(t, &types.OptimizeEngineArgsReq{
			RuntimeFrameworkID: 1,
			ResourceID:         1,
			EngineArgs:         `{}`,
		}).Execute()

		require.Equal(t, http.StatusForbidden, tester.Response().Code)
	})

	t.Run("bad request body", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.OptimizeEngineArgs
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().AllowReadAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(true, nil)

		tester.WithBody(t, "invalid json").Execute()

		require.Equal(t, http.StatusBadRequest, tester.Response().Code)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.OptimizeEngineArgs
		})
		tester.WithUser()

		req := &types.OptimizeEngineArgsReq{
			RuntimeFrameworkID: 1,
			ResourceID:         1,
			EngineArgs:         `{}`,
		}

		tester.mocks.repo.EXPECT().AllowReadAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(true, nil)
		tester.mocks.model.EXPECT().OptimizeEngineArgs(tester.Ctx(), "u", "r", "u", *req).Return(
			nil, errors.New("internal error"),
		)

		tester.WithBody(t, req).Execute()

		require.Equal(t, http.StatusInternalServerError, tester.Response().Code)
	})

	t.Run("repo access error", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.OptimizeEngineArgs
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().AllowReadAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(false, errors.New("db error"))

		tester.WithBody(t, &types.OptimizeEngineArgsReq{
			RuntimeFrameworkID: 1,
			ResourceID:         1,
			EngineArgs:         `{}`,
		}).Execute()

		require.Equal(t, http.StatusInternalServerError, tester.Response().Code)
	})
}
