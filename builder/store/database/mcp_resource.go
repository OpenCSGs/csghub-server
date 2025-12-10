package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type MCPResource struct {
	ID          int64          `bun:",pk,autoincrement" json:"id"`
	Name        string         `bun:",notnull" json:"name"`
	Description string         `bun:",notnull" json:"description"`
	Owner       string         `bun:",nullzero" json:"owner"`
	Avatar      string         `bun:",nullzero" json:"avatar"`
	Url         string         `bun:",notnull" json:"url"`
	Protocol    string         `bun:",notnull" json:"protocol"` // sse/streamable
	Headers     map[string]any `bun:"type:jsonb,nullzero" json:"headers"`
	NeedInstall bool           `bun:",notnull,default:false" json:"need_install"` // set this to true if the headers need to be set by the user or some other before use
	times
}

type MCPResourceStore interface {
	Create(ctx context.Context, input *MCPResource) (*MCPResource, error)
	Update(ctx context.Context, input *MCPResource) (*MCPResource, error)
	Delete(ctx context.Context, input *MCPResource) error
	List(ctx context.Context, filter *types.MCPFilter) ([]MCPResource, int, error)
}

type mcpResourceStoreImpl struct {
	db *DB
}

func NewMCPResourceStore() MCPResourceStore {
	return &mcpResourceStoreImpl{
		db: defaultDB,
	}
}

func NewMCPResourceStoreWithDB(db *DB) MCPResourceStore {
	return &mcpResourceStoreImpl{
		db: db,
	}
}

func (m *mcpResourceStoreImpl) Create(ctx context.Context, input *MCPResource) (*MCPResource, error) {
	res, err := m.db.Core.NewInsert().Model(input).Exec(ctx, input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return input, nil
}

func (m *mcpResourceStoreImpl) Update(ctx context.Context, input *MCPResource) (*MCPResource, error) {
	res, err := m.db.Core.NewUpdate().Model(input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return input, nil
}

func (m *mcpResourceStoreImpl) Delete(ctx context.Context, input *MCPResource) error {
	res, err := m.db.Operator.Core.NewDelete().Model(input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, nil)
	}
	return nil
}

func (m *mcpResourceStoreImpl) List(ctx context.Context, filter *types.MCPFilter) ([]MCPResource, int, error) {
	var mcpResList []MCPResource
	var count int
	q := m.db.Operator.Core.NewSelect().Model(&mcpResList)

	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}

	q = q.Order("id DESC")
	q = q.Limit(filter.Per).Offset((filter.Page - 1) * filter.Per)
	err = q.Scan(ctx, &mcpResList)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}

	return mcpResList, count, nil
}
