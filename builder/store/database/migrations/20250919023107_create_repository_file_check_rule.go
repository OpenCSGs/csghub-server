package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type RepositoryFileCheckRule struct {
	ID       int64  `bun:",pk,autoincrement"`
	RuleType string `bun:",notnull"`
	Pattern  string `bun:",notnull"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &RepositoryFileCheckRule{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model(&RepositoryFileCheckRule{}).
			Index("idx_repository_file_check_rule_rule_type_pattern").
			Column("rule_type", "pattern").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for repository_file_check_rule on rule_type/pattern")
		}
		namespaceWhiteList := []RepositoryFileCheckRule{
			{
				RuleType: "namespace",
				Pattern:  "OpenCSG",
			},
			{
				RuleType: "namespace",
				Pattern:  "deepseek-ai",
			},
			{
				RuleType: "namespace",
				Pattern:  "Qwen",
			},
			{
				RuleType: "namespace",
				Pattern:  "ByteDance",
			},
			{
				RuleType: "namespace",
				Pattern:  "TencentARC",
			},
		}
		return db.NewInsert().Model(&namespaceWhiteList).Scan(ctx)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &RepositoryFileCheckRule{})
	})
}
