package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mq"
)

// AgentComponent defines the interface for agent-related operations
type AgentComponent interface {
	// Template operations
	CreateTemplate(ctx context.Context, template *types.AgentTemplate) error
	GetTemplateByID(ctx context.Context, id int64, userUUID string) (*types.AgentTemplate, error)
	ListTemplatesByUserUUID(ctx context.Context, userUUID string, filter types.AgentTemplateFilter, per int, page int) ([]types.AgentTemplate, int, error)
	UpdateTemplate(ctx context.Context, template *types.AgentTemplate) error
	DeleteTemplate(ctx context.Context, id int64, userUUID string) error

	// Instance operations
	CreateInstance(ctx context.Context, instance *types.AgentInstance) error
	GetInstanceByID(ctx context.Context, id int64, userUUID string) (*types.AgentInstance, error)
	ListInstancesByUserUUID(ctx context.Context, userUUID string, filter types.AgentInstanceFilter, per int, page int) ([]*types.AgentInstance, int, error)
	UpdateInstance(ctx context.Context, instance *types.AgentInstance) error
	UpdateInstanceByContentID(ctx context.Context, userUUID string, instanceType string, instanceContentID string, updateRequest types.UpdateAgentInstanceRequest) (*types.AgentInstance, error)
	DeleteInstance(ctx context.Context, id int64, userUUID string) error
	DeleteInstanceByContentID(ctx context.Context, userUUID string, instanceType string, instanceContentID string) error

	// Session operations
	CreateSession(ctx context.Context, userUUID string, req *types.CreateAgentInstanceSessionRequest) (sessionUUID string, err error)
	ListSessions(ctx context.Context, userUUID string, filter types.AgentInstanceSessionFilter, per int, page int) ([]*types.AgentInstanceSession, int, error)
	GetSessionByUUID(ctx context.Context, userUUID string, sessionUUID string, instanceID int64) (*types.AgentInstanceSession, error)
	DeleteSessionByUUID(ctx context.Context, userUUID string, sessionUUID string, instanceID int64) error
	UpdateSessionByUUID(ctx context.Context, userUUID string, sessionUUID string, instanceID int64, req *types.UpdateAgentInstanceSessionRequest) error
	ListSessionHistories(ctx context.Context, userUUID string, sessionUUID string, instanceID int64) ([]*types.AgentInstanceSessionHistory, error)
	PublishSessionHistoryMsg(ctx context.Context, userUUID string, instanceID int64, req *types.CreateSessionHistoryRequest) error
	CreateSessionHistory(ctx context.Context, msg *types.CreateSessionHistoryMessage) error
}

// agentComponentImpl implements the AgentComponent interface
type agentComponentImpl struct {
	config                    *config.Config
	templateStore             database.AgentTemplateStore
	instanceStore             database.AgentInstanceStore
	sessionStore              database.AgentInstanceSessionStore
	sessionHistoryStore       database.AgentInstanceSessionHistoryStore
	adapterFactory            *AgentInstanceAdapterFactory
	queue                     mq.MessageQueue
	sessionHistoryMsgConsumer jetstream.Consumer
	notificationSvcClient     rpc.NotificationSvcClient
}

var _ AgentComponent = (*agentComponentImpl)(nil)

// NewAgentComponent creates a new AgentComponent
func NewAgentComponent(config *config.Config) (AgentComponent, error) {
	c := &agentComponentImpl{
		config:              config,
		templateStore:       database.NewAgentTemplateStore(),
		instanceStore:       database.NewAgentInstanceStore(),
		sessionStore:        database.NewAgentInstanceSessionStore(),
		sessionHistoryStore: database.NewAgentInstanceSessionHistoryStore(),
		adapterFactory:      createAdapterFactory(config),
	}

	notificationSvcClient := rpc.NewNotificationSvcHttpClientBuilder(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
		rpc.AuthWithApiKey(config.APIToken)).WithRetry(3).WithDelay(time.Millisecond * 200).Build()
	c.notificationSvcClient = notificationSvcClient

	n, err := mq.GetOrInit(config)
	if err != nil {
		slog.Error("failed to init message queue", slog.Any("error", err))
		return nil, err
	}
	c.queue = n
	if err := c.queue.BuildAgentSessionHistoryMsgStream(config); err != nil {
		slog.Error("failed to build agent session history message stream", slog.Any("error", err))
		return nil, err
	}
	consumer, err := c.queue.BuildAgentSessionHistoryMsgConsumer()
	if err != nil {
		slog.Error("failed to build agent session history message consumer", slog.Any("error", err))
		return nil, err
	}
	c.sessionHistoryMsgConsumer = consumer
	if err := c.processSessionHistoryMsg(); err != nil {
		slog.Error("failed to process session history message", slog.Any("error", err))
		return nil, err
	}
	return c, nil
}

