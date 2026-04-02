package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type GatewayMCPServers struct {
	ID          int64          `bun:",pk,autoincrement" json:"id"`
	Name        string         `bun:",notnull,unique" json:"name"`
	Description string         `bun:",nullzero" json:"description"`
	Protocol    string         `bun:",notnull" json:"protocol"`
	URL         string         `bun:",notnull" json:"url"`
	Headers     map[string]any `bun:",type:jsonb,nullzero" json:"headers"`
	Env         map[string]any `bun:",type:jsonb,nullzero" json:"env"`
	Metadata    map[string]any `bun:",type:jsonb,nullzero" json:"metadata"`
	ConfigHash  string         `bun:",unique" json:"config_hash"`
	times
}

// GatewayMCPServerView is the row type for the gateway_mcp_servers_view view.
type GatewayMCPServerView struct {
	bun.BaseModel `bun:"table:gateway_mcp_servers_view"`
	ID            int64          `bun:"id,pk" json:"id"`
	Name          string         `bun:"name,notnull" json:"name"`
	Description   string         `bun:"description" json:"description"`
	Protocol      string         `bun:"protocol,notnull" json:"protocol"`
	URL           string         `bun:"url,notnull" json:"url"`
	Headers       map[string]any `bun:"headers,type:jsonb" json:"headers"`
	Env           map[string]any `bun:"env,type:jsonb" json:"env"`
	Metadata      map[string]any `bun:"metadata,type:jsonb" json:"metadata"`
	Capabilities  map[string]any `bun:"capabilities,type:jsonb" json:"capabilities"`
	Status        string         `bun:"status" json:"status"`
	Error         string         `bun:"error" json:"error"`
	RefreshedAt   *time.Time     `bun:"refreshed_at" json:"refreshed_at"`
	CreatedAt     time.Time      `bun:"created_at" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at" json:"updated_at"`
}

// GatewayMCPServersStore provides database operations for GatewayMCPServers
type GatewayMCPServersStore interface {
	Create(ctx context.Context, mcpServer *GatewayMCPServers) (*GatewayMCPServers, error)
	FindByID(ctx context.Context, id int64) (*GatewayMCPServerView, error)
	Update(ctx context.Context, mcpServer *GatewayMCPServers) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter types.GatewayMCPServerFilter, per int, page int) ([]GatewayMCPServerView, int, error)
	IsNameExists(ctx context.Context, name string) (bool, error)

	GetMCPServer(ctx context.Context, id int64) (*GatewayMCPServers, error)
	GetMCPServers(ctx context.Context) ([]GatewayMCPServers, error)
}

// gatewayMCPServersStoreImpl is the implementation of GatewayMCPServersStore
type gatewayMCPServersStoreImpl struct {
	db *DB
}

// NewGatewayMCPServersStore creates a new GatewayMCPServersStore
func NewGatewayMCPServersStore() GatewayMCPServersStore {
	return &gatewayMCPServersStoreImpl{
		db: defaultDB,
	}
}

// NewGatewayMCPServersStoreWithDB creates a new GatewayMCPServersStore with a specific DB
func NewGatewayMCPServersStoreWithDB(db *DB) GatewayMCPServersStore {
	return &gatewayMCPServersStoreImpl{
		db: db,
	}
}

// Create inserts a new GatewayMCPServers into the database
func (s *gatewayMCPServersStoreImpl) Create(ctx context.Context, mcpServer *GatewayMCPServers) (*GatewayMCPServers, error) {
	res, err := s.db.Core.NewInsert().Model(mcpServer).Exec(ctx, mcpServer)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"name": mcpServer.Name,
		})
	}
	return mcpServer, nil
}

// FindByID retrieves a gateway MCP server by its ID from the view.
func (s *gatewayMCPServersStoreImpl) FindByID(ctx context.Context, id int64) (*GatewayMCPServerView, error) {
	mcpServer := &GatewayMCPServerView{}
	err := s.db.Core.NewSelect().
		Model(mcpServer).
		Where("id = ?", id).
		Scan(ctx, mcpServer)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"id": id,
		})
	}
	return mcpServer, nil
}

