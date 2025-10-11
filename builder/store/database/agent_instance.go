package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
)

// AgentTemplate represents the template for an agent
type AgentTemplate struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	Type        string `bun:",notnull" json:"type"`         // Possible values: langflow, agno, code, etc.
	UserUUID    string `bun:",notnull" json:"user_uuid"`    // Associated with the corresponding field in the User table
	Name        string `bun:",notnull" json:"name"`         // Agent template name
	Description string `bun:",nullzero" json:"description"` // Agent template description
	Content     string `bun:",type:text" json:"content"`    // Used to store the complete content of the template
	Public      bool   `bun:",notnull" json:"public"`       // Whether the template is public
	times
}

// AgentInstance represents an instance created from an agent template
type AgentInstance struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	TemplateID int64  `bun:"" json:"template_id"`        // Associated with the id in the template table
	UserUUID   string `bun:",notnull" json:"user_uuid"`  // Associated with the corresponding field in the User table
	Type       string `bun:",notnull" json:"type"`       // Possible values: langflow, agno, code, etc.
	ContentID  string `bun:",notnull" json:"content_id"` // Used to specify the unique id of the instance resource
	Public     bool   `bun:",notnull" json:"public"`     // Whether the instance is public
	times
}

// AgentTemplateStore provides database operations for AgentTemplate
type AgentTemplateStore interface {
	Create(ctx context.Context, template *AgentTemplate) error
	FindByID(ctx context.Context, id int64) (*AgentTemplate, error)
	ListByUserUUID(ctx context.Context, userUUID string) ([]AgentTemplate, error)
	Update(ctx context.Context, template *AgentTemplate) error
	Delete(ctx context.Context, id int64) error
}

// AgentInstanceStore provides database operations for AgentInstance
type AgentInstanceStore interface {
	Create(ctx context.Context, instance *AgentInstance) error
	FindByID(ctx context.Context, id int64) (*AgentInstance, error)
	ListByUserUUID(ctx context.Context, userUUID string) ([]AgentInstance, error)
	ListByTemplateID(ctx context.Context, templateID int64, userUUID string) ([]AgentInstance, error)
	Update(ctx context.Context, instance *AgentInstance) error
	Delete(ctx context.Context, id int64) error
}

// agentTemplateStoreImpl is the implementation of AgentTemplateStore
type agentTemplateStoreImpl struct {
	db *DB
}

// agentInstanceStoreImpl is the implementation of AgentInstanceStore
type agentInstanceStoreImpl struct {
	db *DB
}

// NewAgentTemplateStore creates a new AgentTemplateStore
func NewAgentTemplateStore() AgentTemplateStore {
	return &agentTemplateStoreImpl{
		db: defaultDB,
	}
}

// NewAgentTemplateStoreWithDB creates a new AgentTemplateStore with a specific DB
func NewAgentTemplateStoreWithDB(db *DB) AgentTemplateStore {
	return &agentTemplateStoreImpl{
		db: db,
	}
}

// NewAgentInstanceStore creates a new AgentInstanceStore
func NewAgentInstanceStore() AgentInstanceStore {
	return &agentInstanceStoreImpl{
		db: defaultDB,
	}
}

// NewAgentInstanceStoreWithDB creates a new AgentInstanceStore with a specific DB
func NewAgentInstanceStoreWithDB(db *DB) AgentInstanceStore {
	return &agentInstanceStoreImpl{
		db: db,
	}
}

// Create inserts a new AgentTemplate into the database
func (s *agentTemplateStoreImpl) Create(ctx context.Context, template *AgentTemplate) error {
	res, err := s.db.Core.NewInsert().Model(template).Exec(ctx, template)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"template_type": template.Type,
			"user_uuid":     template.UserUUID,
		})
	}
	return nil
}

// FindByID retrieves an AgentTemplate by its ID
func (s *agentTemplateStoreImpl) FindByID(ctx context.Context, id int64) (*AgentTemplate, error) {
	template := &AgentTemplate{}
	err := s.db.Core.NewSelect().Model(template).Where("id = ?", id).Scan(ctx, template)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"template_id": id,
		})
	}
	return template, nil
}

// ListByUserUUID retrieves all AgentTemplates for a specific user
func (s *agentTemplateStoreImpl) ListByUserUUID(ctx context.Context, userUUID string) ([]AgentTemplate, error) {
	var templates []AgentTemplate
	err := s.db.Core.NewSelect().Model(&templates).Where("user_uuid = ? OR public = ?", userUUID, true).Scan(ctx, &templates)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
		})
	}
	return templates, nil
}

// Update updates an existing AgentTemplate
func (s *agentTemplateStoreImpl) Update(ctx context.Context, template *AgentTemplate) error {
	res, err := s.db.Core.NewUpdate().Model(template).Where("id = ?", template.ID).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"template_id": template.ID,
		})
	}
	return nil
}

// Delete removes an AgentTemplate from the database
func (s *agentTemplateStoreImpl) Delete(ctx context.Context, id int64) error {
	res, err := s.db.Core.NewDelete().Model((*AgentTemplate)(nil)).Where("id = ?", id).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"template_id": id,
		})
	}
	return nil
}

// Create inserts a new AgentInstance into the database
func (s *agentInstanceStoreImpl) Create(ctx context.Context, instance *AgentInstance) error {
	res, err := s.db.Core.NewInsert().Model(instance).Exec(ctx, instance)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"template_id": instance.TemplateID,
			"user_uuid":   instance.UserUUID,
			"content_id":  instance.ContentID,
		})
	}
	return nil
}

// FindByID retrieves an AgentInstance by its ID
func (s *agentInstanceStoreImpl) FindByID(ctx context.Context, id int64) (*AgentInstance, error) {
	instance := &AgentInstance{}
	err := s.db.Core.NewSelect().Model(instance).Where("id = ?", id).Scan(ctx, instance)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"instance_id": id,
		})
	}
	return instance, nil
}

// ListByUserUUID retrieves all AgentInstances for a specific user
func (s *agentInstanceStoreImpl) ListByUserUUID(ctx context.Context, userUUID string) ([]AgentInstance, error) {
	var instances []AgentInstance
	err := s.db.Core.NewSelect().Model(&instances).Where("user_uuid = ? OR public = ?", userUUID, true).Scan(ctx, &instances)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
		})
	}
	return instances, nil
}

// ListByTemplateID retrieves all AgentInstances created from a specific template
func (s *agentInstanceStoreImpl) ListByTemplateID(ctx context.Context, templateID int64, userUUID string) ([]AgentInstance, error) {
	var instances []AgentInstance
	err := s.db.Core.NewSelect().Model(&instances).
		Where("template_id = ?", templateID).
		Where("user_uuid = ? OR public = ?", userUUID, true).
		Scan(ctx, &instances)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"template_id": templateID,
			"user_uuid":   userUUID,
		})
	}
	return instances, nil
}

// Update updates an existing AgentInstance
func (s *agentInstanceStoreImpl) Update(ctx context.Context, instance *AgentInstance) error {
	res, err := s.db.Core.NewUpdate().Model(instance).Where("id = ?", instance.ID).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"instance_id": instance.ID,
		})
	}
	return nil
}

// Delete removes an AgentInstance from the database
func (s *agentInstanceStoreImpl) Delete(ctx context.Context, id int64) error {
	res, err := s.db.Core.NewDelete().Model((*AgentInstance)(nil)).Where("id = ?", id).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"instance_id": id,
		})
	}
	return nil
}