// createAdapterFactory creates and configures the adapter factory with all adapters
func createAdapterFactory(config *config.Config) *AgentInstanceAdapterFactory {
	factory := NewAgentInstanceAdapterFactory()

	// Register langflow adapter
	langflowAdapter, err := NewLangflowAgentInstanceAdapter(config)
	if err != nil {
		slog.Warn("failed to create langflow agent instance adapter", "error", err)
	} else {
		factory.RegisterAdapter("langflow", langflowAdapter)
	}

	// Register code adapter
	codeAdapter, err := NewCodeAgentInstanceAdapter(config)
	if err != nil {
		slog.Warn("failed to create code agent instance adapter", "error", err)
	} else {
		factory.RegisterAdapter("code", codeAdapter)
	}

	return factory
}

// CreateTemplate creates a new agent template
func (c *agentComponentImpl) CreateTemplate(ctx context.Context, template *types.AgentTemplate) error {
	// Convert types.AgentTemplate to database.AgentTemplate
	dbTemplate := &database.AgentTemplate{
		Type:        *template.Type,
		UserUUID:    *template.UserUUID,
		Name:        *template.Name,
		Description: *template.Description,
		Content:     *template.Content,
		Public:      template.Public,
	}

	createdTemplate, err := c.templateStore.Create(ctx, dbTemplate)
	if err != nil {
		slog.Error("failed to create agent template in database", "user_uuid", *template.UserUUID, "error", err)
		return err
	}
	template.ID = createdTemplate.ID

	return nil
}

// GetTemplateByID retrieves an agent template by ID
func (c *agentComponentImpl) GetTemplateByID(ctx context.Context, id int64, userUUID string) (*types.AgentTemplate, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid template ID")
	}

	dbTemplate, err := c.templateStore.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check permission: resource is public or user UUID matches
	if !dbTemplate.Public && dbTemplate.UserUUID != userUUID {
		return nil, errorx.Forbidden(nil, map[string]any{
			"template_id": id,
			"user_uuid":   userUUID,
		})
	}

	// Convert database.AgentTemplate to types.AgentTemplate
	return &types.AgentTemplate{
		ID:          dbTemplate.ID,
		Type:        &dbTemplate.Type,
		UserUUID:    &dbTemplate.UserUUID,
		Name:        &dbTemplate.Name,
		Description: &dbTemplate.Description,
		Content:     &dbTemplate.Content,
		Public:      dbTemplate.Public,
		CreatedAt:   dbTemplate.CreatedAt,
		UpdatedAt:   dbTemplate.UpdatedAt,
	}, nil
}

// ListTemplatesByUserUUID lists all templates for a specific user
func (c *agentComponentImpl) ListTemplatesByUserUUID(ctx context.Context, userUUID string, filter types.AgentTemplateFilter, per int, page int) ([]types.AgentTemplate, int, error) {
	if userUUID == "" {
		return nil, 0, fmt.Errorf("user uuid cannot be empty")
	}

	dbTemplates, total, err := c.templateStore.ListByUserUUID(ctx, userUUID, filter, per, page)
	if err != nil {
		return nil, 0, err
	}

	// Convert []database.AgentTemplate to []types.AgentTemplate
	typesTemplates := make([]types.AgentTemplate, 0, len(dbTemplates))
	for _, dbTemplate := range dbTemplates {
		typesTemplates = append(typesTemplates, types.AgentTemplate{
			ID:          dbTemplate.ID,
			Type:        &dbTemplate.Type,
			UserUUID:    &dbTemplate.UserUUID,
			Name:        &dbTemplate.Name,
			Description: &dbTemplate.Description,
			Public:      dbTemplate.Public,
			CreatedAt:   dbTemplate.CreatedAt,
			UpdatedAt:   dbTemplate.UpdatedAt,
		})
	}

	return typesTemplates, total, nil
}

