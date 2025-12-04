//go:build !saas

package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestDatasetHandler_Create(t *testing.T) {
	t.Run("not allow public", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Create
		})
		tester.handler.config.Dataset.AllowCreatePublicDataset = false
		tester.WithUser()

		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Username: "u", Namespace: "u", Private: false},
		}).Return(true, nil)
		tester.WithBody(t, &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Private: false},
		}).Execute()

		tester.ResponseEqSimple(t, 400, httpbase.R{
			Code: errorx.ErrForbidden.Error(),
			Msg:  errorx.ErrForbidden.Error() + ": creating public dataset is not allowed",
		})
	})

	t.Run("allow private", func(t *testing.T) {

		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Create
		})
		tester.WithUser()

		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Username: "u", Namespace: "u", Private: true},
		}).Return(true, nil)
		tester.mocks.dataset.EXPECT().Create(tester.Ctx(), &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Namespace: "u", Private: true, Username: "u"},
		}).Return(&types.Dataset{Name: "d"}, nil)
		tester.WithBody(t, &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Private: true},
		}).Execute()

		tester.ResponseEqSimple(t, 200, httpbase.R{
			Data: &types.Dataset{Name: "d"},
			Msg:  "OK",
		})
	})

	t.Run("create private dataset for self", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Create
		})
		tester.handler.config.Dataset.AllowCreatePublicDataset = true
		tester.WithUser()

		req := &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{
				Username:  "u",
				Name:      "d",
				Namespace: "u",
				Nickname:  "nickname",
				Private:   true,
			},
		}

		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
		tester.mocks.dataset.EXPECT().Create(tester.Ctx(), req).Return(&types.Dataset{Name: "d"}, nil)

		tester.WithBody(t, req).Execute()

		require.Equal(t, http.StatusOK, tester.Response().Code)
		tester.ResponseEq(t, http.StatusOK, tester.OKText, &types.Dataset{Name: "d"})
	})
}
