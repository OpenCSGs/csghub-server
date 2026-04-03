package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type AgentMCPServerStatus struct {
	ID              int64          `bun:",pk,autoincrement" json:"id"`
	ServerID        string         `bun:",notnull" json:"server_id"` // "builtin:{id}" or "user:{id}"
	UserUUID        string         `bun:",notnull,default:''" json:"user_uuid"`
	Status          string         `bun:",notnull" json:"status"` // 'connected', 'error'
	Error           string         `bun:",nullzero" json:"error"` // error message when inspection failed
	Capabilities    map[string]any `bun:",type:jsonb,nullzero" json:"capabilities"`
	LastInspectedAt time.Time      `bun:",nullzero" json:"last_inspected_at"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AgentMCPServerStatus{})
		if err != nil {
			return fmt.Errorf("create table agent_mcp_server_statuses fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentMCPServerStatus)(nil)).
			Index("idx_agent_mcp_server_statuses_server_id_user").
			Column("server_id", "user_uuid").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_mcp_server_statuses_server_id_user fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentMCPServerStatus)(nil)).
			Index("idx_agent_mcp_server_statuses_status").
			Column("status").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_mcp_server_statuses_status fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentMCPServerStatus)(nil)).
			Index("idx_agent_mcp_server_statuses_last_inspected_at").
			Column("last_inspected_at").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_mcp_server_statuses_last_inspected_at fail: %w", err)
		}

		// update view agent_mcp_server_views to add `installed` column
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
				mr.need_install AS need_install,
				CASE
					WHEN mr.need_install = FALSE THEN TRUE
					WHEN amsc.resource_id IS NOT NULL THEN TRUE
					ELSE FALSE
				END AS installed,
				CASE
					WHEN mr.need_install = FALSE OR amsc.resource_id IS NOT NULL THEN COALESCE(amss.status, '')
					ELSE ''
				END AS status,
				CASE
					WHEN mr.need_install = FALSE OR amsc.resource_id IS NOT NULL THEN amss.capabilities
					ELSE NULL
				END AS capabilities,
				mr.created_at,
				mr.updated_at
			FROM mcp_resources mr
			LEFT JOIN agent_mcp_server_configs amsc
				ON amsc.resource_id = mr.id
				AND amsc.user_uuid = current_setting('app.current_user')
			LEFT JOIN agent_mcp_server_statuses amss
				ON amss.server_id = 'builtin:' || mr.id::text
				AND amss.user_uuid = CASE
					WHEN mr.need_install = FALSE THEN ''
					ELSE current_setting('app.current_user')
				END
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
				true AS installed,
				COALESCE(amss.status, '') AS status,
				amss.capabilities,
				ams.created_at,
				ams.updated_at
			FROM agent_mcp_servers ams
			LEFT JOIN users u ON ams.user_uuid = u.uuid
			LEFT JOIN agent_mcp_server_statuses amss
				ON amss.server_id = 'user:' || ams.id::text
				AND amss.user_uuid = ams.user_uuid;
		`)
		if err != nil {
			return fmt.Errorf("create view agent_mcp_server_views fail: %w", err)
		}

		_, err = db.ExecContext(ctx, `
			DROP VIEW IF EXISTS agent_mcp_server_all_views;
		`)
		if err != nil {
			return fmt.Errorf("drop view agent_mcp_server_all_views fail: %w", err)
		}

		// agent_mcp_server_all_views is NOT user-specific. It lists all MCP servers across:
		// 1) built-in no-need-install (mcp_resources.need_install=false)
		// 2) built-in need-install with user overrides (agent_mcp_server_configs + mcp_resources)
		// 3) user-added (agent_mcp_servers)
		_, err = db.ExecContext(ctx, `
			CREATE VIEW agent_mcp_server_all_views AS
			-- built-in, no need_install (global)
			SELECT
				'builtin:' || mr.id::text AS id,
				mr.name,
				mr.description,
				'' AS user_uuid,
				mr.protocol,
				mr.url,
				mr.headers,
				true AS built_in,
				mr.need_install AS need_install,
				true AS installed,
				mr.created_at,
				mr.updated_at
			FROM mcp_resources mr
			WHERE mr.need_install = FALSE
			UNION ALL
			-- built-in, need_install (installed per user config)
			SELECT
				'builtin:' || mr.id::text AS id,
				mr.name,
				mr.description,
				amsc.user_uuid AS user_uuid,
				mr.protocol,
				mr.url,
				COALESCE(amsc.headers, mr.headers) AS headers,
				true AS built_in,
				mr.need_install AS need_install,
				true AS installed,
				mr.created_at,
				mr.updated_at
			FROM agent_mcp_server_configs amsc
			JOIN mcp_resources mr
				ON mr.id = amsc.resource_id
			WHERE mr.need_install = TRUE
			UNION ALL
			-- user-added (always installed)
			SELECT
				'user:' || ams.id::text AS id,
				ams.name,
				ams.description,
				ams.user_uuid,
				ams.protocol,
				ams.url,
				ams.headers,
				false AS built_in,
				false AS need_install,
				true AS installed,
				ams.created_at,
				ams.updated_at
			FROM agent_mcp_servers ams;
		`)
		if err != nil {
			return fmt.Errorf("create view agent_mcp_server_all_views fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
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

		_, err = db.ExecContext(ctx, `
			DROP VIEW IF EXISTS agent_mcp_server_all_views;
		`)
		if err != nil {
			return fmt.Errorf("drop view agent_mcp_server_all_views fail: %w", err)
		}

		return dropTables(ctx, db, AgentMCPServerStatus{})
	})
}
