//go:build !saas

package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/types"
)

func TestDatasetHandler_Create(t *testing.T) {
	t.Run("no public", func(t *testing.T) {

		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Create
		})
		tester.RequireUser(t)

		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Private: true},
		}).Return(true, nil)
		tester.mocks.dataset.EXPECT().Create(tester.Ctx(), &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Private: true, Username: "u"},
		}).Return(&types.Dataset{Name: "d"}, nil)
		tester.WithBody(t, &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Private: true},
		}).Execute()

		tester.ResponseEqSimple(t, 200, gin.H{
			"data": &types.Dataset{Name: "d"},
		})

	})

	t.Run("public", func(t *testing.T) {

		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Create
		})
		tester.RequireUser(t)

		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Private: false},
		}).Return(true, nil)
		tester.mocks.dataset.EXPECT().Create(tester.Ctx(), &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Private: false, Username: "u"},
		}).Return(&types.Dataset{Name: "d"}, nil)
		tester.WithBody(t, &types.CreateDatasetReq{
			CreateRepoReq: types.CreateRepoReq{Private: false},
		}).Execute()

		tester.ResponseEqSimple(t, 200, gin.H{
			"data": &types.Dataset{Name: "d"},
		})
	})

}
