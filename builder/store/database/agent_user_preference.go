package database

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/common/errorx"
)

// AgentUserPreference represents a user's preference for an agent-related entity
type AgentUserPreference struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	UserUUID   string `bun:",notnull" json:"user_uuid"`
	EntityType string `bun:",notnull" json:"entity_type"`         // 'agent_instance', 'agent_knowledge_base', 'agent_template', etc.
	EntityID   string `bun:",notnull,type:text" json:"entity_id"` // TEXT to support both integer IDs (as strings) and string IDs
	Action     string `bun:",notnull" json:"action"`              // pin etc.
	times
}

// AgentUserPreferenceStore provides database operations for AgentUserPreference
type AgentUserPreferenceStore interface {
	Create(ctx context.Context, preference *AgentUserPreference) error
	Delete(ctx context.Context, userUUID string, entityType string, entityID string, action string) error
	FindByUserAndEntity(ctx context.Context, userUUID string, entityType string, entityID string, action string) (*AgentUserPreference, error)
	CountByUserAndType(ctx context.Context, userUUID string, entityType string, action string) (int, error)
	DeleteByEntity(ctx context.Context, entityType string, entityID string) error
	ListByUserAndType(ctx context.Context, userUUID string, entityType string, action string) ([]AgentUserPreference, error)
}

var _ AgentUserPreferenceStore = (*agentUserPreferenceStoreImpl)(nil)

// agentUserPreferenceStoreImpl is the implementation of AgentUserPreferenceStore
type agentUserPreferenceStoreImpl struct {
	db *DB
}

// NewAgentUserPreferenceStore creates a new AgentUserPreferenceStore
func NewAgentUserPreferenceStore() AgentUserPreferenceStore {
	return &agentUserPreferenceStoreImpl{
		db: defaultDB,
	}
}

// NewAgentUserPreferenceStoreWithDB creates a new AgentUserPreferenceStore with a specific DB
func NewAgentUserPreferenceStoreWithDB(db *DB) AgentUserPreferenceStore {
	return &agentUserPreferenceStoreImpl{
		db: db,
	}
}

// normalizeEntityID normalizes entity ID format for consistent storage
func normalizeEntityID(entityID string) string {
	entityID = strings.TrimSpace(entityID)
	// Try to parse as integer to normalize numeric IDs
	if id, err := strconv.ParseInt(entityID, 10, 64); err == nil {
		return strconv.FormatInt(id, 10)
	}
	// Return as-is for non-numeric IDs
	return entityID
}

// Create inserts a new AgentUserPreference into the database
func (s *agentUserPreferenceStoreImpl) Create(ctx context.Context, preference *AgentUserPreference) error {
	// Normalize entityID before storing
	preference.EntityID = normalizeEntityID(preference.EntityID)

	_, err := s.db.Core.NewInsert().
		Model(preference).
		On("CONFLICT (user_uuid, action, entity_type, entity_id) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"user_uuid":   preference.UserUUID,
			"entity_type": preference.EntityType,
			"entity_id":   preference.EntityID,
			"action":      preference.Action,
		})
	}
	return nil
}

// Delete removes an AgentUserPreference from the database
func (s *agentUserPreferenceStoreImpl) Delete(ctx context.Context, userUUID string, entityType string, entityID string, action string) error {
	// Normalize entityID before lookup
	normalizedID := normalizeEntityID(entityID)

	_, err := s.db.Core.NewDelete().
		Model((*AgentUserPreference)(nil)).
		Where("user_uuid = ?", userUUID).
		Where("action = ?", action).
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", normalizedID).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"user_uuid":   userUUID,
			"entity_type": entityType,
			"entity_id":   normalizedID,
			"action":      action,
		})
	}
	return nil
}

// FindByUserAndEntity finds a preference by user, entity type, entity ID, and action
func (s *agentUserPreferenceStoreImpl) FindByUserAndEntity(ctx context.Context, userUUID string, entityType string, entityID string, action string) (*AgentUserPreference, error) {
	normalizedID := normalizeEntityID(entityID)

	preference := &AgentUserPreference{}
	err := s.db.Core.NewSelect().
		Model(preference).
		Where("user_uuid = ?", userUUID).
		Where("action = ?", action).
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", normalizedID).
		Scan(ctx, preference)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorx.ErrNotFound
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid":   userUUID,
			"entity_type": entityType,
			"entity_id":   normalizedID,
			"action":      action,
		})
	}
	return preference, nil
}

// CountByUserAndType counts preferences for a user, entity type, and action
func (s *agentUserPreferenceStoreImpl) CountByUserAndType(ctx context.Context, userUUID string, entityType string, action string) (int, error) {
	count, err := s.db.Core.NewSelect().
		Model((*AgentUserPreference)(nil)).
		Where("user_uuid = ?", userUUID).
		Where("action = ?", action).
		Where("entity_type = ?", entityType).
		Count(ctx)
	if err != nil {
		return 0, errorx.HandleDBError(err, map[string]any{
			"user_uuid":   userUUID,
			"entity_type": entityType,
			"action":      action,
		})
	}
	return count, nil
}

// DeleteByEntity removes all preferences for a specific entity (used for auto-cleanup)
func (s *agentUserPreferenceStoreImpl) DeleteByEntity(ctx context.Context, entityType string, entityID string) error {
	// Normalize entityID before lookup
	normalizedID := normalizeEntityID(entityID)

	_, err := s.db.Core.NewDelete().
		Model((*AgentUserPreference)(nil)).
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", normalizedID).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"entity_type": entityType,
			"entity_id":   normalizedID,
		})
	}
	return nil
}

// ListByUserAndType lists all preferences for a user, entity type, and action, ordered by created_at DESC
func (s *agentUserPreferenceStoreImpl) ListByUserAndType(ctx context.Context, userUUID string, entityType string, action string) ([]AgentUserPreference, error) {
	var preferences []AgentUserPreference
	err := s.db.Core.NewSelect().
		Model(&preferences).
		Where("user_uuid = ?", userUUID).
		Where("action = ?", action).
		Where("entity_type = ?", entityType).
		OrderExpr("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid":   userUUID,
			"entity_type": entityType,
			"action":      action,
		})
	}
	return preferences, nil
}
