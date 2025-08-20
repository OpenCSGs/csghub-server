package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type MCPScanResult struct {
	ID           int64  `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64  `bun:",notnull" json:"repository_id"`
	FilePath     string `bun:",notnull" json:"file_path"`
	CommitID     string `bun:",notnull" json:"commit_id"`
	Title        string `bun:",notnull" json:"title"`
	RiskLevel    string `bun:",notnull" json:"risk_level"`
	Detail       string `bun:",notnull" json:"detail"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, MCPScanResult{})
		if err != nil {
			return fmt.Errorf("create table mcp scan result fail: %w", err)
		}
		_, err = db.NewCreateIndex().
			Model((*MCPScanResult)(nil)).
			Index("idx_mcp_scan_result_repo_id").
			Column("repository_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_mcp_scan_result_repo_id fail: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, MCPScanResult{})
	})
}
