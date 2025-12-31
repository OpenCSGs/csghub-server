package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/common/errorx"
)

type agentConfigStoreImpl struct {
	db *DB
}

type AgentConfigStore interface {
	GetByName(ctx context.Context, name string) (*AgentConfig, error)
	GetByID(ctx context.Context, id int64) (*AgentConfig, error)
	List(ctx context.Context) ([]*AgentConfig, error)
	Update(ctx context.Context, config *AgentConfig) error
	Create(ctx context.Context, config *AgentConfig) error
	Delete(ctx context.Context, id int64) error
	GetConfigValue(ctx context.Context, name string, key string) (any, error)
}

func NewAgentConfigStore() AgentConfigStore {
	return &agentConfigStoreImpl{
		db: defaultDB,
	}
}

func NewAgentConfigStoreWithDB(db *DB) AgentConfigStore {
	return &agentConfigStoreImpl{
		db: db,
	}
}

type AgentConfig struct {
	ID     int64          `bun:",pk,autoincrement" json:"id"`
	Name   string         `bun:",notnull,unique" json:"name"`
	Config map[string]any `bun:",type:jsonb,notnull" json:"config"`
	times
}

func (s *agentConfigStoreImpl) GetByName(ctx context.Context, name string) (*AgentConfig, error) {
	var config AgentConfig
	err := s.db.Operator.Core.NewSelect().
		Model(&config).
		Where("name = ?", name).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, map[string]any{"operation": "get_agent_config_by_name", "name": name})
	}
	return &config, nil
}

func (s *agentConfigStoreImpl) GetByID(ctx context.Context, id int64) (*AgentConfig, error) {
	config := AgentConfig{ID: id}
	err := s.db.Operator.Core.NewSelect().
		Model(&config).
		WherePK().
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, map[string]any{"operation": "get_agent_config_by_id", "id": id})
	}
	return &config, nil
}

func (s *agentConfigStoreImpl) List(ctx context.Context) ([]*AgentConfig, error) {
	var configs []*AgentConfig
	err := s.db.Operator.Core.NewSelect().
		Model(&configs).
		Order("updated_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{"operation": "list_agent_configs"})
	}
	return configs, nil
}

func (s *agentConfigStoreImpl) Update(ctx context.Context, config *AgentConfig) error {
	res, err := s.db.Operator.Core.NewUpdate().
		Model(config).
		Where("id = ?", config.ID).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{"operation": "update_agent_config"})
	}
	if err := assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{"operation": "update_agent_config"})
	}
	return nil
}

func (s *agentConfigStoreImpl) Create(ctx context.Context, config *AgentConfig) error {
	res, err := s.db.Operator.Core.NewInsert().
		Model(config).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create agent config: %w", err)
	}
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("failed to create agent config: %w", err)
	}
	return nil
}

func (s *agentConfigStoreImpl) Delete(ctx context.Context, id int64) error {
	res, err := s.db.Operator.Core.NewDelete().
		Model((*AgentConfig)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{"operation": "delete_agent_config", "id": id})
	}
	return assertAffectedOneRow(res, err)
}

func (s *agentConfigStoreImpl) GetConfigValue(ctx context.Context, name string, key string) (any, error) {
	config, err := s.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent config: %w", err)
	}
	if config == nil {
		return nil, fmt.Errorf("agent config not found")
	}

	value, ok := config.Config[key]
	if !ok {
		return nil, fmt.Errorf("config key %s not found", key)
	}

	return value, nil
}
