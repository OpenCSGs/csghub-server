package migrations

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type AccountMetering struct {
	ID           int64     `bun:",pk,autoincrement" json:"id"`
	EventUUID    uuid.UUID `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID     string    `bun:",notnull" json:"user_uuid"`
	Value        float64   `bun:",notnull" json:"value"`
	ValueType    int       `bun:",notnull" json:"value_type"`
	Scene        int       `bun:",notnull" json:"scene"`
	OpUID        string    `json:"op_uid"`
	ResourceID   string    `bun:",notnull" json:"resource_id"`
	ResourceName string    `bun:",notnull" json:"resource_name"`
	CustomerID   string    `json:"customer_id"`
	RecordedAt   time.Time `bun:",notnull" json:"recorded_at"`
	Extra        string    `json:"extra"`
	CreatedAt    time.Time `bun:",notnull,default:current_timestamp" json:"created_at"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountMetering{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountMetering{})
	})
}
