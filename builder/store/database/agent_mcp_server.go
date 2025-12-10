package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// AgentMCPServerView represents a row from the agent_mcp_server_views view
// This view combines built-in servers from mcp_resources and user-created servers from agent_mcp_servers
type AgentMCPServerView struct {
	ID          string `bun:",pk" json:"id"` // String ID (format: "builtin:{id}" or "user:{id}")
	Name        string `json:"name"`
	Description string `json:"description"`
	UserUUID    string `json:"user_uuid"`
	Owner       string `json:"owner"`
	Avatar      string `json:"avatar"`
	Protocol    string `json:"protocol"` // From mcp_resources or agent_mcp_servers
	URL         string `json:"url"`      // From mcp_resources or agent_mcp_servers
	BuiltIn     bool   `json:"built_in"`
	NeedInstall bool   `json:"need_install"`
	times
}

// AgentMCPServer represents an MCP server configuration for an agent
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

// AgentMCPServerConfig represents user override configurations for built-in MCP servers
type AgentMCPServerConfig struct {
	ID         int64          `bun:",pk,autoincrement" json:"id"`
	UserUUID   string         `bun:",notnull" json:"user_uuid"`
	ResourceID int64          `bun:",notnull" json:"resource_id"`
	Headers    map[string]any `bun:",type:jsonb,nullzero" json:"headers"`
	Env        map[string]any `bun:",type:jsonb,nullzero" json:"env"`
	times
}

