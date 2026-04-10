package database

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type accountOrderStoreImpl struct {
	db *DB
}

type AccountOrderStore interface {
	Create(ctx context.Context, order AccountOrder, statement AccountStatement) error
	GetByID(ctx context.Context, uuid string) (*AccountOrder, error)
	GetDetailByID(ctx context.Context, id int64) (*AccountOrderDetail, error)
}

func NewAccountOrderStore() AccountOrderStore {
	return &accountOrderStoreImpl{
		db: defaultDB,
	}
}

func NewAccountOrderStoreWithDB(db *DB) AccountOrderStore {
	return &accountOrderStoreImpl{
		db: db,
	}
}

type AccountOrder struct {
	OrderUUID   string               `bun:",notnull,pk" json:"order_uuid"`
	UserUUID    string               `bun:",notnull" json:"user_uuid"`
	OrderStatus types.OrderStatus    `bun:",notnull" json:"order_status"`
	Amount      float64              `bun:",notnull" json:"amount"`
	CreatedAt   time.Time            `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	EventUUID   string               `json:"event_uuid"`
	RecordedAt  time.Time            `json:"recorded_at"`
	Details     []AccountOrderDetail `bun:"rel:has-many,join:order_uuid=order_uuid" json:"details"`
}

type AccountOrderDetail struct {
	ID          int64         `bun:",pk,autoincrement" json:"id"`
	OrderUUID   string        `bun:",notnull" json:"order_uuid"`
	ResourceID  string        `bun:",notnull" json:"resource_id"`
	SkuType     types.SKUType `bun:",notnull" json:"sku_type"`
	SkuKind     types.SKUKind `bun:",notnull" json:"sku_kind"`
	SkuUnitType string        `bun:",notnull" json:"sku_unit_type"`
	OrderCount  int           `bun:",notnull" json:"order_count"`
	SkuPriceID  int64         `bun:",notnull" json:"sku_price_id"`
	Amount      float64       `bun:",notnull" json:"amount"`
	BeginTime   time.Time     `bun:",notnull" json:"begin_time"`
	EndTime     time.Time     `bun:",notnull" json:"end_time"`
	CreatedAt   time.Time     `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	PresentUUID string        `json:"present_uuid"`
}

func (aos *accountOrderStoreImpl) Create(ctx context.Context, order AccountOrder, input AccountStatement) error {
	err := aos.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if input.Scene != types.ScenePayOrder {
			return fmt.Errorf("invalid scene, expect %v, got %v", types.ScenePayOrder, input.Scene)
		}

		var err error

		err = assertAffectedOneRow(tx.NewInsert().Model(&order).Exec(ctx))
		if err != nil {
			return fmt.Errorf("insert order, error:%w", err)
		}

		for _, detail := range order.Details {
			err = assertAffectedOneRow(tx.NewInsert().Model(&detail).Exec(ctx))
			if err != nil {
				return fmt.Errorf("insert order detail, error:%w", err)
			}
		}

		err = DeductAccountFee(ctx, tx, input)
		if err != nil {
			return fmt.Errorf("deduct account fee for order, error:%w", err)
		}

		return nil
	})

	return err
}

func (aos *accountOrderStoreImpl) GetByID(ctx context.Context, uuid string) (*AccountOrder, error) {
	var order AccountOrder
	err := aos.db.Operator.Core.NewSelect().Model(&order).Where("order_uuid = ?", uuid).Relation("Details").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select account order by uuid, error:%w", err)
	}
	return &order, nil
}

func (aos *accountOrderStoreImpl) GetDetailByID(ctx context.Context, id int64) (*AccountOrderDetail, error) {
	var detail AccountOrderDetail
	err := aos.db.Operator.Core.NewSelect().Model(&detail).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select order detail by id, %d, error:%w", id, err)
	}
	return &detail, nil
}
