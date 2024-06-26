package migrations

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

type AccountStatement struct {
	ID        int64              `bun:",pk,autoincrement" json:"id"`
	EventUUID uuid.UUID          `bun:"type:uuid,notnull" json:"event_uuid"`
	UserID    string             `bun:",notnull" json:"user_id"`
	Value     float64            `bun:",notnull" json:"value"`
	Scene     database.SceneType `bun:",notnull" json:"scene"`
	OpUID     int64              `bun:",nullzero" json:"op_uid"`
	CreatedAt time.Time          `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountStatement{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountStatement{})
	})
}
