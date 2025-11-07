package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// AgentInstanceAdapter interface defines the contract for agent instance adapters
type AgentInstanceAdapter interface {
	CreateInstance(ctx context.Context, userUUID string, instance *types.AgentInstance, template *database.AgentTemplate) (*types.AgentInstanceCreationResult, error)
	DeleteInstance(ctx context.Context, userUUID string, contentID string) error
	UpdateInstance(ctx context.Context, userUUID string, instance *types.AgentInstance) error
	GetInstanceType() string
	IsInstanceRunning(ctx context.Context, userUUID string, contentID string, builtIn bool) (bool, error)
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

// LangflowAgentInstanceAdapter implements AgentInstanceAdapter for Langflow instances
type LangflowAgentInstanceAdapter struct {
	agenthubSvcClient rpc.AgentHubSvcClient
}

func NewLangflowAgentInstanceAdapter(config *config.Config) (AgentInstanceAdapter, error) {
	agenthubSvcClient := rpc.NewAgentHubSvcClientImpl(config.Agent.AgentHubServiceHost, config.Agent.AgentHubServiceToken)
	return &LangflowAgentInstanceAdapter{agenthubSvcClient: agenthubSvcClient}, nil
}

// NewLangflowAgentInstanceAdapterWithClient creates a LangflowAgentInstanceAdapter with a custom client (for testing)
func NewLangflowAgentInstanceAdapterWithClient(client rpc.AgentHubSvcClient) AgentInstanceAdapter {
	return &LangflowAgentInstanceAdapter{agenthubSvcClient: client}
}

func (a *LangflowAgentInstanceAdapter) GetInstanceType() string {
	return "langflow"
}

func (a *LangflowAgentInstanceAdapter) CreateInstance(ctx context.Context, userUUID string, instance *types.AgentInstance, template *database.AgentTemplate) (*types.AgentInstanceCreationResult, error) {
	var data json.RawMessage
	if template != nil {
		data = json.RawMessage(template.Content)
	} else {
		data = json.RawMessage("{}")
	}

	name := common.SafeDeref(instance.Name)
	desc := common.SafeDeref(instance.Description)

	resp, err := a.agenthubSvcClient.CreateAgentInstance(ctx, userUUID, &rpc.CreateAgentInstanceRequest{
		Name:        name,
		Description: desc,
		Data:        data,
	})
	if err != nil {
		slog.Error("failed to create agent instance in agenthub service", "user_uuid", userUUID, "error", err)
		return nil, err
	}

	metadata := make(map[string]any)
	if template != nil {
		metadata["template_metadata"] = template.Metadata
	}

	return &types.AgentInstanceCreationResult{
		ID:          resp.ID,
		Name:        resp.Name,
		Description: resp.Description,
		Metadata:    metadata,
	}, nil
}

func (a *LangflowAgentInstanceAdapter) DeleteInstance(ctx context.Context, userUUID string, contentID string) error {
	// return a.agenthubSvcClient.DeleteAgentInstance(ctx, userUUID, &rpc.DeleteAgentInstanceRequest{ContentID: contentID})
	return nil
}

func (a *LangflowAgentInstanceAdapter) UpdateInstance(ctx context.Context, userUUID string, instance *types.AgentInstance) error {
	return nil
}

func (a *LangflowAgentInstanceAdapter) IsInstanceRunning(ctx context.Context, userUUID string, contentID string, builtIn bool) (bool, error) {
	return true, nil
}

// CodeAgentInstanceAdapter implements AgentInstanceAdapter for Code instances
type CodeAgentInstanceAdapter struct {
	spaceComponent SpaceComponent
}

func NewCodeAgentInstanceAdapter(config *config.Config) (AgentInstanceAdapter, error) {
	spaceComponent, err := NewSpaceComponent(config)
	if err != nil {
		slog.Warn("failed to create space component", "error", err)
		return nil, fmt.Errorf("failed to create space component: %w", err)
	}
	return &CodeAgentInstanceAdapter{spaceComponent: spaceComponent}, nil
}

func (a *CodeAgentInstanceAdapter) GetInstanceType() string {
	return "code"
}

func (a *CodeAgentInstanceAdapter) CreateInstance(ctx context.Context, userUUID string, instance *types.AgentInstance, template *database.AgentTemplate) (*types.AgentInstanceCreationResult, error) {
	name := common.SafeDeref(instance.Name)
	desc := common.SafeDeref(instance.Description)
	contentID := common.SafeDeref(instance.ContentID)

	return &types.AgentInstanceCreationResult{
		ID:          contentID,
		Name:        name,
		Description: desc,
	}, nil
}

func (a *CodeAgentInstanceAdapter) DeleteInstance(ctx context.Context, userUUID string, contentID string) error {
	return nil
}

func (a *CodeAgentInstanceAdapter) UpdateInstance(ctx context.Context, userUUID string, instance *types.AgentInstance) error {
	return nil
}

func (a *CodeAgentInstanceAdapter) IsInstanceRunning(ctx context.Context, userUUID string, contentID string, builtIn bool) (bool, error) {
	if builtIn {
		return true, nil
	}
	splitPath := strings.Split(contentID, "/")
	if len(splitPath) != 2 {
		return false, fmt.Errorf("invalid contentID: %s", contentID)
	}
	namespace := splitPath[0]
	name := splitPath[1]
	_, status, err := a.spaceComponent.Status(ctx, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to get space status: %w", err)
	}
	return status == SpaceStatusRunning, nil
}