// UpdateTemplate updates an existing agent template
func (c *agentComponentImpl) UpdateTemplate(ctx context.Context, template *types.AgentTemplate) error {
	if template == nil {
		return fmt.Errorf("template cannot be nil")
	}

	if template.ID <= 0 {
		return fmt.Errorf("invalid template ID")
	}

	// Verify the template exists before updating
	existing, err := c.templateStore.FindByID(ctx, template.ID)
	if err != nil {
		return err
	}

	// Ensure the user can only update their own templates
	if existing.UserUUID != *template.UserUUID {
		return errorx.Forbidden(nil, map[string]any{
			"template_id": template.ID,
			"user_uuid":   *template.UserUUID,
		})
	}

	// Convert types.AgentTemplate to database.AgentTemplate
	dbTemplate := &database.AgentTemplate{
		ID:          template.ID,
		Type:        *template.Type,
		UserUUID:    *template.UserUUID,
		Name:        *template.Name,
		Description: *template.Description,
		Content:     *template.Content,
		Public:      template.Public,
	}

	return c.templateStore.Update(ctx, dbTemplate)
}

// DeleteTemplate deletes an agent template
func (c *agentComponentImpl) DeleteTemplate(ctx context.Context, id int64, userUUID string) error {
	if id <= 0 {
		return fmt.Errorf("invalid template ID")
	}
	// Verify the template exists
	existing, err := c.templateStore.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Check permission: resource is public or user UUID matches
	if existing.UserUUID != userUUID {
		return errorx.Forbidden(nil, map[string]any{
			"template_id": id,
			"user_uuid":   userUUID,
		})
	}

	return c.templateStore.Delete(ctx, id)
}

// CreateInstance creates a new agent instance
func (c *agentComponentImpl) CreateInstance(ctx context.Context, instance *types.AgentInstance) error {
	var tmpl *database.AgentTemplate
	var err error
	if instance.TemplateID != nil {
		tmpl, err = c.templateStore.FindByID(ctx, *instance.TemplateID)
		if err != nil {
			slog.Error("failed to find agent template by id", "template_id", *instance.TemplateID, "error", err)
			return fmt.Errorf("failed to find agent template by id %d, error:%w", *instance.TemplateID, err)
		}

		// Check permission: resource is public or user UUID matches
		if !tmpl.Public && tmpl.UserUUID != *instance.UserUUID {
			slog.Error("forbidden to create agent instance from private template", "template_id", *instance.TemplateID, "user_uuid", *instance.UserUUID)
			return errorx.Forbidden(nil, map[string]any{
				"template_id": *instance.TemplateID,
				"user_uuid":   *instance.UserUUID,
			})
		}
	}

	adapter := c.adapterFactory.GetAdapter(*instance.Type)
	if adapter == nil {
		slog.Error("unsupported agent type", "user_uuid", *instance.UserUUID, "agent_type", *instance.Type)
		return fmt.Errorf("unsupported agent type: %s", *instance.Type)
	}

	creationResult, err := adapter.CreateInstance(ctx, *instance.UserUUID, instance, tmpl)
	if err != nil {
		slog.Error("failed to create agent instance", "user_uuid", *instance.UserUUID, "agent_type", *instance.Type, "error", err)
		return fmt.Errorf("failed to create agent instance, error:%w", err)
	}

	dbInstance := &database.AgentInstance{
		UserUUID:    *instance.UserUUID,
		Type:        *instance.Type,
		ContentID:   creationResult.ID,
		Public:      instance.Public,
		Name:        creationResult.Name,
		Description: creationResult.Description,
	}

	if instance.TemplateID != nil {
		dbInstance.TemplateID = *instance.TemplateID
	}

	createdInstance, err := c.instanceStore.Create(ctx, dbInstance)
	if err != nil {
		slog.Error("failed to create agent instance", "user_uuid", *instance.UserUUID, "error", err)
		//TODO: delete agent instance from target system using adapter
		if delErr := adapter.DeleteInstance(ctx, *instance.UserUUID, creationResult.ID); delErr != nil {
			slog.Error("failed to delete agent instance from target system", "user_uuid", *instance.UserUUID, "agent_type", *instance.Type, "content_id", creationResult.ID, "error", delErr)
		}
		return err
	}

	instance.ID = createdInstance.ID
	instance.TemplateID = &createdInstance.TemplateID
	instance.ContentID = &createdInstance.ContentID
	instance.Name = &createdInstance.Name
	instance.Description = &createdInstance.Description
	instance.Editable = true
	instance.CreatedAt = createdInstance.CreatedAt
	instance.UpdatedAt = createdInstance.UpdatedAt

	return nil
}

