package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type MCPResource struct {
	ID          int64          `bun:",pk,autoincrement" json:"id"`
	Name        string         `bun:",notnull" json:"name"`
	Description string         `bun:",notnull" json:"description"`
	Owner       string         `bun:",nullzero" json:"owner"`
	Avatar      string         `bun:",nullzero" json:"avatar"`
	Url         string         `bun:",notnull" json:"url"`
	Protocol    string         `bun:",notnull" json:"protocol"` // sse/streamable
	Headers     map[string]any `bun:"type:jsonb,nullzero" json:"headers"`
	NeedInstall bool           `bun:",notnull,default:false" json:"need_install"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &MCPResource{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &MCPResource{})
	})
}