// GetMCPServer retrieves a gateway MCP server by ID from the gateway_mcp_servers table.
func (s *gatewayMCPServersStoreImpl) GetMCPServer(ctx context.Context, id int64) (*GatewayMCPServers, error) {
	mcpServer := &GatewayMCPServers{}
	err := s.db.Core.NewSelect().
		Model(mcpServer).
		Where("id = ?", id).
		Scan(ctx, mcpServer)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"id": id,
		})
	}
	return mcpServer, nil
}

// Update updates the MCP server in the gateway_mcp_servers table.
func (s *gatewayMCPServersStoreImpl) Update(ctx context.Context, mcpServer *GatewayMCPServers) error {
	res, err := s.db.Core.NewUpdate().
		Model(mcpServer).
		WherePK().
		Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{"id": mcpServer.ID})
	}
	return nil
}

// Delete deletes a GatewayMCPServers and its capability cache row in one transaction.
func (s *gatewayMCPServersStoreImpl) Delete(ctx context.Context, id int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewDelete().
			Model((*GatewayMCPServerCapability)(nil)).
			Where("mcp_server_id = ?", id).
			Exec(ctx)
		if err != nil {
			return errorx.HandleDBError(err, map[string]any{"mcp_server_id": id})
		}
		res, err := tx.NewDelete().
			Model((*GatewayMCPServers)(nil)).
			Where("id = ?", id).
			Exec(ctx)
		if err = assertAffectedOneRow(res, err); err != nil {
			return errorx.HandleDBError(err, map[string]any{"id": id})
		}
		return nil
	})
}

// GetMCPServers returns all gateway MCP servers (for building the gateway; same list for all users)
func (s *gatewayMCPServersStoreImpl) GetMCPServers(ctx context.Context) ([]GatewayMCPServers, error) {
	var mcpServers []GatewayMCPServers
	err := s.db.Core.NewSelect().
		Model((*GatewayMCPServers)(nil)).
		OrderExpr("id ASC").
		Scan(ctx, &mcpServers)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{"operation": "get_all_mcp_servers"})
	}
	return mcpServers, nil
}

func (s *gatewayMCPServersStoreImpl) applyGatewayMCPServerFilters(query *bun.SelectQuery, filter types.GatewayMCPServerFilter) *bun.SelectQuery {
	if filter.Status != nil && *filter.Status != "" {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.Search != "" {
		filter.Search = strings.TrimSpace(filter.Search)
		if filter.Search != "" {
			if filter.ExactMatch {
				query = query.Where("LOWER(name) = LOWER(?) OR LOWER(description) = LOWER(?)", strings.ToLower(filter.Search), strings.ToLower(filter.Search))
			} else {
				searchPattern := "%" + filter.Search + "%"
				query = query.Where("LOWER(name) LIKE LOWER(?) OR LOWER(description) LIKE LOWER(?)", searchPattern, searchPattern)
			}
		}
	}
	return query
}

// List selects from gateway_mcp_servers_view with pagination and filtering.
func (s *gatewayMCPServersStoreImpl) List(ctx context.Context, filter types.GatewayMCPServerFilter, per int, page int) ([]GatewayMCPServerView, int, error) {
	var rows []GatewayMCPServerView
	viewQuery := s.db.Core.NewSelect().
		Table("gateway_mcp_servers_view")
	viewQuery = s.applyGatewayMCPServerFilters(viewQuery, filter)

	total, err := viewQuery.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{"operation": "count_gateway_mcp_servers_view"})
	}

	err = viewQuery.
		OrderExpr("updated_at DESC").
		Limit(per).
		Offset((page-1)*per).
		Scan(ctx, &rows)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{"operation": "list_gateway_mcp_servers_view"})
	}

	return rows, total, nil
}

// IsNameExists checks if an MCP server name already exists globally
func (s *gatewayMCPServersStoreImpl) IsNameExists(ctx context.Context, name string) (bool, error) {
	exists, err := s.db.Core.NewSelect().
		Model((*GatewayMCPServers)(nil)).
		Where("name = ?", name).
		Exists(ctx)
	if err != nil {
		return false, errorx.HandleDBError(err, map[string]any{"name": name})
	}
	return exists, nil
}
