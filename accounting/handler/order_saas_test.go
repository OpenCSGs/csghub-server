//go:build saas

package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
)

type OrderHandlerTester struct {
	*testutil.GinTester
	handler *OrderHandler
	mocks   struct {
		orderComp *mockcomponent.MockAccountingOrderComponent
	}
}

func NewOrderHandlerTester(t *testing.T) *OrderHandlerTester {
	tester := &OrderHandlerTester{GinTester: testutil.NewGinTester()}
	tester.mocks.orderComp = mockcomponent.NewMockAccountingOrderComponent(t)
	tester.handler = &OrderHandler{
		nao: tester.mocks.orderComp,
	}
	return tester
}

func (t *OrderHandlerTester) WithHandleFunc(fn func(h *OrderHandler) gin.HandlerFunc) *OrderHandlerTester {
	t.Handler(fn(t.handler))
	return t
}

func TestOrderHandler_OrderGetByID(t *testing.T) {
	tester := NewOrderHandlerTester(t)
	tester.WithHandleFunc(func(h *OrderHandler) gin.HandlerFunc {
		return h.OrderGetByID
	})

	expectedOrder := &database.AccountOrder{
		OrderUUID: "test-order-uuid",
	}

	tester.mocks.orderComp.EXPECT().GetByID(mock.Anything, expectedOrder.OrderUUID).
		Return(expectedOrder, nil)

	tester.WithParam("id", expectedOrder.OrderUUID).Execute()
	tester.ResponseEq(t, 200, tester.OKText, expectedOrder)
}