// GetInstanceByID retrieves an agent instance by ID
func (c *agentComponentImpl) GetInstanceByID(ctx context.Context, id int64, userUUID string) (*types.AgentInstance, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid instance ID")
	}

	dbInstance, err := c.instanceStore.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check permission: resource is public or user UUID matches
	if !dbInstance.Public && dbInstance.UserUUID != userUUID {
		return nil, errorx.ErrForbidden
	}

	adapter := c.adapterFactory.GetAdapter(dbInstance.Type)
	if adapter == nil {
		slog.Error("unsupported agent type", "instance_type", dbInstance.Type, "content_id", dbInstance.ContentID, "user_uuid", userUUID)
		return nil, fmt.Errorf("unsupported agent type: %s", dbInstance.Type)
	}

	isRunning, err := adapter.IsInstanceRunning(ctx, userUUID, dbInstance.ContentID, dbInstance.BuiltIn)
	if err != nil {
		slog.Warn("failed to check if agent instance is running", "instance_type", dbInstance.Type, "content_id", dbInstance.ContentID, "user_uuid", userUUID, "error", err)
		isRunning = false
	}

	// Convert database.AgentInstance to types.AgentInstance
	return &types.AgentInstance{
		ID:          dbInstance.ID,
		TemplateID:  &dbInstance.TemplateID,
		UserUUID:    &dbInstance.UserUUID,
		Type:        &dbInstance.Type,
		ContentID:   &dbInstance.ContentID,
		Public:      dbInstance.Public,
		Editable:    !dbInstance.BuiltIn && dbInstance.UserUUID == userUUID, //only the owner can edit the instance, built-in instances are not editable
		Name:        &dbInstance.Name,
		Description: &dbInstance.Description,
		IsRunning:   isRunning,
		BuiltIn:     dbInstance.BuiltIn,
		Metadata:    dbInstance.Metadata,
		CreatedAt:   dbInstance.CreatedAt,
		UpdatedAt:   dbInstance.UpdatedAt,
	}, nil
}

// ListInstancesByUserUUID lists all instances for a specific user
func (c *agentComponentImpl) ListInstancesByUserUUID(ctx context.Context, userUUID string, filter types.AgentInstanceFilter, per int, page int) ([]*types.AgentInstance, int, error) {
	dbInstances, total, err := c.instanceStore.ListByUserUUID(ctx, userUUID, filter, per, page)
	if err != nil {
		return nil, 0, err
	}

	// Convert []database.AgentInstance to []types.AgentInstance
	typesInstances := make([]*types.AgentInstance, 0, len(dbInstances))
	for _, dbInstance := range dbInstances {
		isRunning := false
		adapter := c.adapterFactory.GetAdapter(dbInstance.Type)
		if adapter != nil {
			isRunning, err = adapter.IsInstanceRunning(ctx, userUUID, dbInstance.ContentID, dbInstance.BuiltIn)
			if err != nil {
				slog.Warn("failed to check if agent instance is running", "instance_type", dbInstance.Type, "content_id", dbInstance.ContentID, "user_uuid", userUUID, "error", err)
			}
		}
		typesInstances = append(typesInstances, &types.AgentInstance{
			ID:          dbInstance.ID,
			TemplateID:  &dbInstance.TemplateID,
			UserUUID:    &dbInstance.UserUUID,
			Type:        &dbInstance.Type,
			ContentID:   &dbInstance.ContentID,
			Public:      dbInstance.Public,
			Editable:    !dbInstance.BuiltIn && dbInstance.UserUUID == userUUID, //only the owner can edit the instance, built-in instances are not editable
			Name:        &dbInstance.Name,
			Description: &dbInstance.Description,
			IsRunning:   isRunning,
			BuiltIn:     dbInstance.BuiltIn,
			Metadata:    dbInstance.Metadata,
			CreatedAt:   dbInstance.CreatedAt,
			UpdatedAt:   dbInstance.UpdatedAt,
		})
	}

	return typesInstances, total, nil
}

