package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// AgentInstanceAdapter interface defines the contract for agent instance adapters
type AgentInstanceAdapter interface {
	CreateInstance(ctx context.Context, userUUID string, instance *types.AgentInstance, template *database.AgentTemplate) (*types.AgentInstanceCreationResult, error)
	DeleteInstance(ctx context.Context, userUUID string, contentID string) error
	UpdateInstance(ctx context.Context, userUUID string, instance *types.AgentInstance) error
	GetInstanceType() string
	IsInstanceRunning(ctx context.Context, userUUID string, contentID string, builtIn bool) (bool, error)
	Status(ctx context.Context, userUUID string, contentIDs []string, builtInMap map[string]bool) ([]types.AgentInstanceStatusResult, error)
	GetQuotaPerUser() int
}

// AgentInstanceAdapterFactory manages agent instance adapters
type AgentInstanceAdapterFactory struct {
	adapters map[string]AgentInstanceAdapter
}

func NewAgentInstanceAdapterFactory() *AgentInstanceAdapterFactory {
	return &AgentInstanceAdapterFactory{adapters: make(map[string]AgentInstanceAdapter)}
}

func (f *AgentInstanceAdapterFactory) GetAdapter(agentType string) AgentInstanceAdapter {
	return f.adapters[agentType]
}

func (f *AgentInstanceAdapterFactory) RegisterAdapter(agentType string, adapter AgentInstanceAdapter) {
	f.adapters[agentType] = adapter
}

func (f *AgentInstanceAdapterFactory) GetSupportedTypes() []string {
	types := make([]string, 0, len(f.adapters))
	for t := range f.adapters {
		types = append(types, t)
	}
	return types
}
