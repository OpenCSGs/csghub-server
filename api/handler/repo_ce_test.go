//go:build !ee && !saas

package handler

import (
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
		tester.mocks.repo.EXPECT().MirrorFromSaas(
			tester.Ctx(), "CSG_repo", "r", "u", types.ModelRepo,
		).Return(nil)

		tester.Execute()
		tester.ResponseEq(t, 200, tester.OKText, nil)
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
