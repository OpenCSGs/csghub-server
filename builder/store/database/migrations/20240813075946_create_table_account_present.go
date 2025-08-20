package migrations

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type AccountPresent struct {
	ID         int64     `bun:",pk,autoincrement" json:"id"`
	EventUUID  uuid.UUID `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID   string    `bun:",notnull" json:"user_uuid"`
	ActivityID int64     `bun:",notnull" json:"activity_id"`
	Value      float64   `bun:",notnull" json:"value"`
	OpUID      string    `bun:",notnull" json:"op_uid"`
	OpDesc     string    `bun:",notnull" json:"op_desc"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountPresent{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*AccountPresent)(nil)).
			Index("idx_account_present_useruuid_activityid").
			Column("user_uuid", "activity_id").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountPresent{})
	})
}
