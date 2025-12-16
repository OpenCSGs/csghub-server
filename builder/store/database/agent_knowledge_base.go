package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// AgentKnowledgeBase represents a knowledge base configuration for an agent
type AgentKnowledgeBase struct {
	ID          int64          `bun:",pk,autoincrement" json:"id"`
	UserUUID    string         `bun:",notnull" json:"user_uuid"`
	Name        string         `bun:",notnull" json:"name"`
	Description string         `bun:",nullzero" json:"description"`
	ContentID   string         `bun:",notnull,unique" json:"content_id"`    // Used to specify the unique id of the knowledge base resource
	Public      bool           `bun:",notnull" json:"public"`               // Whether the knowledge base is public
	Metadata    map[string]any `bun:",type:jsonb,nullzero" json:"metadata"` // Knowledge base metadata
	User        *User          `bun:"rel:belongs-to,join:user_uuid=uuid" json:"user"`
	times
}

// AgentKnowledgeBaseStore provides database operations for AgentKnowledgeBase
type AgentKnowledgeBaseStore interface {
	Create(ctx context.Context, kb *AgentKnowledgeBase) (*AgentKnowledgeBase, error)
	FindByID(ctx context.Context, id int64) (*AgentKnowledgeBase, error)
	FindByContentID(ctx context.Context, contentID string) (*AgentKnowledgeBase, error)
	Update(ctx context.Context, kb *AgentKnowledgeBase) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter types.AgentKnowledgeBaseFilter, per int, page int) ([]AgentKnowledgeBase, int, error)
	Exists(ctx context.Context, userUUID string, name string) (bool, error)
}

var _ AgentKnowledgeBaseStore = (*agentKnowledgeBaseStoreImpl)(nil)

// agentKnowledgeBaseStoreImpl is the implementation of AgentKnowledgeBaseStore
type agentKnowledgeBaseStoreImpl struct {
	db *DB
}

// NewAgentKnowledgeBaseStore creates a new AgentKnowledgeBaseStore
func NewAgentKnowledgeBaseStore() AgentKnowledgeBaseStore {
	return &agentKnowledgeBaseStoreImpl{
		db: defaultDB,
	}
}

// NewAgentKnowledgeBaseStoreWithDB creates a new AgentKnowledgeBaseStore with a specific DB
func NewAgentKnowledgeBaseStoreWithDB(db *DB) AgentKnowledgeBaseStore {
	return &agentKnowledgeBaseStoreImpl{
		db: db,
	}
}

// Create inserts a new AgentKnowledgeBase into the database
func (s *agentKnowledgeBaseStoreImpl) Create(ctx context.Context, kb *AgentKnowledgeBase) (*AgentKnowledgeBase, error) {
	res, err := s.db.Core.NewInsert().Model(kb).Exec(ctx, kb)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid": kb.UserUUID,
			"name":      kb.Name,
		})
	}
	return kb, nil
}

// FindByID retrieves an AgentKnowledgeBase by its ID
func (s *agentKnowledgeBaseStoreImpl) FindByID(ctx context.Context, id int64) (*AgentKnowledgeBase, error) {
	kb := &AgentKnowledgeBase{}
	err := s.db.Core.NewSelect().
		Model(kb).
		Relation("User").
		Where("agent_knowledge_base.id = ?", id).
		Scan(ctx, kb)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorx.ErrNotFound
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"knowledge_base_id": id,
		})
	}
	return kb, nil
}

// FindByContentID retrieves an AgentKnowledgeBase by its ContentID
func (s *agentKnowledgeBaseStoreImpl) FindByContentID(ctx context.Context, contentID string) (*AgentKnowledgeBase, error) {
	kb := &AgentKnowledgeBase{}
	err := s.db.Core.NewSelect().
		Model(kb).
		Relation("User").
		Where("agent_knowledge_base.content_id = ?", contentID).
		Scan(ctx, kb)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorx.ErrNotFound
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"content_id": contentID,
		})
	}
	return kb, nil
}

// Update updates an existing AgentKnowledgeBase
func (s *agentKnowledgeBaseStoreImpl) Update(ctx context.Context, kb *AgentKnowledgeBase) error {
	res, err := s.db.Core.NewUpdate().Model(kb).Where("id = ?", kb.ID).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"knowledge_base_id": kb.ID,
		})
	}
	return nil
}

// Delete deletes an AgentKnowledgeBase
func (s *agentKnowledgeBaseStoreImpl) Delete(ctx context.Context, id int64) error {
	res, err := s.db.Core.NewDelete().Model((*AgentKnowledgeBase)(nil)).Where("id = ?", id).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"knowledge_base_id": id,
		})
	}
	return nil
}

// applyAgentKnowledgeBaseFilters applies filters to the query
func (s *agentKnowledgeBaseStoreImpl) applyAgentKnowledgeBaseFilters(query *bun.SelectQuery, filter types.AgentKnowledgeBaseFilter) *bun.SelectQuery {
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("LOWER(name) LIKE LOWER(?)", searchPattern)
	}

	if filter.Public != nil {
		query = query.Where("public = ?", *filter.Public)
	}

	if filter.Editable != nil {
		if *filter.Editable {
			query = query.Where("user_uuid = ?", filter.UserUUID)
		} else {
			query = query.Where("user_uuid != ?", filter.UserUUID)
		}
	}

	return query
}

// List retrieves AgentKnowledgeBases with filtering and pagination
func (s *agentKnowledgeBaseStoreImpl) List(ctx context.Context, filter types.AgentKnowledgeBaseFilter, per int, page int) ([]AgentKnowledgeBase, int, error) {
	var knowledgeBases []AgentKnowledgeBase
	var total int

	q := s.db.Core.NewSelect().Model(&knowledgeBases).Where("user_uuid = ? OR public = ?", filter.UserUUID, true)

	q = s.applyAgentKnowledgeBaseFilters(q, filter)

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"operation": "count_agent_knowledge_bases",
		})
	}

	err = q.Order("updated_at DESC").Limit(per).Offset((page-1)*per).Scan(ctx, &knowledgeBases)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"operation": "list_agent_knowledge_bases",
		})
	}

	return knowledgeBases, total, nil
}

// Exists checks if an AgentKnowledgeBase exists
func (s *agentKnowledgeBaseStoreImpl) Exists(ctx context.Context, userUUID string, name string) (bool, error) {
	exists, err := s.db.Core.NewSelect().
		Model((*AgentKnowledgeBase)(nil)).
		Where("user_uuid = ? AND name = ?", userUUID, name).
		Exists(ctx)
	if err != nil {
		return false, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
			"name":      name,
		})
	}
	return exists, nil
}
