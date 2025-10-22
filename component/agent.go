package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
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

	// Chat operations
	ListSessionsByInstanceID(ctx context.Context, userUUID string, instanceID int64) ([]*types.AgentInstanceSession, int, error)
	ListSessionHistories(ctx context.Context, userUUID string, instanceID int64, sessionID int64) ([]*types.AgentInstanceSessionHistory, error)
	InitializeSession(ctx context.Context, userUUID string, instanceType string, contentID string, req *types.AgentChatRequest) (sessionUUID string, err error)
	RecordSessionHistory(ctx context.Context, req *types.RecordAgentInstanceSessionHistoryRequest) error
}

// agentComponentImpl implements the AgentComponent interface
type agentComponentImpl struct {
	config              *config.Config
	templateStore       database.AgentTemplateStore
	instanceStore       database.AgentInstanceStore
	sessionStore        database.AgentInstanceSessionStore
	sessionHistoryStore database.AgentInstanceSessionHistoryStore
	adapterFactory      *AgentInstanceAdapterFactory
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

	isRunning, err := adapter.IsInstanceRunning(ctx, userUUID, dbInstance.ContentID)
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
		Editable:    dbInstance.UserUUID == userUUID, //only the owner can edit the instance
		Name:        &dbInstance.Name,
		Description: &dbInstance.Description,
		IsRunning:   isRunning,
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
			isRunning, err = adapter.IsInstanceRunning(ctx, userUUID, dbInstance.ContentID)
			if err != nil {
				slog.Warn("failed to check if agent instance is running", "instance_type", dbInstance.Type, "content_id", dbInstance.ContentID, "user_uuid", userUUID, "error", err)
				isRunning = false
			}
		}
		typesInstances = append(typesInstances, &types.AgentInstance{
			ID:          dbInstance.ID,
			TemplateID:  &dbInstance.TemplateID,
			UserUUID:    &dbInstance.UserUUID,
			Type:        &dbInstance.Type,
			ContentID:   &dbInstance.ContentID,
			Public:      dbInstance.Public,
			Editable:    dbInstance.UserUUID == userUUID, //only the owner can edit the instance
			Name:        &dbInstance.Name,
			Description: &dbInstance.Description,
			IsRunning:   isRunning,
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

func (c *agentComponentImpl) InitializeSession(ctx context.Context, userUUID string, instanceType string, contentID string, req *types.AgentChatRequest) (sessionUUID string, err error) {
	if req == nil {
		return "", fmt.Errorf("chat request is nil")
	}

	// get instance from database
	instance, err := c.instanceStore.FindByContentID(ctx, instanceType, contentID)
	if err != nil {
		slog.Error("failed to get agent instance by content id", "instance_type", instanceType, "content_id", contentID, "user_uuid", userUUID, "error", err)
		return "", errorx.HandleDBError(err, map[string]any{
			"instance_type": instanceType,
			"content_id":    contentID,
			"user_uuid":     userUUID,
		})
	}

	// check permission
	if !instance.Public && instance.UserUUID != userUUID {
		return "", errorx.Forbidden(nil, map[string]any{
			"instance_type": instanceType,
			"content_id":    contentID,
			"user_uuid":     userUUID,
		})
	}

	// Generate session ID if not provided by client
	var newSession bool
	sessionID := req.SessionID
	if sessionID == nil || *sessionID == "" {
		generatedID := uuid.New().String()
		sessionID = &generatedID
		newSession = true
	} else {
		newSession, err = c.isNewSession(ctx, *sessionID)
		if err != nil {
			slog.Error("failed to check if session is new", "session_id", *sessionID, "error", err)
			return "", fmt.Errorf("failed to check if session is new, error:%w", err)
		}
	}

	var dbSession *database.AgentInstanceSession

	if newSession {
		// create a new session
		session := &database.AgentInstanceSession{
			UUID:       *sessionID,
			Name:       extractSessionName(req.InputValue),
			InstanceID: instance.ID,
			UserUUID:   userUUID,
			Type:       instanceType,
		}
		dbSession, err = c.sessionStore.Create(ctx, session)
		if err != nil {
			slog.Error("failed to create agent instance session", "session_id", *sessionID, "instance_id", instance.ID, "user_uuid", userUUID, "error", err)
			return "", fmt.Errorf("failed to create agent instance session, error:%w", err)
		}
	} else {
		dbSession, err = c.sessionStore.FindByUUID(ctx, *sessionID)
		if err != nil {
			slog.Error("failed to find agent instance session by uuid", "session_id", *sessionID, "instance_type", instanceType, "content_id", contentID, "user_uuid", userUUID, "error", err)
			return "", fmt.Errorf("failed to find agent instance session by uuid, error:%w", err)
		}
	}

	return dbSession.UUID, nil
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

func (c *agentComponentImpl) ListSessionsByInstanceID(ctx context.Context, userUUID string, instanceID int64) ([]*types.AgentInstanceSession, int, error) {
	sessions, total, err := c.sessionStore.ListByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, 0, err
	}

	// Convert []database.AgentInstanceSession to []types.AgentInstanceSession
	typesSessions := make([]*types.AgentInstanceSession, 0, len(sessions))
	for _, session := range sessions {
		typesSessions = append(typesSessions, &types.AgentInstanceSession{
			ID:         session.ID,
			UUID:       session.UUID,
			Name:       session.Name,
			InstanceID: session.InstanceID,
			UserUUID:   session.UserUUID,
			Type:       session.Type,
			CreatedAt:  session.CreatedAt,
			UpdatedAt:  session.UpdatedAt,
		})
	}
	return typesSessions, total, nil
}

func (c *agentComponentImpl) ListSessionHistories(ctx context.Context, userUUID string, instanceID int64, sessionID int64) ([]*types.AgentInstanceSessionHistory, error) {
	histories, err := c.sessionHistoryStore.ListBySessionID(ctx, sessionID)
	if err != nil {
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
			CreatedAt: history.CreatedAt,
			UpdatedAt: history.UpdatedAt,
		})
	}
	return typesHistories, nil
}

func (c *agentComponentImpl) RecordSessionHistory(ctx context.Context, req *types.RecordAgentInstanceSessionHistoryRequest) error {
	// get session from database by session uuid
	session, err := c.sessionStore.FindByUUID(ctx, req.SessionUUID)
	if err != nil {
		return fmt.Errorf("failed to find agent instance session by uuid, error:%w", err)
	}
	if session == nil {
		return fmt.Errorf("agent instance session not found, session_uuid: %s", req.SessionUUID)
	}

	// create a new session history
	history := &database.AgentInstanceSessionHistory{
		SessionID: session.ID,
		Request:   req.Request,
		Content:   req.Content,
	}
	err = c.sessionHistoryStore.Create(ctx, history)
	if err != nil {
		return fmt.Errorf("failed to create agent instance session history, error:%w", err)
	}
	return nil
}
