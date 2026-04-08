package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type AgentMCPServer struct {
	ID          int64          `bun:",pk,autoincrement" json:"id"`
	UserUUID    string         `bun:",notnull" json:"user_uuid"`
	Name        string         `bun:",notnull" json:"name"`
	Description string         `bun:",nullzero" json:"description"`
	Protocol    string         `bun:",notnull" json:"protocol"` // enum: streamable, sse
	URL         string         `bun:",notnull" json:"url"`
	Headers     map[string]any `bun:",type:jsonb,nullzero" json:"headers"`
	Env         map[string]any `bun:",type:jsonb,nullzero" json:"env"`
	times
}

type AgentMCPServerConfig struct {
	ID         int64          `bun:",pk,autoincrement" json:"id"`
	UserUUID   string         `bun:",notnull" json:"user_uuid"`
	ResourceID int64          `bun:",notnull" json:"resource_id"`
	Headers    map[string]any `bun:",type:jsonb,nullzero" json:"headers"`
	Env        map[string]any `bun:",type:jsonb,nullzero" json:"env"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AgentMCPServer{})
		if err != nil {
			return fmt.Errorf("create table agent_mcp_server fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentMCPServer)(nil)).
			Index("idx_agent_mcp_servers_user_uuid").
			Column("user_uuid").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_mcp_servers_user_uuid fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentMCPServer)(nil)).
			Index("idx_agent_mcp_servers_name").
			Column("name").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_mcp_servers_name fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentMCPServer)(nil)).
			Index("idx_agent_mcp_servers_updated_at").
			Column("updated_at").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_mcp_servers_updated_at fail: %w", err)
		}

		// Create agent_mcp_server_configs table
		err = createTables(ctx, db, AgentMCPServerConfig{})
		if err != nil {
			return fmt.Errorf("create table agent_mcp_server_configs fail: %w", err)
		}

		// Create unique constraint on (user_uuid, resource_id)
		_, err = db.ExecContext(ctx, `
			CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_mcp_server_configs_user_resource 
			ON agent_mcp_server_configs (user_uuid, resource_id);
		`)
		if err != nil {
			return fmt.Errorf("create unique index idx_agent_mcp_server_configs_user_resource fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentMCPServerConfig)(nil)).
			Index("idx_agent_mcp_server_configs_user_uuid").
			Column("user_uuid").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_mcp_server_configs_user_uuid fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentMCPServerConfig)(nil)).
			Index("idx_agent_mcp_server_configs_resource_id").
			Column("resource_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_mcp_server_configs_resource_id fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentMCPServerConfig)(nil)).
			Index("idx_agent_mcp_server_configs_updated_at").
			Column("updated_at").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_mcp_server_configs_updated_at fail: %w", err)
		}

		// Create view to join mcp_resources and agent_mcp_servers
		_, err = db.ExecContext(ctx, `
			DROP VIEW IF EXISTS agent_mcp_server_views;
		`)
		if err != nil {
			return fmt.Errorf("drop view agent_mcp_server_views fail: %w", err)
		}

		_, err = db.ExecContext(ctx, `
			CREATE VIEW agent_mcp_server_views AS
			SELECT 
				'builtin:' || mr.id::text AS id,
				mr.name,
				mr.description,
				'' AS user_uuid,
				mr.owner,
				mr.avatar,
				mr.protocol,
				mr.url,
				true AS built_in,
				CASE 
					WHEN amsc.resource_id IS NOT NULL THEN FALSE
					ELSE mr.need_install
				END AS need_install,
				mr.created_at,
				mr.updated_at
			FROM mcp_resources mr
			LEFT JOIN agent_mcp_server_configs amsc
				ON amsc.resource_id = mr.id
				AND amsc.user_uuid = current_setting('app.current_user')
			UNION ALL
			SELECT 
				'user:' || ams.id::text AS id,
				ams.name,
				ams.description,
				ams.user_uuid,
				COALESCE(u.username, '') AS owner,
				COALESCE(u.avatar, '') AS avatar,
				ams.protocol,
				ams.url,
				false AS built_in,
				false AS need_install,
				ams.created_at,
				ams.updated_at
			FROM agent_mcp_servers ams
			LEFT JOIN users u ON ams.user_uuid = u.uuid;
		`)
		if err != nil {
			return fmt.Errorf("create view agent_mcp_server_views fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Drop view first
		_, err := db.ExecContext(ctx, `
			DROP VIEW IF EXISTS agent_mcp_server_views;
		`)
		if err != nil {
			return fmt.Errorf("drop view agent_mcp_server_views fail: %w", err)
		}

		// Drop agent_mcp_server_configs table
		err = dropTables(ctx, db, AgentMCPServerConfig{})
		if err != nil {
			return fmt.Errorf("drop table agent_mcp_server_configs fail: %w", err)
		}

		// Then drop agent_mcp_servers table
		return dropTables(ctx, db, AgentMCPServer{})
	})
}
