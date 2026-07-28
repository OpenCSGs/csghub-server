//go:build !ee && !saas

package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoHandler_MirrorFromSaas(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.MirrorFromSaas
		})
		tester.WithUser()

		tester.WithParam("namespace", types.OpenCSGPrefix+"repo")
		tester.WithKV("repo_type", types.ModelRepo)
		result := &types.MirrorFromSaasResponse{RepositoryID: 1, MirrorID: 2, TaskID: 3, Status: types.MirrorQueued}
		tester.mocks.mirror.EXPECT().MirrorFromSaas(
			tester.Ctx(), types.MirrorFromSaasReq{
				Namespace: "CSG_repo", Name: "r", RepoType: types.ModelRepo, CurrentUser: "u",
			},
		).Return(result, nil)

		tester.Execute()
		tester.ResponseEq(t, http.StatusAccepted, "Accepted", result)
	})

	t.Run("invalid", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.MirrorFromSaas
		})
		tester.WithUser()

		tester.WithKV("repo_type", types.ModelRepo)
		tester.Execute()
		tester.ResponseEq(t, 400, "Repo could not be mirrored", nil)
	})
}

func TestRepoHandler_MirrorFromSaasStatus(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.MirrorFromSaasStatus
		})
		tester.WithUser().WithParam("namespace", types.OpenCSGPrefix+"repo").WithKV("repo_type", types.ModelRepo).WithQuery("task_id", "30")
		result := &types.MirrorSyncStatusResponse{RepositoryID: 1, MirrorID: 2, TaskID: 30, Status: types.MirrorRepoSyncStart, Phase: types.MirrorSyncPhaseRepo}
		tester.mocks.mirror.EXPECT().MirrorFromSaasStatus(tester.Ctx(), types.MirrorFromSaasStatusReq{
			Namespace: "CSG_repo", Name: "r", RepoType: types.ModelRepo, CurrentUser: "u", RequestedTaskID: 30,
		}).Return(result, nil)

		tester.Execute()
		tester.ResponseEq(t, http.StatusOK, tester.OKText, result)
	})

	t.Run("invalid task id", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.MirrorFromSaasStatus
		})
		tester.WithUser().WithParam("namespace", types.OpenCSGPrefix+"repo").WithQuery("task_id", "invalid")

		tester.Execute()
		tester.ResponseEqCode(t, http.StatusBadRequest)
	})
}
