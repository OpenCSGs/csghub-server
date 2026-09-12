package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type MeteringHandlerTester struct {
	*testutil.GinTester
	handler *MeteringHandler
	mocks   struct {
		meterComp *mockcomponent.MockMeteringComponent
	}
}

func NewMeteringHandlerTester(t *testing.T) *MeteringHandlerTester {
	tester := &MeteringHandlerTester{GinTester: testutil.NewGinTester()}
	tester.mocks.meterComp = mockcomponent.NewMockMeteringComponent(t)

	tester.handler = &MeteringHandler{
		amc: tester.mocks.meterComp,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *MeteringHandlerTester) WithHandleFunc(fn func(h *MeteringHandler) gin.HandlerFunc) *MeteringHandlerTester {
	t.Handler(fn(t.handler))
	return t
}

func TestMeterHandler_QueryMeteringStatementByUserID(t *testing.T) {
	tester := NewMeteringHandlerTester(t)
	tester.WithHandleFunc(func(h *MeteringHandler) gin.HandlerFunc {
		return h.QueryMeteringStatementByUserID
	})

	req := types.ActStatementsReq{
		UserUUID:     "1234567890",
		Scene:        10,
		InstanceName: "abc",
		StartTime:    "2024-06-12 08:27:22",
		EndTime:      "2024-06-12 08:27:22",
		Per:          10,
		Page:         1,
	}

	tester.mocks.meterComp.EXPECT().ListMeteringByUserIDAndDate(mock.Anything, req).Return(
		[]database.AccountMetering{
			{
				ID: 1,
			},
		},
		1,
		nil,
	)

	tester.WithQuery("start_time", req.StartTime).
		WithQuery("end_time", req.EndTime).
		WithQuery("instance_name", req.InstanceName).
		WithQuery("per", "10").
		WithQuery("page", "1").
		WithQuery("scene", "10").
		WithParam("id", req.UserUUID).
		Execute()

	tester.ResponseEq(t, 200, tester.OKText,
		&gin.H{
			"total": 1,
			"data": []database.AccountMetering{
				{
					ID: 1,
				},
			},
		})
}

func TestMeterHandler_QueryMeteringStatByDate(t *testing.T) {
	tester := NewMeteringHandlerTester(t)
	tester.WithHandleFunc(func(h *MeteringHandler) gin.HandlerFunc {
		return h.QueryMeteringStatByDate
	})

	req := types.ActStatementsReq{
		Scene:     10,
		StartTime: "2024-06-01 00:00:00",
		EndTime:   "2024-06-30 23:59:59",
	}

	tester.mocks.meterComp.EXPECT().GetMeteringStatByDate(mock.Anything, req).Return(
		[]map[string]interface{}{},
		nil,
	)

	tester.WithQuery("start_date", "2024-06-01").
		WithQuery("end_date", "2024-06-30").
		WithQuery("scene", "10").
		Execute()

	tester.ResponseEq(t, 200, tester.OKText, []map[string]interface{}{})

}

func TestMeterHandler_QueryStatisticsByUserID(t *testing.T) {
	tester := NewMeteringHandlerTester(t)
	tester.WithHandleFunc(func(h *MeteringHandler) gin.HandlerFunc {
		return h.QueryStatisticsByUserID
	})

	req := types.AcctBillsReq{
		TargetUUID:   "1234567890",
		Scene:        10,
		StartDate:    "2024-06-12",
		EndDate:      "2024-06-12",
		Per:          10,
		Page:         1,
		InstanceName: "abc",
	}

	tester.mocks.meterComp.EXPECT().ListStatisticsByUserIDAndDate(mock.Anything, req).Return(
		database.AccountStatisticsRes{},
		nil,
	)

	tester.WithQuery("start_date", req.StartDate).
		WithQuery("end_date", req.EndDate).
		WithQuery("instance_name", req.InstanceName).
		WithQuery("per", "10").
		WithQuery("page", "1").
		WithQuery("scene", "10").
		WithParam("id", req.TargetUUID).
		Execute()

	tester.ResponseEq(t, 200, tester.OKText, database.AccountStatisticsRes{})
}

func TestMeterHandler_QueryStatisticsDetailByUserID(t *testing.T) {
	tester := NewMeteringHandlerTester(t)
	tester.WithHandleFunc(func(h *MeteringHandler) gin.HandlerFunc {
		return h.QueryStatisticsDetailByUserID
	})

	req := types.AcctBillsDetailReq{
		TargetUUID:   "1234567890",
		Scene:        10,
		StartDate:    "2024-06-12",
		EndDate:      "2024-06-12",
		InstanceName: "abc",
		Per:          10,
		Page:         1,
		UnitType:     "",
	}

	tester.mocks.meterComp.EXPECT().ListStatisticsDetailByUserID(mock.Anything, req).Return(
		database.AccountStatisticsDetailRes{},
		nil,
	)

	tester.WithQuery("start_date", req.StartDate).
		WithQuery("end_date", req.EndDate).
		WithQuery("instance_name", req.InstanceName).
		WithQuery("per", "10").
		WithQuery("page", "1").
		WithQuery("scene", "10").
		WithParam("id", req.TargetUUID).
		Execute()

	tester.ResponseEq(t, 200, tester.OKText, database.AccountStatisticsDetailRes{})
}

func TestMeterHandler_QueryStatisticsSummary(t *testing.T) {
	tester := NewMeteringHandlerTester(t)
	tester.WithHandleFunc(func(h *MeteringHandler) gin.HandlerFunc {
		return h.QueryStatisticsSummary
	})

	req := types.AcctBillsReq{
		TargetUUID: "1234567890",
		Scene:      10,
		StartDate:  "2024-06-12",
		EndDate:    "2024-06-12",
	}

	tester.mocks.meterComp.EXPECT().GetStatisticsSummary(mock.Anything, req).Return(
		database.AccountStatisticsSummaryRes{},
		nil,
	)

	tester.WithQuery("start_date", req.StartDate).
		WithQuery("end_date", req.EndDate).
		WithQuery("scene", "10").
		WithParam("id", req.TargetUUID).
		Execute()

	tester.ResponseEq(t, 200, tester.OKText, database.AccountStatisticsSummaryRes{})
}
