package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type GitServerType string

const (
	MirrorServer GitServerType = "mirror"
	GitServer    GitServerType = "git"
)

type GitServerAccessToken struct {
	ID         int64         `bun:",pk,autoincrement" json:"id"`
	Token      string        `bun:",notnull" json:"token"`
	ServerType GitServerType `bun:",notnull" json:"server_type"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, GitServerAccessToken{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, GitServerAccessToken{})
	})
}
