package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type AgentConfigComponent interface {
	GetByName(ctx context.Context, name string) (*types.AgentConfig, error)
	GetByID(ctx context.Context, id int64) (*types.AgentConfig, error)
	List(ctx context.Context) ([]*types.AgentConfig, error)
	Create(ctx context.Context, req *types.CreateAgentConfigReq) (*types.AgentConfig, error)
	Update(ctx context.Context, id int64, req *types.UpdateAgentConfigReq) (*types.AgentConfig, error)
	Delete(ctx context.Context, id int64) error
}

type agentConfigComponentImpl struct {
	agentConfigStore database.AgentConfigStore
}

func NewAgentConfigComponent(config *config.Config) (AgentConfigComponent, error) {
	agentConfigStore := database.NewAgentConfigStore()
	return &agentConfigComponentImpl{
		agentConfigStore: agentConfigStore,
	}, nil
}

func (c *agentConfigComponentImpl) GetByName(ctx context.Context, name string) (*types.AgentConfig, error) {
	dbConfig, err := c.agentConfigStore.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent config by name: %w", err)
	}
	if dbConfig == nil {
		return nil, fmt.Errorf("agent config not found")
	}

	return &types.AgentConfig{
		ID:        dbConfig.ID,
		Name:      dbConfig.Name,
		Config:    dbConfig.Config,
		CreatedAt: dbConfig.CreatedAt,
		UpdatedAt: dbConfig.UpdatedAt,
	}, nil
}

func (c *agentConfigComponentImpl) GetByID(ctx context.Context, id int64) (*types.AgentConfig, error) {
	dbConfig, err := c.agentConfigStore.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent config by id: %w", err)
	}
	if dbConfig == nil {
		return nil, fmt.Errorf("agent config not found")
	}

	return &types.AgentConfig{
		ID:        dbConfig.ID,
		Name:      dbConfig.Name,
		Config:    dbConfig.Config,
		CreatedAt: dbConfig.CreatedAt,
		UpdatedAt: dbConfig.UpdatedAt,
	}, nil
}

func (c *agentConfigComponentImpl) List(ctx context.Context) ([]*types.AgentConfig, error) {
	dbConfigs, err := c.agentConfigStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent configs: %w", err)
	}

	configs := make([]*types.AgentConfig, len(dbConfigs))
	for i, dbConfig := range dbConfigs {
		configs[i] = &types.AgentConfig{
			ID:        dbConfig.ID,
			Name:      dbConfig.Name,
			Config:    dbConfig.Config,
			CreatedAt: dbConfig.CreatedAt,
			UpdatedAt: dbConfig.UpdatedAt,
		}
	}

	return configs, nil
}

func (c *agentConfigComponentImpl) Create(ctx context.Context, req *types.CreateAgentConfigReq) (*types.AgentConfig, error) {
	dbConfig := &database.AgentConfig{
		Name:   req.Name,
		Config: req.Config,
	}

	err := c.agentConfigStore.Create(ctx, dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent config: %w", err)
	}

	return &types.AgentConfig{
		ID:        dbConfig.ID,
		Name:      dbConfig.Name,
		Config:    dbConfig.Config,
		CreatedAt: dbConfig.CreatedAt,
		UpdatedAt: dbConfig.UpdatedAt,
	}, nil
}

func (c *agentConfigComponentImpl) Delete(ctx context.Context, id int64) error {
	err := c.agentConfigStore.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent config: %w", err)
	}
	return nil
}

func (c *agentConfigComponentImpl) Update(ctx context.Context, id int64, req *types.UpdateAgentConfigReq) (*types.AgentConfig, error) {
	dbConfig, err := c.agentConfigStore.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent config: %w", err)
	}
	if dbConfig == nil {
		return nil, fmt.Errorf("agent config not found")
	}

	if req.Name != nil && *req.Name != "" {
		dbConfig.Name = *req.Name
	}
	if req.Config != nil {
		dbConfig.Config = *req.Config
	}

	err = c.agentConfigStore.Update(ctx, dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to update agent config: %w", err)
	}

	return &types.AgentConfig{
		ID:        dbConfig.ID,
		Name:      dbConfig.Name,
		Config:    dbConfig.Config,
		CreatedAt: dbConfig.CreatedAt,
		UpdatedAt: dbConfig.UpdatedAt,
	}, nil
}
