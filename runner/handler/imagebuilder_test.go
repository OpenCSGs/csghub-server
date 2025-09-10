package handler

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcom "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/runner/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type ImageBuilderTester struct {
	*testutil.GinTester
	handler *ImagebuilderHandler
	mocks   struct {
		imagebuilder *mockcom.MockImagebuilderComponent
	}
}

func (t *ImageBuilderTester) WithHandleFunc(fn func(h *ImagebuilderHandler) gin.HandlerFunc) *ImageBuilderTester {
	t.Handler(fn(t.handler))
	return t
}
func NewImageBuilderTester(t *testing.T) *ImageBuilderTester {
	tester := &ImageBuilderTester{GinTester: testutil.NewGinTester()}
	tester.mocks.imagebuilder = mockcom.NewMockImagebuilderComponent(t)

	tester.handler = &ImagebuilderHandler{
		ibc: tester.mocks.imagebuilder,
	}
	tester.WithParam("namespace", "testInternalId")
	tester.WithParam("name", "testUserId")
	return tester
}

func TestImagebuilderHandler_Stop(t *testing.T) {
	tester := NewImageBuilderTester(t).WithHandleFunc(func(h *ImagebuilderHandler) gin.HandlerFunc {
		return h.Stop
	})
	tester.WithBody(t, types.ImageBuildStopReq{
		OrgName:   "testOrg",
		SpaceName: "testSpace",
		DeployId:  "testImagebuilderId",
		TaskId:    "testTaskId",
	})
	tester.mocks.imagebuilder.EXPECT().Stop(tester.Ctx(), mock.Anything).Return(nil)
	tester.Execute()
	assert.Equal(t, http.StatusOK, tester.Response().Code)
}

func TestImagebuilderHandler_Build(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewImageBuilderTester(t).WithHandleFunc(func(h *ImagebuilderHandler) gin.HandlerFunc {
			return h.Build
		})
		tester.WithQuery("build_id", "testImagebuilderId")
		tester.WithBody(t, types.ImageBuilderRequest{
			SpaceName: "testSpace",
			OrgName:   "testOrg",
		})
		tester.mocks.imagebuilder.EXPECT().Build(tester.Ctx(), mock.Anything).Return(nil)
		tester.Execute()
	})

	t.Run("bad request", func(t *testing.T) {
		tester := NewImageBuilderTester(t).WithHandleFunc(func(h *ImagebuilderHandler) gin.HandlerFunc {
			return h.Build
		})
		tester.WithBody(t, "invalid json")
		tester.Execute()
		assert.Equal(t, http.StatusBadRequest, tester.Response().Code)
		assert.True(t, strings.Contains(tester.Response().Body.String(), "bad params imagebuilder request format"))
	})

	t.Run("internal server error", func(t *testing.T) {
		tester := NewImageBuilderTester(t).WithHandleFunc(func(h *ImagebuilderHandler) gin.HandlerFunc {
			return h.Build
		})
		tester.WithBody(t, types.ImageBuilderRequest{
			SpaceName: "testSpace",
			OrgName:   "testOrg",
		})
		tester.mocks.imagebuilder.EXPECT().Build(tester.Ctx(), mock.Anything).Return(errors.New("some error"))
		tester.Execute()
		tester.ResponseEqSimple(t, http.StatusInternalServerError, gin.H{
			"error": "fail to imagebuilder build:some error",
		})
	})
}
