package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/common/types"
	commonTypes "opencsg.com/csghub-server/common/types"
)

type AccountMeteringStore struct {
	db *DB
}

func NewAccountMeteringStore() *AccountMeteringStore {
	return &AccountMeteringStore{
		db: defaultDB,
	}
}

type AccountMetering struct {
	ID           int64           `bun:",pk,autoincrement" json:"id"`
	EventUUID    uuid.UUID       `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID     string          `bun:",notnull" json:"user_uuid"`
	Value        float64         `bun:",notnull" json:"value"`
	ValueType    int             `bun:",notnull" json:"value_type"`
	Scene        types.SceneType `bun:",notnull" json:"scene"`
	OpUID        string          `json:"op_uid"`
	ResourceID   string          `bun:",notnull" json:"resource_id"`
	ResourceName string          `bun:",notnull" json:"resource_name"`
	CustomerID   string          `json:"customer_id"`
	RecordedAt   time.Time       `bun:",notnull" json:"recorded_at"`
	Extra        string          `json:"extra"`
	CreatedAt    time.Time       `bun:",notnull,default:current_timestamp" json:"created_at"`
	SkuUnitType  string          `json:"sku_unit_type"`
}

func (am *AccountMeteringStore) Create(ctx context.Context, input AccountMetering) error {
	res, err := am.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("failed to save metering event, error: %w", err)
	}
	return nil
}

func (am *AccountMeteringStore) ListByUserIDAndTime(ctx context.Context, req commonTypes.ACCT_STATEMENTS_REQ) ([]AccountMetering, int, error) {
	var accountMeters []AccountMetering
	q := am.db.Operator.Core.NewSelect().Model(&accountMeters).Where("user_uuid = ? and scene = ? and customer_id = ? and recorded_at >= ? and recorded_at <= ?", req.UserUUID, req.Scene, req.InstanceName, req.StartTime, req.EndTime)

	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to counting recorders, error: %w", err)
	}

	_, err = q.Order("id DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &accountMeters)
	if err != nil {
		return nil, 0, fmt.Errorf("list all meters, error: %w", err)
	}
	return accountMeters, count, nil
}
