package component

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type meteringComponentImpl struct {
	ams database.AccountMeteringStore
	ass database.AccountStatisticsStore
}

type MeteringComponent interface {
	SaveMeteringEventRecord(ctx context.Context, req *types.MeteringEvent, extra types.MeteringExtra) error
	ListMeteringByUserIDAndDate(ctx context.Context, req types.ActStatementsReq) ([]database.AccountMetering, int, error)
	GetMeteringStatByDate(ctx context.Context, req types.ActStatementsReq) ([]map[string]interface{}, error)
	GetMeteringByEventUUID(ctx context.Context, eventUUID uuid.UUID) (*database.AccountMetering, error)
	FindMeteringByCustomerIDAndRecordAtInMin(ctx context.Context, customerID string, recordAt time.Time) (*database.AccountMetering, error)
	ListStatisticsByUserIDAndDate(ctx context.Context, req types.AcctBillsReq) (database.AccountStatisticsRes, error)
	ListStatisticsDetailByUserID(ctx context.Context, req types.AcctBillsDetailReq) (database.AccountStatisticsDetailRes, error)
	GetStatisticsSummary(ctx context.Context, req types.AcctBillsReq) (database.AccountStatisticsSummaryRes, error)
}

func NewMeteringComponent() MeteringComponent {
	ams := &meteringComponentImpl{
		ams: database.NewAccountMeteringStore(),
		ass: database.NewAccountStatisticsStore(),
	}
	return ams
}

func (mc *meteringComponentImpl) SaveMeteringEventRecord(ctx context.Context, req *types.MeteringEvent, extra types.MeteringExtra) error {
	am := database.AccountMetering{
		EventUUID:    req.Uuid,
		UserUUID:     req.UserUUID,
		Value:        float64(req.Value),
		ValueType:    req.ValueType,
		Scene:        types.SceneType(req.Scene),
		OpUID:        req.OpUID,
		ResourceID:   req.ResourceID,
		ResourceName: req.ResourceName,
		CustomerID:   req.CustomerID,
		RecordedAt:   req.CreatedAt,
		Extra:        req.Extra,
		SkuUnitType:  extra.SkuUnitType,
	}
	err := mc.ams.Create(ctx, am, extra)
	if err != nil {
		return fmt.Errorf("failed to save metering event record, error: %w", err)
	}
	return nil
}

func (mc *meteringComponentImpl) ListMeteringByUserIDAndDate(ctx context.Context, req types.ActStatementsReq) ([]database.AccountMetering, int, error) {
	meters, total, err := mc.ams.ListByUserIDAndTime(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list metering by UserIDAndDate, error: %w", err)
	}
	return meters, total, nil
}

func (mc *meteringComponentImpl) GetMeteringStatByDate(ctx context.Context, req types.ActStatementsReq) ([]map[string]interface{}, error) {
	res, err := mc.ams.GetStatByDate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("fail to get metering stat, error: %w", err)
	}
	return res, nil
}

func (mc *meteringComponentImpl) GetMeteringByEventUUID(ctx context.Context, eventUUID uuid.UUID) (*database.AccountMetering, error) {
	metering, err := mc.ams.GetByEventUUID(ctx, eventUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to find metering by event uuid, error: %w", err)
	}
	return metering, nil
}

func (mc *meteringComponentImpl) FindMeteringByCustomerIDAndRecordAtInMin(ctx context.Context, customerID string, recordAt time.Time) (*database.AccountMetering, error) {
	metering, err := mc.ams.FindByCustomerIDAndRecordAtInMin(ctx, customerID, recordAt)
	if err != nil {
		return nil, fmt.Errorf("failed to find metering by customer id and record at, error: %w", err)
	}
	return metering, nil
}

func (mc *meteringComponentImpl) ListStatisticsByUserIDAndDate(ctx context.Context, req types.AcctBillsReq) (database.AccountStatisticsRes, error) {
	res, err := mc.ass.ListByUserIDAndDate(ctx, req)
	if err != nil {
		return database.AccountStatisticsRes{}, fmt.Errorf("failed to list statistics by UserIDAndDate, error: %w", err)
	}
	return res, nil
}

func (mc *meteringComponentImpl) ListStatisticsDetailByUserID(ctx context.Context, req types.AcctBillsDetailReq) (database.AccountStatisticsDetailRes, error) {
	res, err := mc.ass.ListStatisticsDetailByUserID(ctx, req)
	if err != nil {
		return database.AccountStatisticsDetailRes{}, fmt.Errorf("failed to list statistics detail by user, error: %w", err)
	}
	return res, nil
}

func (mc *meteringComponentImpl) GetStatisticsSummary(ctx context.Context, req types.AcctBillsReq) (database.AccountStatisticsSummaryRes, error) {
	res, err := mc.ass.SummaryByUserIDAndDate(ctx, req)
	if err != nil {
		return database.AccountStatisticsSummaryRes{}, fmt.Errorf("failed to get statistics summary, error: %w", err)
	}
	return res, nil
}
