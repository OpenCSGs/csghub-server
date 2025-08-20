package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type UserResources struct {
	ID            int64     `bun:",pk,autoincrement" json:"id"`
	UserUID       string    `bun:",notnull" json:"user_uid"`
	OrderId       string    `bun:",notnull" json:"order_id"`
	OrderDetailId int64     `bun:",notnull,unique" json:"order_detail_id"`
	ResourceId    int64     `bun:",notnull" json:"resource_id"`
	DeployId      int64     `bun:",notnull" json:"deploy_id"`
	XPUNum        int       `bun:",notnull" json:"xpu_num"`
	PayMode       string    `bun:",notnull" json:"pay_mode"`
	Price         float64   `bun:",notnull" json:"price"`
	CreatedAt     time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
	StartTime     time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"start_time"`
	EndTime       time.Time `bun:",nullzero,notnull,skipupdate" json:"end_time"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, UserResources{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*UserResources)(nil)).
			Index("idx_user_resources_useruid").
			Column("user_uid", "order_detail_id", "end_time").
			Exec(ctx)
		return err

	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, UserResources{})
	})
}