// UpdateInstance updates an existing agent instance
func (c *agentComponentImpl) UpdateInstance(ctx context.Context, instance *types.AgentInstance) error {
	if instance == nil {
		return fmt.Errorf("instance cannot be nil")
	}

	if instance.ID <= 0 {
		return fmt.Errorf("invalid instance ID")
	}

	// Verify the instance exists before updating
	existing, err := c.instanceStore.FindByID(ctx, instance.ID)
	if err != nil {
		return err
	}

	// Ensure the user can only update their own instances
	if existing.UserUUID != *instance.UserUUID {
		return errorx.ErrForbidden
	}

	// Convert types.AgentInstance to database.AgentInstance
	dbInstance := &database.AgentInstance{
		ID:         instance.ID,
		TemplateID: *instance.TemplateID,
		UserUUID:   *instance.UserUUID,
		Type:       *instance.Type,
		ContentID:  *instance.ContentID,
		Public:     instance.Public,
	}

	return c.instanceStore.Update(ctx, dbInstance)
}

// UpdateInstanceByContentID updates an existing agent instance by type and content id
func (c *agentComponentImpl) UpdateInstanceByContentID(ctx context.Context, userUUID string, instanceType string, instanceContentID string, updateRequest types.UpdateAgentInstanceRequest) (*types.AgentInstance, error) {
	// check permission
	instance, err := c.instanceStore.FindByContentID(ctx, instanceType, instanceContentID)
	if err != nil {
		return nil, err
	}
	if instance == nil {
		slog.Error("agent instance not found", "instance_type", instanceType, "content_id", instanceContentID, "user_uuid", userUUID)
		return nil, fmt.Errorf("agent instance not found")
	}
	if instance.UserUUID != userUUID {
		return nil, errorx.Forbidden(nil, map[string]any{
			"instance_type": instanceType,
			"content_id":    instanceContentID,
			"user_uuid":     userUUID,
		})
	}

	if updateRequest.Name != nil {
		instance.Name = *updateRequest.Name
	}
	if updateRequest.Description != nil {
		instance.Description = *updateRequest.Description
	}
	if err := c.instanceStore.Update(ctx, instance); err != nil {
		slog.Error("failed to update agent instance", "instance_type", instanceType, "content_id", instanceContentID, "user_uuid", userUUID, "error", err)
		return nil, err
	}

	return nil, nil
}

// DeleteInstance deletes an agent instance
func (c *agentComponentImpl) DeleteInstance(ctx context.Context, id int64, userUUID string) error {
	if id <= 0 {
		return fmt.Errorf("invalid instance ID")
	}

	// Verify the instance exists before deleting
	existing, err := c.instanceStore.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Ensure the user can only delete their own instances
	if existing.UserUUID != userUUID {
		return errorx.ErrForbidden
	}

	// forbid to delete built-in instances
	if existing.BuiltIn {
		slog.Error("cannot delete built-in instance", "instance_id", id, "user_uuid", userUUID)
		return fmt.Errorf("cannot delete built-in instance, id: %d", id)
	}

	// Get the appropriate adapter for the agent type
	adapter := c.adapterFactory.GetAdapter(existing.Type)
	if adapter != nil {
		// Delete the agent instance from the target system
		if err := adapter.DeleteInstance(ctx, userUUID, existing.ContentID); err != nil {
			slog.Error("failed to delete agent instance from target system", "user_uuid", userUUID, "agent_type", existing.Type, "content_id", existing.ContentID, "error", err)
			// Continue with database deletion even if target system deletion fails
		}
	} else {
		slog.Warn("no adapter found for agent type, skipping target system deletion", "agent_type", existing.Type, "content_id", existing.ContentID)
	}

	// Delete from database
	return c.instanceStore.Delete(ctx, id)
}

// DeleteInstanceByContentID deletes an agent instance by type and content id
func (c *agentComponentImpl) DeleteInstanceByContentID(ctx context.Context, userUUID string, instanceType string, instanceContentID string) error {
	// check permission
	instance, err := c.instanceStore.FindByContentID(ctx, instanceType, instanceContentID)
	if err != nil {
		return err
	}
	if instance == nil {
		return fmt.Errorf("agent instance not found")
	}
	if instance.UserUUID != userUUID {
		return errorx.Forbidden(nil, map[string]any{
			"instance_type": instanceType,
			"content_id":    instanceContentID,
			"user_uuid":     userUUID,
		})
	}

	if err := c.instanceStore.Delete(ctx, instance.ID); err != nil {
		slog.Error("failed to delete agent instance", "instance_type", instanceType, "content_id", instanceContentID, "user_uuid", userUUID, "error", err)
		return err
	}
	return nil
}

