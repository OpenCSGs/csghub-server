package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type MeteringComponent struct {
	ams *database.AccountMeteringStore
}

func NewMeteringComponent() *MeteringComponent {
	ams := &MeteringComponent{
		ams: database.NewAccountMeteringStore(),
	}
	return ams
}

func (mc *MeteringComponent) SaveMeteringEventRecord(ctx context.Context, req *types.METERING_EVENT) error {
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
		SkuUnitType:  getUnitString(req.Scene),
	}
	err := mc.ams.Create(ctx, am)
	if err != nil {
		return fmt.Errorf("failed to save metering event record, error: %w", err)
	}
	return nil
}

func (mc *MeteringComponent) ListMeteringByUserIDAndDate(ctx context.Context, req types.ACCT_STATEMENTS_REQ) ([]database.AccountMetering, int, error) {
	meters, total, err := mc.ams.ListByUserIDAndTime(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list metering by UserIDAndDate, error: %w", err)
	}
	return meters, total, nil
}

func getUnitString(scene int) string {
	switch types.SceneType(scene) {
	case types.SceneModelInference:
		return types.UnitMinute
	case types.SceneSpace:
		return types.UnitMinute
	case types.SceneModelFinetune:
		return types.UnitMinute
	case types.SceneStarship:
		return types.UnitToken
	case types.SceneMultiSync:
		return types.UnitRepo
	default:
		return types.UnitMinute
	}
}
