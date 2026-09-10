package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccountStatistics struct {
	ID                int64           `bun:",pk,autoincrement" json:"id"`
	EventDate         time.Time       `bun:"type:date" json:"event_date"`
	UserUUID          string          `bun:",notnull" json:"user_uuid"`
	Scene             types.SceneType `bun:",notnull" json:"scene"`
	CustomerID        string          `bun:",notnull" json:"customer_id"`
	Consumption       float64         `bun:",notnull" json:"consumption"`
	PromptToken       float64         `bun:",notnull" json:"prompt_token"`
	PromptCachedToken float64         `bun:",notnull" json:"prompt_cached_token"`
	CompletionToken   float64         `bun:",notnull" json:"completion_token"`
	Count             float64         `bun:",notnull,default:0" json:"count"`
	TokenID           int64           `bun:",notnull,default:0" json:"token_id"`
	DataType          string          `bun:",notnull,default:''" json:"data_type"`
	Resolution        string          `bun:",notnull,default:''" json:"resolution"`
	Duration          float64         `bun:",notnull,default:0" json:"duration"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, (*AccountStatistics)(nil)); err != nil {
			return fmt.Errorf("create table account_statistics error: %w", err)
		}
		_, err := db.NewCreateIndex().
			Model((*AccountStatistics)(nil)).
			Index("idx_unique_account_statistics").
			Unique().
			Column("event_date", "user_uuid", "scene", "customer_id", "token_id", "data_type", "resolution").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create unique index error: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, (*AccountStatistics)(nil))
	})
}