// check the session uuid is new
func (c *agentComponentImpl) isNewSession(ctx context.Context, sessionID string) (bool, error) {
	_, err := c.sessionStore.FindByUUID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, errorx.ErrDatabaseNoRows) {
			return true, nil
		}
		slog.Error("failed to find agent instance session by uuid", "session_id", sessionID, "error", err)
		return false, err
	}
	return false, nil
}

func (c *agentComponentImpl) CreateSession(ctx context.Context, userUUID string, req *types.CreateAgentInstanceSessionRequest) (sessionUUID string, err error) {
	if req == nil {
		return "", fmt.Errorf("create session request is nil")
	}

	if req.InstanceID == nil && req.ContentID == nil {
		return "", fmt.Errorf("instance ID or content ID is required for instance type: %s", req.Type)
	}

	var instance *database.AgentInstance
	if req.InstanceID != nil {
		instance, err = c.instanceStore.FindByID(ctx, *req.InstanceID)
		if err != nil {
			return "", fmt.Errorf("failed to find agent instance by ID, error:%w", err)
		}
	} else if req.ContentID != nil {
		instance, err = c.instanceStore.FindByContentID(ctx, req.Type, *req.ContentID)
		if err != nil {
			return "", fmt.Errorf("failed to find agent instance by content id and type, error:%w", err)
		}
	}

	if instance != nil && !instance.Public && instance.UserUUID != userUUID {
		return "", errorx.Forbidden(nil, map[string]any{
			"instance_id":   instance.ID,
			"instance_type": instance.Type,
			"content_id":    instance.ContentID,
			"user_uuid":     userUUID,
		})
	}

	var newSession bool
	if req.SessionUUID == nil || *req.SessionUUID == "" {
		generatedID := uuid.New().String()
		req.SessionUUID = &generatedID
		newSession = true
	} else {
		newSession, err = c.isNewSession(ctx, *req.SessionUUID)
		if err != nil {
			return "", fmt.Errorf("failed to check if session is new, error:%w", err)
		}
	}

	var dbSession *database.AgentInstanceSession

	if newSession {
		instanceID := int64(0)
		if req.InstanceID != nil {
			instanceID = *req.InstanceID
		}

		session := &database.AgentInstanceSession{
			UUID:       common.SafeDeref(req.SessionUUID),
			Name:       common.SafeDeref(req.Name),
			InstanceID: instanceID,
			UserUUID:   userUUID,
			Type:       instance.Type,
		}

		dbSession, err = c.sessionStore.Create(ctx, session)
		if err != nil {
			slog.Error("failed to create agent instance session",
				"session_id", *req.SessionUUID,
				"user_uuid", userUUID,
				"error", err)
			return "", fmt.Errorf("failed to create agent instance session, error:%w", err)
		}
	} else {
		dbSession, err = c.sessionStore.FindByUUID(ctx, *req.SessionUUID)
		if err != nil {
			slog.Error("failed to find agent instance session by uuid",
				"session_id", *req.SessionUUID,
				"instance_type", instance.Type,
				"user_uuid", userUUID,
				"error", err)
			return "", fmt.Errorf("failed to find agent instance session by uuid, error:%w", err)
		}
	}

	return dbSession.UUID, nil
}

func extractSessionName(inputValue string) string {
	name := inputValue
	if newlineIndex := strings.Index(name, "\n"); newlineIndex != -1 {
		name = name[:newlineIndex]
	}
	if len(name) > 20 {
		name = name[:20]
	}
	return name
}

func (c *agentComponentImpl) ListSessions(ctx context.Context, userUUID string, filter types.AgentInstanceSessionFilter, per int, page int) ([]*types.AgentInstanceSession, int, error) {

	sessions, total, err := c.sessionStore.List(ctx, filter, per, page)
	if err != nil {
		return nil, 0, err
	}

	// Convert []database.AgentInstanceSession to []types.AgentInstanceSession
	typesSessions := make([]*types.AgentInstanceSession, 0, len(sessions))
	for _, session := range sessions {
		typesSessions = append(typesSessions, &types.AgentInstanceSession{
			ID:          session.ID,
			SessionUUID: session.UUID,
			Name:        session.Name,
			InstanceID:  session.InstanceID,
			UserUUID:    session.UserUUID,
			Type:        session.Type,
			LastTurn:    session.LastTurn,
			CreatedAt:   session.CreatedAt,
			UpdatedAt:   session.UpdatedAt,
		})
	}
	return typesSessions, total, nil
}