// AgentMCPServerDetail represents a complete MCP server with all configuration details
type AgentMCPServerDetail struct {
	ID          string         `json:"id"` // String ID (format: "builtin:{id}" or "user:{id}")
	Name        string         `json:"name"`
	Description string         `json:"description"`
	UserUUID    string         `json:"user_uuid"`
	Owner       string         `json:"owner"`
	Avatar      string         `json:"avatar"`
	Protocol    string         `json:"protocol,omitempty"`
	URL         string         `json:"url,omitempty"`
	Headers     map[string]any `json:"headers,omitempty"`
	Env         map[string]any `json:"env,omitempty"`
	BuiltIn     bool           `json:"built_in"`
	NeedInstall bool           `json:"need_install"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// AgentMCPServerStore provides database operations for AgentMCPServer
type AgentMCPServerStore interface {
	Create(ctx context.Context, server *AgentMCPServer) (*AgentMCPServer, error)
	FindByID(ctx context.Context, userUUID string, id string) (*AgentMCPServerDetail, error)
	Find(ctx context.Context, id int64) (*AgentMCPServer, error)
	Update(ctx context.Context, server *AgentMCPServer) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter types.AgentMCPServerFilter, per int, page int) ([]AgentMCPServerView, int, error)
}

// AgentMCPServerConfigStore provides database operations for AgentMCPServerConfig
type AgentMCPServerConfigStore interface {
	Create(ctx context.Context, config *AgentMCPServerConfig) (*AgentMCPServerConfig, error)
	FindByUserUUIDAndResourceID(ctx context.Context, userUUID string, resourceID int64) (*AgentMCPServerConfig, error)
	Update(ctx context.Context, config *AgentMCPServerConfig) error
	Delete(ctx context.Context, id int64) error
}

// agentMCPServerStoreImpl is the implementation of AgentMCPServerStore
type agentMCPServerStoreImpl struct {
	db *DB
}

// agentMCPServerConfigStoreImpl is the implementation of AgentMCPServerConfigStore
type agentMCPServerConfigStoreImpl struct {
	db *DB
}

// NewAgentMCPServerStore creates a new AgentMCPServerStore
func NewAgentMCPServerStore() AgentMCPServerStore {
	return &agentMCPServerStoreImpl{
		db: defaultDB,
	}
}

// NewAgentMCPServerStoreWithDB creates a new AgentMCPServerStore with a specific DB
func NewAgentMCPServerStoreWithDB(db *DB) AgentMCPServerStore {
	return &agentMCPServerStoreImpl{
		db: db,
	}
}

// Create inserts a new AgentMCPServer into the database
func (s *agentMCPServerStoreImpl) Create(ctx context.Context, server *AgentMCPServer) (*AgentMCPServer, error) {
	res, err := s.db.Core.NewInsert().Model(server).Exec(ctx, server)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid": server.UserUUID,
			"name":      server.Name,
		})
	}
	return server, nil
}

// FindByID retrieves an AgentMCPServer by its string ID (format: "builtin:{id}" or "user:{id}")
// and returns the full configuration including headers and env
func (s *agentMCPServerStoreImpl) FindByID(ctx context.Context, userUUID string, id string) (*AgentMCPServerDetail, error) {
	if id == "" {
		return nil, fmt.Errorf("server ID cannot be empty")
	}

	// Parse the ID to determine if it's built-in or user-created
	var isBuiltIn bool
	var numericID int64
	var err error

	if strings.HasPrefix(id, types.AgentMCPServerIDPrefixBuiltin.String()) {
		isBuiltIn = true
		numericIDStr := strings.TrimPrefix(id, types.AgentMCPServerIDPrefixBuiltin.String())
		numericID, err = strconv.ParseInt(numericIDStr, 10, 64)
		if err != nil {
			return nil, errorx.HandleDBError(err, map[string]any{
				"server_id": id,
				"error":     "invalid built-in server ID format",
			})
		}
	} else if strings.HasPrefix(id, types.AgentMCPServerIDPrefixUser.String()) {
		isBuiltIn = false
		numericIDStr := strings.TrimPrefix(id, types.AgentMCPServerIDPrefixUser.String())
		numericID, err = strconv.ParseInt(numericIDStr, 10, 64)
		if err != nil {
			return nil, errorx.HandleDBError(err, map[string]any{
				"server_id": id,
				"error":     "invalid user server ID format",
			})
		}
	} else {
		return nil, fmt.Errorf("invalid server ID format: expected 'builtin:{id}' or 'user:{id}', got %s", id)
	}

	if isBuiltIn {
		// Query built-in server from mcp_resources
		var detail AgentMCPServerDetail
		err = s.db.Core.NewSelect().
			TableExpr("mcp_resources").
			ColumnExpr("'"+types.AgentMCPServerIDPrefixBuiltin.String()+"' || id::text AS id").
			ColumnExpr("name").
			ColumnExpr("description").
			ColumnExpr("'' AS user_uuid").
			ColumnExpr("owner").
			ColumnExpr("avatar").
			ColumnExpr("protocol").
			ColumnExpr("url").
			ColumnExpr("headers").
			ColumnExpr("NULL::jsonb AS env").
			ColumnExpr("true AS built_in").
			ColumnExpr("need_install").
			ColumnExpr("created_at").
			ColumnExpr("updated_at").
			Where("id = ?", numericID).
			Scan(ctx, &detail)
		if err != nil {
			return nil, errorx.HandleDBError(err, map[string]any{
				"server_id":   id,
				"resource_id": numericID,
			})
		}

		// Fetch user's override config if it exists
		configStore := NewAgentMCPServerConfigStoreWithDB(s.db)
		config, err := configStore.FindByUserUUIDAndResourceID(ctx, userUUID, numericID)
		// If config exists, merge it with defaults
		if err == nil && config != nil {
			// Merge headers: default + override (override takes precedence)
			if detail.Headers == nil {
				detail.Headers = make(map[string]any)
			}
			if config.Headers != nil {
				for k, v := range config.Headers {
					detail.Headers[k] = v
				}
			}
			// Override env if present
			if config.Env != nil {
				detail.Env = config.Env
			}
			detail.NeedInstall = false
		}

		return &detail, nil
	} else {
		// Query user-created server from agent_mcp_servers
		var detail AgentMCPServerDetail
		err = s.db.Core.NewSelect().
			TableExpr("agent_mcp_servers ams").
			ColumnExpr("'"+types.AgentMCPServerIDPrefixUser.String()+"' || ams.id::text AS id").
			ColumnExpr("ams.name").
			ColumnExpr("ams.description").
			ColumnExpr("ams.user_uuid").
			ColumnExpr("COALESCE(u.username, '') AS owner").
			ColumnExpr("COALESCE(u.avatar, '') AS avatar").
			ColumnExpr("ams.protocol").
			ColumnExpr("ams.url").
			ColumnExpr("ams.headers").
			ColumnExpr("ams.env").
			ColumnExpr("false AS built_in").
			ColumnExpr("false AS need_install").
			ColumnExpr("ams.created_at").
			ColumnExpr("ams.updated_at").
			Join("LEFT JOIN users u ON ams.user_uuid = u.uuid").
			Where("ams.id = ?", numericID).
			Scan(ctx, &detail)
		if err != nil {
			return nil, errorx.HandleDBError(err, map[string]any{
				"server_id":      id,
				"user_server_id": numericID,
			})
		}

		return &detail, nil
	}
}

// Find retrieves an AgentMCPServer by its numeric ID
func (s *agentMCPServerStoreImpl) Find(ctx context.Context, id int64) (*AgentMCPServer, error) {
	server := &AgentMCPServer{}
	err := s.db.Core.NewSelect().
		Model(server).
		Where("id = ?", id).
		Scan(ctx, server)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"server_id": id,
		})
	}
	return server, nil
}

// Update updates an existing AgentMCPServer
func (s *agentMCPServerStoreImpl) Update(ctx context.Context, server *AgentMCPServer) error {
	res, err := s.db.Core.NewUpdate().Model(server).Where("id = ?", server.ID).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"server_id": server.ID,
		})
	}
	return nil
}

// Delete soft-deletes an AgentMCPServer
func (s *agentMCPServerStoreImpl) Delete(ctx context.Context, id int64) error {
	res, err := s.db.Core.NewDelete().Model((*AgentMCPServer)(nil)).Where("id = ?", id).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"server_id": id,
		})
	}
	return nil
}

// applyAgentMCPServerFilters applies filters to the query for the union view
func (s *agentMCPServerStoreImpl) applyAgentMCPServerFilters(query *bun.SelectQuery, filter types.AgentMCPServerFilter) *bun.SelectQuery {
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("LOWER(name) LIKE LOWER(?)", searchPattern)
	}

	// Handle built_in filter
	if filter.BuiltIn != nil {
		if *filter.BuiltIn {
			// If built_in is true, ignore user_uuid filter and show only built-in servers
			query = query.Where("built_in = ?", true)
		} else {
			// If built_in is false, show only user's servers (filter by user_uuid)
			query = query.Where("built_in = ? AND user_uuid = ?", false, filter.UserUUID)
		}
	} else {
		// Handle user_uuid filter: show both built-in and user's servers (only when built_in is not set)
		if filter.UserUUID != "" {
			query = query.Where("built_in = ? OR user_uuid = ?", true, filter.UserUUID)
		}
	}

	// Handle protocol filter
	if filter.Protocol != nil {
		query = query.Where("protocol = ?", *filter.Protocol)
	}

	// Handle need_install filter
	if filter.NeedInstall != nil {
		query = query.Where("need_install = ?", *filter.NeedInstall)
	}

	return query
}

// List retrieves AgentMCPServers from the agent_mcp_server_views view with filtering and pagination
func (s *agentMCPServerStoreImpl) List(ctx context.Context, filter types.AgentMCPServerFilter, per int, page int) ([]AgentMCPServerView, int, error) {
	var servers []AgentMCPServerView
	var total int

	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Set context variable in transaction
		_, err := tx.ExecContext(ctx, `SET LOCAL "app.current_user" = ?`, filter.UserUUID)
		if err != nil {
			return errorx.HandleDBError(err, map[string]any{
				"operation": "set_current_user",
			})
		}

		// Build query
		query := tx.NewSelect().
			TableExpr("agent_mcp_server_views").
			ColumnExpr("*")

		query = s.applyAgentMCPServerFilters(query, filter)

		// Get total count
		total, err = query.Count(ctx)
		if err != nil {
			return errorx.HandleDBError(err, map[string]any{
				"operation": "count_agent_mcp_servers",
			})
		}

		// Get paginated results
		err = query.Order("updated_at DESC").Limit(per).Offset((page-1)*per).Scan(ctx, &servers)
		if err != nil {
			return errorx.HandleDBError(err, map[string]any{
				"operation": "list_agent_mcp_servers",
			})
		}

		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	return servers, total, nil
}

// NewAgentMCPServerConfigStore creates a new AgentMCPServerConfigStore
func NewAgentMCPServerConfigStore() AgentMCPServerConfigStore {
	return &agentMCPServerConfigStoreImpl{
		db: defaultDB,
	}
}

// NewAgentMCPServerConfigStoreWithDB creates a new AgentMCPServerConfigStore with a specific DB
func NewAgentMCPServerConfigStoreWithDB(db *DB) AgentMCPServerConfigStore {
	return &agentMCPServerConfigStoreImpl{
		db: db,
	}
}

// Create inserts a new AgentMCPServerConfig into the database
func (s *agentMCPServerConfigStoreImpl) Create(ctx context.Context, config *AgentMCPServerConfig) (*AgentMCPServerConfig, error) {
	res, err := s.db.Core.NewInsert().Model(config).Exec(ctx, config)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid":   config.UserUUID,
			"resource_id": config.ResourceID,
		})
	}
	return config, nil
}

// FindByUserUUIDAndResourceID retrieves an AgentMCPServerConfig by user UUID and resource ID
func (s *agentMCPServerConfigStoreImpl) FindByUserUUIDAndResourceID(ctx context.Context, userUUID string, resourceID int64) (*AgentMCPServerConfig, error) {
	config := &AgentMCPServerConfig{}
	err := s.db.Core.NewSelect().
		Model(config).
		Where("user_uuid = ? AND resource_id = ?", userUUID, resourceID).
		Scan(ctx, config)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid":   userUUID,
			"resource_id": resourceID,
		})
	}
	return config, nil
}

// Update updates an existing AgentMCPServerConfig
func (s *agentMCPServerConfigStoreImpl) Update(ctx context.Context, config *AgentMCPServerConfig) error {
	res, err := s.db.Core.NewUpdate().
		Model(config).
		Where("id = ?", config.ID).
		Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"config_id": config.ID,
		})
	}
	return nil
}

// Delete deletes an AgentMCPServerConfig
func (s *agentMCPServerConfigStoreImpl) Delete(ctx context.Context, id int64) error {
	res, err := s.db.Core.NewDelete().Model((*AgentMCPServerConfig)(nil)).Where("id = ?", id).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"config_id": id,
		})
	}
	return nil
}
