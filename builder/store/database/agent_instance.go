package database

import (
	"context"
	"strings"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// AgentTemplate represents the template for an agent
type AgentTemplate struct {
	ID          int64          `bun:",pk,autoincrement" json:"id"`
	Type        string         `bun:",notnull" json:"type"`         // Possible values: langflow, agno, code, etc.
	UserUUID    string         `bun:",notnull" json:"user_uuid"`    // Associated with the corresponding field in the User table
	Name        string         `bun:",notnull" json:"name"`         // Agent template name
	Description string         `bun:",nullzero" json:"description"` // Agent template description
	Content     string         `bun:",type:text" json:"content"`    // Used to store the complete content of the template
	Public      bool           `bun:",notnull" json:"public"`       // Whether the template is public
	Metadata    map[string]any `bun:",type:jsonb" json:"metadata"`  // Template metadata
	times
}

// AgentInstance represents an instance created from an agent template
type AgentInstance struct {
	ID          int64          `bun:",pk,autoincrement" json:"id"`
	TemplateID  int64          `bun:"" json:"template_id"`          // Associated with the id in the template table
	UserUUID    string         `bun:",notnull" json:"user_uuid"`    // Associated with the corresponding field in the User table
	Type        string         `bun:",notnull" json:"type"`         // Possible values: langflow, agno, code, etc.
	ContentID   string         `bun:",notnull" json:"content_id"`   // Used to specify the unique id of the instance resource
	Public      bool           `bun:",notnull" json:"public"`       // Whether the instance is public
	Name        string         `bun:",nullzero" json:"name"`        // Instance name
	Description string         `bun:",nullzero" json:"description"` // Instance description
	BuiltIn     bool           `bun:",notnull" json:"built_in"`     // Whether the instance is built-in
	Metadata    map[string]any `bun:",type:jsonb" json:"metadata"`  // Instance metadata
	times
}

// AgentTemplateStore provides database operations for AgentTemplate
type AgentTemplateStore interface {
	Create(ctx context.Context, template *AgentTemplate) (*AgentTemplate, error)
	FindByID(ctx context.Context, id int64) (*AgentTemplate, error)
	ListByUserUUID(ctx context.Context, userUUID string, filter types.AgentTemplateFilter, per int, page int) ([]AgentTemplate, int, error)
	Update(ctx context.Context, template *AgentTemplate) error
	Delete(ctx context.Context, id int64) error
}

// AgentInstanceStore provides database operations for AgentInstance
type AgentInstanceStore interface {
	Create(ctx context.Context, instance *AgentInstance) (*AgentInstance, error)
	FindByID(ctx context.Context, id int64) (*AgentInstance, error)
	FindByContentID(ctx context.Context, instanceType string, contentID string) (*AgentInstance, error)
	IsInstanceExistsByContentID(ctx context.Context, instanceType string, contentID string) (bool, error)
	ListByUserUUID(ctx context.Context, userUUID string, filter types.AgentInstanceFilter, per int, page int) ([]AgentInstance, int, error)
	Update(ctx context.Context, instance *AgentInstance) error
	Delete(ctx context.Context, id int64) error
	CountByUserAndType(ctx context.Context, userUUID string, instanceType string) (int, error)
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
func (s *agentTemplateStoreImpl) Create(ctx context.Context, template *AgentTemplate) (*AgentTemplate, error) {
	res, err := s.db.Core.NewInsert().Model(template).Exec(ctx, template)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"template_type": template.Type,
			"user_uuid":     template.UserUUID,
		})
	}
	return template, nil
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

func (s *agentTemplateStoreImpl) applyAgentTemplateFilters(query *bun.SelectQuery, filter types.AgentTemplateFilter) *bun.SelectQuery {
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("LOWER(name) LIKE LOWER(?) OR LOWER(description) LIKE LOWER(?)", searchPattern, searchPattern)
	}

	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}

	return query
}

// ListByUserUUID retrieves all AgentTemplates for a specific user
func (s *agentTemplateStoreImpl) ListByUserUUID(ctx context.Context, userUUID string, filter types.AgentTemplateFilter, per int, page int) ([]AgentTemplate, int, error) {
	var templates []AgentTemplate
	query := s.db.Core.NewSelect().Model(&templates).Where("user_uuid = ? OR public = ?", userUUID, true)

	query = s.applyAgentTemplateFilters(query, filter)

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
		})
	}

	err = query.Order("updated_at DESC").Limit(per).Offset((page-1)*per).Scan(ctx, &templates)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
		})
	}
	return templates, total, nil
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
func (s *agentInstanceStoreImpl) Create(ctx context.Context, instance *AgentInstance) (*AgentInstance, error) {
	res, err := s.db.Core.NewInsert().Model(instance).Exec(ctx, instance)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"template_id": instance.TemplateID,
			"user_uuid":   instance.UserUUID,
			"content_id":  instance.ContentID,
		})
	}
	return instance, nil
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

// FindByContentID retrieves an AgentInstance by its content ID
func (s *agentInstanceStoreImpl) FindByContentID(ctx context.Context, instanceType string, contentID string) (*AgentInstance, error) {
	instance := &AgentInstance{}
	err := s.db.Core.NewSelect().Model(instance).Where("type = ? AND content_id = ?", instanceType, contentID).Limit(1).Scan(ctx, instance)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"instance_type": instanceType,
			"content_id":    contentID,
		})
	}
	return instance, nil
}

func (s *agentInstanceStoreImpl) IsInstanceExistsByContentID(ctx context.Context, instanceType string, contentID string) (bool, error) {
	exists, err := s.db.Core.NewSelect().
		Model((*AgentInstance)(nil)).
		Where("type = ? AND content_id = ?", instanceType, contentID).
		Exists(ctx)
	if err != nil {
		return false, errorx.HandleDBError(err, map[string]any{
			"instance_type": instanceType,
			"content_id":    contentID,
		})
	}

	return exists, nil
}

func (s *agentInstanceStoreImpl) applyAgentInstanceFilters(query *bun.SelectQuery, filter types.AgentInstanceFilter) *bun.SelectQuery {
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("LOWER(name) LIKE LOWER(?) OR LOWER(description) LIKE LOWER(?)", searchPattern, searchPattern)
	}

	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}

	// Apply template ID filter
	if filter.TemplateID != nil {
		query = query.Where("template_id = ?", *filter.TemplateID)
	}

	if filter.BuiltIn != nil {
		query = query.Where("built_in = ?", *filter.BuiltIn)
	}

	return query
}

func (s *agentInstanceStoreImpl) ListByUserUUID(ctx context.Context, userUUID string, filter types.AgentInstanceFilter, per int, page int) ([]AgentInstance, int, error) {
	var instances []AgentInstance
	query := s.db.Core.NewSelect().Model(&instances).Where("user_uuid = ? OR public = ?", userUUID, true)

	query = s.applyAgentInstanceFilters(query, filter)

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
		})
	}

	err = query.Order("updated_at DESC").Limit(per).Offset((page-1)*per).Scan(ctx, &instances)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
		})
	}
	return instances, total, nil
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

// CountByUserAndType returns the count of agent instances for a specific user and type
func (s *agentInstanceStoreImpl) CountByUserAndType(ctx context.Context, userUUID string, instanceType string) (int, error) {
	count, err := s.db.Core.NewSelect().
		Model((*AgentInstance)(nil)).
		Where("user_uuid = ? AND type = ?", userUUID, instanceType).
		Count(ctx)
	if err != nil {
		return 0, errorx.HandleDBError(err, map[string]any{
			"user_uuid":     userUUID,
			"instance_type": instanceType,
		})
	}
	return count, nil
}
