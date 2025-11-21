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

// LangflowAgentInstanceAdapter implements AgentInstanceAdapter for Langflow instances
type LangflowAgentInstanceAdapter struct {
	agenthubSvcClient rpc.AgentHubSvcClient
	config            *config.Config
}

func NewLangflowAgentInstanceAdapter(config *config.Config) (AgentInstanceAdapter, error) {
	agenthubSvcClient := rpc.NewAgentHubSvcClientImpl(config.Agent.AgentHubServiceHost, config.Agent.AgentHubServiceToken)
	return &LangflowAgentInstanceAdapter{agenthubSvcClient: agenthubSvcClient, config: config}, nil
}

// NewLangflowAgentInstanceAdapterWithClient creates a LangflowAgentInstanceAdapter with a custom client (for testing)
func NewLangflowAgentInstanceAdapterWithClient(client rpc.AgentHubSvcClient, config *config.Config) AgentInstanceAdapter {
	return &LangflowAgentInstanceAdapter{agenthubSvcClient: client, config: config}
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
	return a.agenthubSvcClient.DeleteAgentInstance(ctx, userUUID, contentID)
}

func (a *LangflowAgentInstanceAdapter) UpdateInstance(ctx context.Context, userUUID string, instance *types.AgentInstance) error {
	return nil
}

func (a *LangflowAgentInstanceAdapter) IsInstanceRunning(ctx context.Context, userUUID string, contentID string, builtIn bool) (bool, error) {
	return true, nil
}

func (a *LangflowAgentInstanceAdapter) GetQuotaPerUser() int {
	return a.config.Agent.LangflowInstanceQuotaPerUser
}

// CodeAgentInstanceAdapter implements AgentInstanceAdapter for Code instances
type CodeAgentInstanceAdapter struct {
	config          *config.Config
	spaceComponent  SpaceComponent
	userSvcClient   rpc.UserSvcClient
	csgbotSvcClient rpc.CsgbotSvcClient
}

func NewCodeAgentInstanceAdapter(config *config.Config) (AgentInstanceAdapter, error) {
	spaceComponent, err := NewSpaceComponent(config)
	if err != nil {
		slog.Warn("failed to create space component", "error", err)
		return nil, fmt.Errorf("failed to create space component: %w", err)
	}
	userSvcClient := rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	csgbotSvcClient := rpc.NewCsgbotSvcHttpClient(fmt.Sprintf("%s:%d", config.CSGBot.Host, config.CSGBot.Port),
		rpc.AuthWithApiKey(config.APIToken))
	return &CodeAgentInstanceAdapter{config: config, spaceComponent: spaceComponent, userSvcClient: userSvcClient, csgbotSvcClient: csgbotSvcClient}, nil
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

func parseContentID(contentID string) (namespace, name string, err error) {
	splitPath := strings.Split(contentID, "/")
	if len(splitPath) != 2 {
		return "", "", fmt.Errorf("invalid contentID: %s", contentID)
	}
	return splitPath[0], splitPath[1], nil
}

func (a *CodeAgentInstanceAdapter) DeleteInstance(ctx context.Context, userUUID string, contentID string) error {
	namespace, name, err := parseContentID(contentID)
	if err != nil {
		return err
	}

	users, err := a.userSvcClient.FindByUUIDs(ctx, []string{userUUID})
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if len(users) == 0 || users[userUUID] == nil {
		return fmt.Errorf("user not found: %s", userUUID)
	}

	username := users[userUUID].Username

	// Delete workspace files for code agent. Code agent may not create a space, so we need to clean up workspace files explicitly.
	// Note: If a space exists, workspace files will also be deleted when the space is deleted, but this ensures cleanup even when no space was created.
	if err := a.deleteWorkspaceFiles(ctx, userUUID, username, name); err != nil {
		slog.Warn("failed to delete workspace files for code agent", "error", err)
	}

	return a.spaceComponent.Delete(ctx, namespace, name, username)
}

func (a *CodeAgentInstanceAdapter) deleteWorkspaceFiles(ctx context.Context, userUUID string, username string, agentName string) error {
	token, err := a.userSvcClient.GetOrCreateFirstAvaiTokens(ctx, username, username, string(types.AccessTokenAppGit), "csgbot")
	if err != nil {
		return fmt.Errorf("failed to get or create access token for csgbot: %w", err)
	}
	if len(token) == 0 {
		return fmt.Errorf("can not get access token for csgbot")
	}

	err = a.csgbotSvcClient.DeleteWorkspaceFiles(ctx, userUUID, username, token, agentName)
	if err != nil {
		return fmt.Errorf("failed to delete workspace files for code agent: %w", err)
	}

	return nil
}

func (a *CodeAgentInstanceAdapter) UpdateInstance(ctx context.Context, userUUID string, instance *types.AgentInstance) error {
	return nil
}

func (a *CodeAgentInstanceAdapter) IsInstanceRunning(ctx context.Context, userUUID string, contentID string, builtIn bool) (bool, error) {
	if builtIn {
		return true, nil
	}
	namespace, name, err := parseContentID(contentID)
	if err != nil {
		return false, err
	}
	_, status, err := a.spaceComponent.Status(ctx, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to get space status: %w", err)
	}
	return status == SpaceStatusRunning, nil
}

func (a *CodeAgentInstanceAdapter) GetQuotaPerUser() int {
	return a.config.Agent.CodeInstanceQuotaPerUser
}
