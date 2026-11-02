package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"opencsg.com/csghub-server/common/errorx"
)

// GatewayMCPServerCapability represents cached Inspect results with TTL
type GatewayMCPServerCapability struct {
	ID              int64          `bun:",pk,autoincrement" json:"id"`
	MCPServerID     int64          `bun:",notnull" json:"mcp_server_id"`
	MCPServerName   string         `bun:",notnull" json:"mcp_server_name"`
	ConfigHash      string         `bun:",notnull" json:"config_hash"`
	Capabilities    map[string]any `bun:",type:jsonb,nullzero" json:"capabilities"`
	Status          string         `bun:",notnull" json:"status"` // "ok" | "error"
	Error           string         `bun:",nullzero" json:"error"`
	ProtocolVersion string         `bun:",notnull,default:''" json:"protocol_version"`
	RefreshedAt     time.Time      `bun:",notnull" json:"refreshed_at"`
	ExpiresAt       time.Time      `bun:",notnull" json:"expires_at"`
	times
}

// GatewayMCPServerCapabilityRevision is the fingerprint-relevant state of a
// capability row, used by the shared gateway rebuild fingerprint. ExpiresAt
// drives availability decisions only; it is excluded from the fingerprint.
type GatewayMCPServerCapabilityRevision struct {
	MCPServerID     int64          `json:"mcp_server_id"`
	ConfigHash      string         `json:"config_hash"`
	Status          string         `json:"status"`
	ProtocolVersion string         `json:"protocol_version"`
	Capabilities    map[string]any `json:"capabilities"`
	ExpiresAt       time.Time      `json:"expires_at"`
}

// GatewayMCPServerCapabilityStore provides database operations for GatewayMCPServerCapability
type GatewayMCPServerCapabilityStore interface {
	CreateOrUpdate(ctx context.Context, cap *GatewayMCPServerCapability) error
	UpsertError(ctx context.Context, cap *GatewayMCPServerCapability) error
	FindByServerAndConfig(ctx context.Context, mcpServerID int64, configHash string) (*GatewayMCPServerCapability, error)
	ListRevisions(ctx context.Context) ([]GatewayMCPServerCapabilityRevision, error)
	DeleteByServer(ctx context.Context, mcpServerID int64) error
	DeleteByServerAndConfig(ctx context.Context, mcpServerID int64, configHash string) error
	DeleteExpired(ctx context.Context) error
	DeleteAllForUserBackend(ctx context.Context, backendID int64) error
}

// gatewayMCPServerCapabilityStoreImpl is the implementation of GatewayMCPServerCapabilityStore
type gatewayMCPServerCapabilityStoreImpl struct {
	db *DB
}

// NewGatewayMCPServerCapabilityStore creates a new GatewayMCPServerCapabilityStore
func NewGatewayMCPServerCapabilityStore() GatewayMCPServerCapabilityStore {
	return &gatewayMCPServerCapabilityStoreImpl{
		db: defaultDB,
	}
}

// NewGatewayMCPServerCapabilityStoreWithDB creates a new GatewayMCPServerCapabilityStore with a specific DB
func NewGatewayMCPServerCapabilityStoreWithDB(db *DB) GatewayMCPServerCapabilityStore {
	return &gatewayMCPServerCapabilityStoreImpl{
		db: db,
	}
}

// CreateOrUpdate upserts a GatewayMCPServerCapability entry
func (s *gatewayMCPServerCapabilityStoreImpl) CreateOrUpdate(ctx context.Context, cap *GatewayMCPServerCapability) error {
	cap.UpdatedAt = time.Now()
	_, err := s.db.Core.NewInsert().
		Model(cap).
		On("CONFLICT (mcp_server_id, config_hash) DO UPDATE").
		Set("mcp_server_name = EXCLUDED.mcp_server_name").
		Set("capabilities = EXCLUDED.capabilities").
		Set("status = EXCLUDED.status").
		Set("error = EXCLUDED.error").
		Set("protocol_version = EXCLUDED.protocol_version").
		Set("refreshed_at = EXCLUDED.refreshed_at").
		Set("expires_at = EXCLUDED.expires_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"mcp_server_id":   cap.MCPServerID,
			"mcp_server_name": cap.MCPServerName,
			"config_hash":     cap.ConfigHash,
		})
	}
	return nil
}

