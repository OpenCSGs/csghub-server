package component

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/accounting/utils"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type meteringComponentImpl struct {
	ams database.AccountMeteringStore
}

type MeteringComponent interface {
	SaveMeteringEventRecord(ctx context.Context, req *types.MeteringEvent) error
	ListMeteringByUserIDAndDate(ctx context.Context, req types.ActStatementsReq) ([]database.AccountMetering, int, error)
	GetMeteringStatByDate(ctx context.Context, req types.ActStatementsReq) ([]map[string]interface{}, error)
	GetMeteringByEventUUID(ctx context.Context, eventUUID uuid.UUID) (*database.AccountMetering, error)
	FindMeteringByCustomerIDAndRecordAtInMin(ctx context.Context, customerID string, recordAt time.Time) (*database.AccountMetering, error)
}

func NewMeteringComponent() MeteringComponent {
	ams := &meteringComponentImpl{
		ams: database.NewAccountMeteringStore(),
	}
	return ams
}

func (mc *meteringComponentImpl) SaveMeteringEventRecord(ctx context.Context, req *types.MeteringEvent) error {
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
		SkuUnitType:  string(utils.GetSkuUnitTypeByScene(types.SceneType(req.Scene))),
	}
	err := mc.ams.Create(ctx, am)
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
