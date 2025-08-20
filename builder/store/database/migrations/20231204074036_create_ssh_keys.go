package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type SSHKey struct {
	ID                int64  `bun:",pk,autoincrement" json:"id"`
	GitID             int64  `bun:",notnull" json:"git_id"`
	Name              string `bun:",notnull" json:"name"`
	Content           string `bun:",notnull" json:"content"`
	UserID            int64  `bun:",notnull" json:"user_id"`
	User              *User  `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	FingerprintSHA256 string `bun:"," json:"fingerprint_sha256"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, SSHKey{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, SSHKey{})
	})
}