// UpsertError records a failure without overwriting the last known catalog or revision.
func (s *gatewayMCPServerCapabilityStoreImpl) UpsertError(ctx context.Context, cap *GatewayMCPServerCapability) error {
	cap.UpdatedAt = time.Now()
	cap.Status = "error"
	cap.Capabilities = nil
	cap.ProtocolVersion = ""
	_, err := s.db.Core.NewInsert().Model(cap).
		On("CONFLICT (mcp_server_id, config_hash) DO UPDATE").
		Set("mcp_server_name = EXCLUDED.mcp_server_name").
		Set("status = EXCLUDED.status").Set("error = EXCLUDED.error").
		Set("refreshed_at = EXCLUDED.refreshed_at").Set("expires_at = EXCLUDED.expires_at").
		Set("updated_at = EXCLUDED.updated_at").Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{"mcp_server_id": cap.MCPServerID, "config_hash": cap.ConfigHash})
	}
	return nil
}

// FindByServerAndConfig retrieves a capability entry by MCP server ID and config hash
func (s *gatewayMCPServerCapabilityStoreImpl) FindByServerAndConfig(ctx context.Context, mcpServerID int64, configHash string) (*GatewayMCPServerCapability, error) {
	cap := &GatewayMCPServerCapability{}
	err := s.db.Core.NewSelect().
		Model(cap).
		Where("mcp_server_id = ? AND config_hash = ?", mcpServerID, configHash).
		Scan(ctx, cap)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"mcp_server_id": mcpServerID,
			"config_hash":   configHash,
		})
	}
	return cap, nil
}

// ListRevisions returns the fingerprint-relevant columns of all capability rows.
func (s *gatewayMCPServerCapabilityStoreImpl) ListRevisions(ctx context.Context) ([]GatewayMCPServerCapabilityRevision, error) {
	var revisions []GatewayMCPServerCapabilityRevision
	err := s.db.Core.NewSelect().
		Model((*GatewayMCPServerCapability)(nil)).
		Column("mcp_server_id", "config_hash", "status", "protocol_version", "capabilities", "expires_at").
		Scan(ctx, &revisions)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"operation": "list_capability_revisions",
		})
	}
	return revisions, nil
}

// DeleteByServer deletes all capability entries for a specific MCP server
func (s *gatewayMCPServerCapabilityStoreImpl) DeleteByServer(ctx context.Context, mcpServerID int64) error {
	_, err := s.db.Core.NewDelete().
		Model((*GatewayMCPServerCapability)(nil)).
		Where("mcp_server_id = ?", mcpServerID).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"mcp_server_id": mcpServerID,
		})
	}
	return nil
}

// DeleteByServerAndConfig deletes a capability entry for a specific MCP server and config
func (s *gatewayMCPServerCapabilityStoreImpl) DeleteByServerAndConfig(ctx context.Context, mcpServerID int64, configHash string) error {
	res, err := s.db.Core.NewDelete().
		Model((*GatewayMCPServerCapability)(nil)).
		Where("mcp_server_id = ? AND config_hash = ?", mcpServerID, configHash).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"mcp_server_id": mcpServerID,
			"config_hash":   configHash,
		})
	}
	_ = res
	return nil
}

// DeleteExpired deletes all expired capability entries
func (s *gatewayMCPServerCapabilityStoreImpl) DeleteExpired(ctx context.Context) error {
	_, err := s.db.Core.NewDelete().
		Model((*GatewayMCPServerCapability)(nil)).
		Where("expires_at < ?", time.Now()).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"operation": "delete_expired_capabilities",
		})
	}
	return nil
}

// DeleteAllForUserBackend deletes all capability entries for a user's backend.
// This is used when a user backend is created, updated, or deleted.
func (s *gatewayMCPServerCapabilityStoreImpl) DeleteAllForUserBackend(ctx context.Context, backendID int64) error {
	return s.DeleteByServer(ctx, backendID)
}
