package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type MCPServer struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	ToolsNum      int         `bun:",nullzero" json:"tools_num"`
	Configuration string      `bun:",nullzero" json:"configuration"` // server configuration json string
	Schema        string      `bun:",nullzero" json:"schema"`        // all properties json string
	times
}

type MCPServerProperty struct {
	ID          int64                 `bun:",pk,autoincrement" json:"id"`
	MCPServerID int64                 `bun:",notnull" json:"mcp_server_id"`
	MCPServer   *MCPServer            `bun:"rel:belongs-to,join:mcp_server_id=id" json:"mcp_server"`
	Kind        types.MCPPropertyKind `bun:",notnull" json:"kind"` // tool, prompt, resource, resource_template
	Name        string                `bun:",notnull" json:"name"`
	Description string                `bun:",nullzero" json:"description"`
	Schema      string                `bun:",nullzero" json:"schema"` // single property json string
	times
}

type MCPServerStore interface {
	ByRepoIDs(ctx context.Context, repoIDs []int64) ([]MCPServer, error)
	ByRepoID(ctx context.Context, repoID int64) (*MCPServer, error)
	ByPath(ctx context.Context, namespace string, name string) (*MCPServer, error)
	Create(ctx context.Context, input MCPServer) (*MCPServer, error)
	Delete(ctx context.Context, input MCPServer) error
	Update(ctx context.Context, input MCPServer) (*MCPServer, error)
	AddProperty(ctx context.Context, input MCPServerProperty) (*MCPServerProperty, error)
	DeletePropertiesByServerID(ctx context.Context, serverID int64) error
	DeleteProperty(ctx context.Context, input MCPServerProperty) error
	ListProperties(ctx context.Context, req *types.MCPPropertyFilter) ([]MCPServerProperty, int, error)
}

type mcpServerStoreImpl struct {
	db *DB
}

func NewMCPServerStore() MCPServerStore {
	return &mcpServerStoreImpl{
		db: defaultDB,
	}
}

func NewMCPServerStoreWithDB(db *DB) MCPServerStore {
	return &mcpServerStoreImpl{
		db: db,
	}
}

func (m *mcpServerStoreImpl) ByRepoIDs(ctx context.Context, repoIDs []int64) ([]MCPServer, error) {
	var mcps []MCPServer
	err := m.db.Operator.Core.NewSelect().
		Model(&mcps).
		Relation("Repository").
		Where("repository_id in (?)", bun.In(repoIDs)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select mcp servers by repo ids error: %w", err)
	}
	return mcps, nil
}

func (m *mcpServerStoreImpl) Create(ctx context.Context, input MCPServer) (*MCPServer, error) {
	res, err := m.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("insert mcp server in db error: %w", err)
	}
	return &input, nil
}

func (m *mcpServerStoreImpl) ByPath(ctx context.Context, namespace string, name string) (*MCPServer, error) {
	mcpServer := new(MCPServer)
	err := m.db.Operator.Core.
		NewSelect().
		Model(mcpServer).
		Relation("Repository.User").
		Where("repository.path =?", fmt.Sprintf("%s/%s", namespace, name)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select mcp server %s/%s error: %w", namespace, name, err)
	}
	err = m.db.Operator.Core.NewSelect().
		Model(mcpServer.Repository).
		WherePK().
		Relation("Tags", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("repository_tag.count > 0")
		}).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select mcp server %s/%s repository tags error: %w", namespace, name, err)
	}
	return mcpServer, nil
}

func (m *mcpServerStoreImpl) Delete(ctx context.Context, input MCPServer) error {
	res, err := m.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete mcp server %d error:%w", input.ID, err)
	}
	return nil
}

func (m *mcpServerStoreImpl) ByRepoID(ctx context.Context, repoID int64) (*MCPServer, error) {
	var mcpServer MCPServer
	err := m.db.Operator.Core.NewSelect().
		Model(&mcpServer).
		Where("repository_id = ?", repoID).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("select mcp server by repo id %d, error: %w", repoID, err)
	}

	return &mcpServer, nil
}

func (m *mcpServerStoreImpl) Update(ctx context.Context, input MCPServer) (*MCPServer, error) {
	res, err := m.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("update mcp server %d error: %w", input.ID, err)
	}
	return &input, nil
}

func (m *mcpServerStoreImpl) AddProperty(ctx context.Context, input MCPServerProperty) (*MCPServerProperty, error) {
	res, err := m.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("insert mcp server property error: %w", err)
	}
	return &input, nil
}

func (m *mcpServerStoreImpl) ListProperties(ctx context.Context, req *types.MCPPropertyFilter) ([]MCPServerProperty, int, error) {
	var mcpProps []MCPServerProperty
	var count int
	q := m.db.Operator.Core.NewSelect().Model(&mcpProps).Relation("MCPServer").Relation("MCPServer.Repository").Where("kind = ?", req.Kind)

	if !req.IsAdmin {
		if len(req.UserIDs) > 0 {
			q.Where("mcp_server__repository.private = ? or mcp_server__repository.user_id in (?)", false, bun.In(req.UserIDs))
		} else {
			q.Where("mcp_server__repository.private = ?", false)
		}
	}

	if len(req.Search) > 0 {
		q = q.Where("mcp_server_property.name LIKE ? OR mcp_server_property.description LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}

	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count mcp tools error: %w", err)
	}
	q = q.Order("id DESC")
	q = q.Limit(req.Per).Offset((req.Page - 1) * req.Per)
	err = q.Scan(ctx, &mcpProps)
	if err != nil {
		return nil, 0, fmt.Errorf("select mcp tools error: %w", err)
	}

	var repoIDs []int64
	for _, prop := range mcpProps {
		if prop.MCPServer != nil {
			repoIDs = append(repoIDs, prop.MCPServer.RepositoryID)
		}
	}

	repos := make([]*Repository, 0)
	err = m.db.Operator.Core.NewSelect().Model(&repos).Column("repository.id").
		Relation("Tags").Where("repository.id in (?)", bun.In(repoIDs)).Scan(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("load repository tags for mcp servers error: %w", err)
	}

	repoMap := make(map[int64]*Repository)
	for _, repo := range repos {
		repoMap[repo.ID] = repo
	}

	for _, prop := range mcpProps {
		if _, ok := repoMap[prop.MCPServer.RepositoryID]; !ok {
			continue
		}
		prop.MCPServer.Repository.Tags = repoMap[prop.MCPServer.RepositoryID].Tags
	}

	return mcpProps, count, nil
}

func (m *mcpServerStoreImpl) DeleteProperty(ctx context.Context, input MCPServerProperty) error {
	res, err := m.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete mcp server property %d error:%w", input.ID, err)
	}
	return nil
}

func (m *mcpServerStoreImpl) DeletePropertiesByServerID(ctx context.Context, serverID int64) error {
	_, err := m.db.Operator.Core.NewDelete().Model(&MCPServerProperty{}).Where("mcp_server_id = ?", serverID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete mcp server properties by server id %d error:%w", serverID, err)
	}
	return nil
}