func (c *agentComponentImpl) GetSessionByUUID(ctx context.Context, userUUID string, sessionUUID string, instanceID int64) (*types.AgentInstanceSession, error) {
	session, err := c.sessionStore.FindByUUID(ctx, sessionUUID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("agent instance session not found, session_uuid: %s", sessionUUID)
	}
	if session.UserUUID != userUUID {
		return nil, errorx.Forbidden(nil, map[string]any{
			"session_uuid": sessionUUID,
			"user_uuid":    userUUID,
		})
	}
	if session.InstanceID != instanceID {
		return nil, fmt.Errorf("agent instance session does not belong to the specified instance, session_uuid: %s, instance_id: %d, session_instance_id: %d", sessionUUID, instanceID, session.InstanceID)
	}
	return &types.AgentInstanceSession{
		ID:          session.ID,
		SessionUUID: session.UUID,
		Name:        session.Name,
		Type:        session.Type,
		InstanceID:  session.InstanceID,
		UserUUID:    session.UserUUID,
		LastTurn:    session.LastTurn,
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   session.UpdatedAt,
	}, nil
}

func (c *agentComponentImpl) DeleteSessionByUUID(ctx context.Context, userUUID string, sessionUUID string, instanceID int64) error {
	session, err := c.sessionStore.FindByUUID(ctx, sessionUUID)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("agent instance session not found, session_uuid: %s", sessionUUID)
	}
	if session.UserUUID != userUUID {
		return errorx.Forbidden(nil, map[string]any{
			"session_uuid": sessionUUID,
			"user_uuid":    userUUID,
		})
	}
	if session.InstanceID != instanceID {
		return fmt.Errorf("agent instance session does not belong to the specified instance, session_uuid: %s, instance_id: %d, session_instance_id: %d", sessionUUID, instanceID, session.InstanceID)
	}
	return c.sessionStore.Delete(ctx, session.ID)
}

func (c *agentComponentImpl) UpdateSessionByUUID(ctx context.Context, userUUID string, sessionUUID string, instanceID int64, req *types.UpdateAgentInstanceSessionRequest) error {
	session, err := c.sessionStore.FindByUUID(ctx, sessionUUID)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("agent instance session not found, session_uuid: %s", sessionUUID)
	}
	if session.InstanceID != instanceID {
		return fmt.Errorf("agent instance session does not belong to the specified instance, session_uuid: %s, instance_id: %d, session_instance_id: %d", sessionUUID, instanceID, session.InstanceID)
	}
	if session.UserUUID != userUUID {
		return errorx.Forbidden(nil, map[string]any{
			"session_uuid": sessionUUID,
			"user_uuid":    userUUID,
		})
	}

	session.Name = req.Name

	return c.sessionStore.Update(ctx, session)
}

func (c *agentComponentImpl) ListSessionHistories(ctx context.Context, userUUID string, sessionUUID string, instanceID int64) ([]*types.AgentInstanceSessionHistory, error) {
	session, err := c.sessionStore.FindByUUID(ctx, sessionUUID)
	if err != nil {
		slog.Error("failed to find agent instance session by uuid", "session_uuid", sessionUUID, "user_uuid", userUUID, "error", err)
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("agent instance session not found, session_uuid: %s", sessionUUID)
	}
	if session.UserUUID != userUUID {
		return nil, errorx.Forbidden(nil, map[string]any{
			"session_uuid": sessionUUID,
			"user_uuid":    userUUID,
		})
	}
	if session.InstanceID != instanceID {
		return nil, fmt.Errorf("agent instance session does not belong to the specified instance, session_uuid: %s, instance_id: %d, session_instance_id: %d", sessionUUID, instanceID, session.InstanceID)
	}

	histories, err := c.sessionHistoryStore.ListBySessionID(ctx, session.ID)
	if err != nil {
		slog.Error("failed to list agent instance session histories by session id", "session_id", session.ID, "user_uuid", userUUID, "error", err)
		return nil, err
	}

	// Convert []database.AgentInstanceSessionHistory to []types.AgentInstanceSessionHistory
	typesHistories := make([]*types.AgentInstanceSessionHistory, 0, len(histories))
	for _, history := range histories {
		typesHistories = append(typesHistories, &types.AgentInstanceSessionHistory{
			ID:        history.ID,
			SessionID: history.SessionID,
			Request:   history.Request,
			Content:   history.Content,
			Turn:      history.Turn,
			CreatedAt: history.CreatedAt,
			UpdatedAt: history.UpdatedAt,
		})
	}
	return typesHistories, nil
}

