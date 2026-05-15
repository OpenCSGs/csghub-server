package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type SkillVersion struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	SkillID     int64  `bun:",notnull" json:"skill_id"`
	Version     string `bun:",notnull" json:"version"`
	Hash        string `bun:"," json:"hash"`
	Changelog   string `bun:",type:text" json:"changelog"`
	License     string `bun:"," json:"license"`
	StoragePath string `bun:"," json:"storage_path"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, SkillVersion{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*SkillVersion)(nil)).
			Index("idx_unique_skill_versions_skill_id_version").
			Column("skill_id", "version").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_unique_skill_versions_skill_id_version fail: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, SkillVersion{})
	})
}
