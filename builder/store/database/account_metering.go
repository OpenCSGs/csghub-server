package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/common/types"
)

type accountMeteringStoreImpl struct {
	db *DB
}

type AccountMeteringStore interface {
	Create(ctx context.Context, input AccountMetering) error
	ListByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) ([]AccountMetering, int, error)
	GetStatByDate(ctx context.Context, req types.ActStatementsReq) ([]map[string]interface{}, error)
	ListAllByUserUUID(ctx context.Context, userUUID string) ([]AccountMetering, error)
	GetByEventUUID(ctx context.Context, eventUUID uuid.UUID) (*AccountMetering, error)
	FindByCustomerIDAndRecordAtInMin(ctx context.Context, customerID string, recordAt time.Time) (*AccountMetering, error)
}

func NewAccountMeteringStore() AccountMeteringStore {
	return &accountMeteringStoreImpl{
		db: defaultDB,
	}
}

func NewAccountMeteringStoreWithDB(db *DB) AccountMeteringStore {
	return &accountMeteringStoreImpl{
		db: db,
	}
}

type AccountMetering struct {
	ID           int64                 `bun:",pk,autoincrement" json:"id"`
	EventUUID    uuid.UUID             `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID     string                `bun:",notnull" json:"user_uuid"`
	Value        float64               `bun:",notnull" json:"value"`
	ValueType    types.ChargeValueType `bun:",notnull" json:"value_type"`
	Scene        types.SceneType       `bun:",notnull" json:"scene"`
	OpUID        string                `json:"op_uid"`
	ResourceID   string                `bun:",notnull" json:"resource_id"`
	ResourceName string                `bun:",notnull" json:"resource_name"`
	CustomerID   string                `json:"customer_id"`
	RecordedAt   time.Time             `bun:",notnull" json:"recorded_at"`
	Extra        string                `json:"extra"`
	CreatedAt    time.Time             `bun:",notnull,default:current_timestamp" json:"created_at"`
	SkuUnitType  string                `json:"sku_unit_type"`
}

func (am *accountMeteringStoreImpl) Create(ctx context.Context, input AccountMetering) error {
	res, err := am.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("failed to save metering event, error: %w", err)
	}
	return nil
}

func (am *accountMeteringStoreImpl) ListByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) ([]AccountMetering, int, error) {
	var accountMeters []AccountMetering
	q := am.db.Operator.Core.NewSelect().Model(&accountMeters).Where("user_uuid = ? and scene = ?", req.UserUUID, req.Scene)
	if len(req.InstanceName) > 0 {
		q = q.Where("customer_id = ?", req.InstanceName)
	}
	q = q.Where("recorded_at >= ? and recorded_at <= ?", req.StartTime, req.EndTime)

	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to counting metering recorders, error: %w", err)
	}

	_, err = q.Order("id DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &accountMeters)
	if err != nil {
		return nil, 0, fmt.Errorf("list all metering recorders, error: %w", err)
	}
	return accountMeters, count, nil
}

func (am *accountMeteringStoreImpl) GetStatByDate(ctx context.Context, req types.ActStatementsReq) ([]map[string]interface{}, error) {
	var meter []AccountMetering
	var res []map[string]interface{}
	err := am.db.Operator.Core.NewSelect().Model(&meter).
		ColumnExpr("users.username").
		ColumnExpr("account_metering.user_uuid").
		ColumnExpr("account_metering.resource_id").
		ColumnExpr("sum(account_metering.value) as value").
		Join("join users on users.uuid = account_metering.user_uuid").
		Where("account_metering.scene = ?", req.Scene).
		Where("account_metering.recorded_at >= ?", req.StartTime).
		Where("account_metering.recorded_at <= ?", req.EndTime).
		Group("users.username").
		Group("account_metering.user_uuid").
		Group("account_metering.resource_id").
		Order("account_metering.resource_id").
		Order("value desc").
		Scan(ctx, &res)

	if err != nil {
		return nil, fmt.Errorf("select metering stat, error: %w", err)
	}
	return res, nil
}

func (am *accountMeteringStoreImpl) ListAllByUserUUID(ctx context.Context, userUUID string) ([]AccountMetering, error) {
	var accountMeters []AccountMetering
	err := am.db.Operator.Core.NewSelect().Model(&accountMeters).Where("user_uuid = ?", userUUID).Scan(ctx, &accountMeters)
	if err != nil {
		return nil, fmt.Errorf("failed to list all meters by user uuid: %w", err)
	}
	return accountMeters, nil
}

func (am *accountMeteringStoreImpl) GetByEventUUID(ctx context.Context, eventUUID uuid.UUID) (*AccountMetering, error) {
	var result AccountMetering
	_, err := am.db.Core.NewSelect().Model(&result).Where("event_uuid = ?", eventUUID).Exec(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select metering by event uuid, error:%w", err)
	}
	return &result, nil
}

func (am *accountMeteringStoreImpl) FindByCustomerIDAndRecordAtInMin(ctx context.Context, customerID string, recordAt time.Time) (*AccountMetering, error) {
	var result AccountMetering
	startTime := recordAt.Truncate(time.Minute)
	endTime := recordAt.Add(time.Minute).Truncate(time.Minute)

	_, err := am.db.Core.NewSelect().Model(&result).
		Where("customer_id = ?", customerID).
		Where("recorded_at >= ? and recorded_at < ?", startTime, endTime).
		Limit(1).Exec(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select metering by customer_id and record_at, error:%w", err)
	}
	return &result, nil
}