func (c *agentComponentImpl) CreateSessionHistory(ctx context.Context, msg *types.CreateSessionHistoryMessage) error {
	history := &database.AgentInstanceSessionHistory{
		SessionID: msg.SessionID,
		Request:   msg.Request,
		Content:   msg.Content,
	}

	if err := c.sessionHistoryStore.Create(ctx, history); err != nil {
		slog.Error("failed to create agent instance session history", "session_id", msg.SessionID, "request", msg.Request, "msg_uuid", msg.MsgUUID, "content", msg.Content, "error", err)
		return fmt.Errorf("failed to create agent instance session history, error:%w", err)
	}

	slog.Info("agent instance session history created", "session_id", msg.SessionID, "request", msg.Request, "msg_uuid", msg.MsgUUID)
	return nil
}

func (c *agentComponentImpl) PublishSessionHistoryMsg(ctx context.Context, userUUID string, instanceID int64, req *types.CreateSessionHistoryRequest) error {
	if req == nil {
		return fmt.Errorf("create session history request is nil")
	}

	session, err := c.sessionStore.FindByUUID(ctx, req.SessionUUID)
	if err != nil {
		slog.Error("failed to find agent instance session by uuid", "session_uuid", req.SessionUUID, "error", err)
		return fmt.Errorf("failed to find agent instance session by uuid, error:%w", err)
	}

	if session == nil {
		return fmt.Errorf("agent instance session not found, session_uuid: %s", req.SessionUUID)
	}

	if session.UserUUID != userUUID {
		return errorx.Forbidden(nil, map[string]any{
			"session_uuid": req.SessionUUID,
			"user_uuid":    userUUID,
		})
	}

	if session.InstanceID != instanceID {
		return fmt.Errorf("agent instance session does not belong to the specified instance, session_uuid: %s, instance_id: %d, session_instance_id: %d", req.SessionUUID, instanceID, session.InstanceID)
	}

	for _, message := range req.Messages {
		slog.Debug("publish session history message", "session_uuid", req.SessionUUID, "message", message)
		msg := types.CreateSessionHistoryMessage{
			MsgUUID:     uuid.New().String(),
			SessionID:   session.ID,
			SessionUUID: req.SessionUUID,
			Request:     message.Request,
			Content:     message.Content,
		}
		if err := c.queue.PublishAgentSessionHistoryMsg(msg); err != nil {
			slog.Error("failed to publish session history message", "session_uuid", req.SessionUUID, "message", message, "error", err)
			return fmt.Errorf("failed to publish session history message, error:%w", err)
		}
	}

	return nil
}

func (c *agentComponentImpl) processSessionHistoryMsg() error {
	slog.Debug("start processing session history messages")

	_, err := c.sessionHistoryMsgConsumer.Consume(func(msg jetstream.Msg) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered while processing session history message", slog.Any("recover", r))
				_ = msg.Nak()
			}
		}()
		slog.Debug("received session history message", slog.String("data", string(msg.Data())))

		var history types.CreateSessionHistoryMessage
		if err := json.Unmarshal(msg.Data(), &history); err != nil {
			slog.Error("failed to unmarshal session history message", "error", err, "session_uuid", history.SessionUUID, "msg_uuid", history.MsgUUID)
			_ = msg.Term()
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.CreateSessionHistory(ctx, &history); err != nil {
			slog.Error("failed to create session history", "error", err, "session_uuid", history.SessionUUID, "msg_uuid", history.MsgUUID)
			_ = msg.Nak()
			return
		}

		if err := msg.Ack(); err != nil {
			slog.Error("failed to ack session history message", "error", err, "session_uuid", history.SessionUUID, "msg_uuid", history.MsgUUID)
			return
		}
		slog.Debug("session history message processed and acked", "session_uuid", history.SessionUUID, "msg_uuid", history.MsgUUID)
	})

	if err != nil {
		slog.Error("failed to start consuming session history messages", "error", err)
		return err
	}

	return nil
}
