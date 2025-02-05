package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type AccountingTester struct {
	*testutil.GinTester
	handler *AccountingHandler
	mocks   struct {
		accounting *mockcomponent.MockAccountingComponent
	}
}

func NewAccountingTester(t *testing.T) *AccountingTester {
	tester := &AccountingTester{GinTester: testutil.NewGinTester()}
	tester.mocks.accounting = mockcomponent.NewMockAccountingComponent(t)

	tester.handler = &AccountingHandler{
		accounting: tester.mocks.accounting,
		apiToken:   "testApiToken", // You can set this dynamically if needed
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *AccountingTester) WithHandleFunc(fn func(h *AccountingHandler) gin.HandlerFunc) *AccountingTester {
	t.Handler(fn(t.handler))
	return t
}

func TestAccountingHandler_QueryMeteringStatementByUserID(t *testing.T) {
	tester := NewAccountingTester(t).WithHandleFunc(func(h *AccountingHandler) gin.HandlerFunc {
		return h.QueryMeteringStatementByUserID
	})
	tester.RequireUser(t)

	tester.mocks.accounting.EXPECT().ListMeteringsByUserIDAndTime(
		tester.Ctx(), types.ACCT_STATEMENTS_REQ{
			CurrentUser:  "u",
			UserUUID:     "1",
			Scene:        2,
			InstanceName: "in",
			StartTime:    "2020-10-20 12:34:05",
			EndTime:      "2020-11-21 12:34:05",
			Per:          10,
			Page:         1,
		},
	).Return("go", nil)
	tester.AddPagination(1, 10).WithParam("id", "1").WithQuery("instance_name", "in").WithQuery("scene", "2")
	tester.WithQuery("start_time", "2020-10-20 12:34:05").WithQuery("end_time", "2020-11-21 12:34:05")
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, "go")
}
