package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type MCPServer struct {
	ID            int64  `bun:",pk,autoincrement" json:"id"`
	RepositoryID  int64  `bun:",notnull" json:"repository_id"`
	ToolsNum      int    `bun:",nullzero" json:"tools_num"`
	Configuration string `bun:",nullzero" json:"configuration"` // server configuration json string
	Schema        string `bun:",nullzero" json:"schema"`        // all properties json string
	times
}

type MCPServerProperty struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	MCPServerID int64  `bun:",notnull" json:"mcp_server_id"`
	Kind        string `bun:",notnull" json:"kind"` // tool, prompt, resource, resource_template
	Name        string `bun:",notnull" json:"name"`
	Description string `bun:",nullzero" json:"description"`
	Schema      string `bun:",nullzero" json:"schema"` // single property json string
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, MCPServer{}, MCPServerProperty{})
		if err != nil {
			return fmt.Errorf("create table mcp server and property fail: %w", err)
		}
		_, err = db.NewCreateIndex().
			Model((*MCPServer)(nil)).
			Index("idx_unique_mcp_server_repo_id").
			Column("repository_id").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_unique_mcp_server_repo_id fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*MCPServerProperty)(nil)).
			Index("idx_mcp_server_property_mcpserverid_kind").
			Column("kind", "mcp_server_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_mcp_server_property_mcpserverid_kind fail: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, MCPServer{}, MCPServerProperty{})
	})
}
