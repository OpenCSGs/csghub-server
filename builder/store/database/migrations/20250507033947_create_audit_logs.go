package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type AuditLog struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	TableName  string `bun:",notnull" json:"table_name"`
	Action     string `bun:",notnull" json:"action"`
	Operator   User   `bun:"rel:belongs-to,join:operator_id=id" json:"operator"`
	OperatorID int64  `bun:",notnull" json:"operator_id"`
	Before     string `bun:",nullzero" json:"before"`
	After      string `bun:",nullzero" json:"after"`

	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, &AuditLog{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &AuditLog{})
	})
}
