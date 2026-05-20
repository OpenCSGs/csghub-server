package handler

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type ActivityLogTester struct {
	*testutil.GinTester
	handler *ActivityLogHandler
	mocks   struct {
		comp *mockcomponent.MockActivityLogComponent
	}
}

func NewActivityLogTester(t *testing.T) *ActivityLogTester {
	tester := &ActivityLogTester{GinTester: testutil.NewGinTester()}
	tester.mocks.comp = mockcomponent.NewMockActivityLogComponent(t)
	tester.handler = &ActivityLogHandler{comp: tester.mocks.comp}
	return tester
}

func (t *ActivityLogTester) WithHandleFunc(fn func(h *ActivityLogHandler) gin.HandlerFunc) *ActivityLogTester {
	t.Handler(fn(t.handler))
	return t
}

func TestActivityLogHandler_List(t *testing.T) {
	tester := NewActivityLogTester(t).WithHandleFunc(func(h *ActivityLogHandler) gin.HandlerFunc {
		return h.List
	})
	tester.WithUser()

	tester.mocks.comp.EXPECT().ListActivityLogs(tester.Ctx(), mock.MatchedBy(func(req types.QueryActivityLogReq) bool {
		return req.Per == 20 && req.Page == 1
	})).Return([]database.ActivityLog{
		{ID: 1, Username: "user1", Action: "create", ResourceType: "models", ResourceName: "ns/model1"},
	}, 1, nil)

	tester.AddPagination(1, 20).WithQuery("after", time.Now().Format(time.RFC3339)).Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"msg": "OK",
		"data": []database.ActivityLog{
			{ID: 1, Username: "user1", Action: "create", ResourceType: "models", ResourceName: "ns/model1"},
		},
		"total": 1,
	})
}

func TestActivityLogHandler_ListInvalidAfter(t *testing.T) {
	tester := NewActivityLogTester(t).WithHandleFunc(func(h *ActivityLogHandler) gin.HandlerFunc {
		return h.List
	})
	tester.WithUser()

	tester.AddPagination(1, 10).WithQuery("after", "invalid-time").Execute()

	tester.ResponseEqCode(t, 400)
}
