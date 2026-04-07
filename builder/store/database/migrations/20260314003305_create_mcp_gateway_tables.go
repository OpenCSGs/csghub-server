package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// GatewayMCPServers represents admin-managed MCP servers (table: gateway_mcp_servers)
type GatewayMCPServers struct {
	ID          int64          `bun:",pk,autoincrement" json:"id"`
	Name        string         `bun:",notnull,unique" json:"name"`
	Description string         `bun:",nullzero" json:"description"`
	Protocol    string         `bun:",notnull" json:"protocol"`
	URL         string         `bun:",notnull" json:"url"`
	Headers     map[string]any `bun:",type:jsonb,nullzero" json:"headers"`
	ConfigHash  string         `bun:",unique" json:"config_hash"`
	Env         map[string]any `bun:",type:jsonb,nullzero" json:"env"`
	Metadata    map[string]any `bun:",type:jsonb,nullzero" json:"metadata"`
	times
}

// GatewayMCPServerCapability represents cached Inspect results with TTL (one row per server)
type GatewayMCPServerCapability struct {
	ID            int64          `bun:",pk,autoincrement" json:"id"`
	MCPServerID   int64          `bun:",notnull,unique" json:"mcp_server_id"`
	MCPServerName string         `bun:",notnull" json:"mcp_server_name"`
	ConfigHash    string         `bun:",notnull" json:"config_hash"`
	Capabilities  map[string]any `bun:",type:jsonb,nullzero" json:"capabilities"`
	Status        string         `bun:",notnull" json:"status"`
	Error         string         `bun:",nullzero" json:"error"`
	RefreshedAt   time.Time      `bun:",notnull" json:"refreshed_at"`
	ExpiresAt     time.Time      `bun:",notnull" json:"expires_at"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, GatewayMCPServers{}, GatewayMCPServerCapability{})
		if err != nil {
			return fmt.Errorf("create table gateway_mcp_servers and gateway_mcp_server_capabilities fail: %w", err)
		}

		// create index on lower(name) for table gateway_mcp_servers (case-insensitive lookups)
		_, err = db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_gateway_mcp_servers_lower_name ON gateway_mcp_servers (lower(name))
		`)
		if err != nil {
			return fmt.Errorf("create index idx_gateway_mcp_servers_lower_name fail: %w", err)
		}

		// create unique index on (mcp_server_id, config_hash) for ON CONFLICT upserts
		_, err = db.NewCreateIndex().
			Model((*GatewayMCPServerCapability)(nil)).
			Index("uniq_gateway_mcp_server_capability_server_id_config_hash").
			Column("mcp_server_id", "config_hash").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create unique index uniq_gateway_mcp_server_capability_server_id_config_hash fail: %w", err)
		}

		// create index on mcp_server_name for table gateway_mcp_server_capabilities
		_, err = db.NewCreateIndex().
			Model((*GatewayMCPServerCapability)(nil)).
			Index("idx_gateway_mcp_server_capability_server_name").
			Column("mcp_server_name").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_gateway_mcp_server_capability_server_name fail: %w", err)
		}

		// create index on status for table gateway_mcp_server_capabilities
		_, err = db.NewCreateIndex().
			Model((*GatewayMCPServerCapability)(nil)).
			Index("idx_gateway_mcp_server_capability_status").
			Column("status").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_gateway_mcp_server_capability_status fail: %w", err)
		}

		// create index on refreshed_at for table gateway_mcp_server_capabilities
		_, err = db.NewCreateIndex().
			Model((*GatewayMCPServerCapability)(nil)).
			Index("idx_gateway_mcp_server_capability_refreshed_at").
			Column("refreshed_at").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_gateway_mcp_server_capability_refreshed_at fail: %w", err)
		}

		// create view gateway_mcp_servers_view (servers left join capability; no expires_at in view)
		// table names from bun: gateway_mcp_servers, gateway_mcp_server_capabilities
		_, err = db.ExecContext(ctx, `
			CREATE VIEW gateway_mcp_servers_view AS
			SELECT
				s.id,
				s.name,
				s.description,
				s.protocol,
				s.url,
				s.headers,
				s.env,
				s.metadata,
				s.created_at,
				s.updated_at,
				c.capabilities,
				c.status,
				c.error,
				c.refreshed_at
			FROM gateway_mcp_servers s
			LEFT JOIN gateway_mcp_server_capabilities c
				ON c.mcp_server_id = s.id AND c.config_hash = s.config_hash
		`)
		if err != nil {
			return fmt.Errorf("create view gateway_mcp_servers_view fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// drop view first (depends on tables)
		_, err := db.ExecContext(ctx, "DROP VIEW IF EXISTS gateway_mcp_servers_view CASCADE")
		if err != nil {
			return fmt.Errorf("drop view gateway_mcp_servers_view fail: %w", err)
		}
		err = dropTables(ctx, db, GatewayMCPServers{}, GatewayMCPServerCapability{})
		if err != nil {
			return fmt.Errorf("drop table gateway_mcp_servers and gateway_mcp_server_capabilities fail: %w", err)
		}
		return nil
	})
}
