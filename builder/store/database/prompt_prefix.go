package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type promptPrefixStoreImpl struct {
	db                  *DB
	dbDriver            string
	searchConfiguration string
}

type PromptPrefix struct {
	ID   int64  `bun:",pk,autoincrement" json:"id"`
	ZH   string `bun:",notnull" json:"zh"`
	EN   string `bun:",notnull" json:"en"`
	Kind string `bun:",notnull" json:"kind"`
}

type PromptPrefixStore interface {
	Get(ctx context.Context, kind types.PromptPrefixKind) (*PromptPrefix, error)
	GetByID(ctx context.Context, id int64) (*PromptPrefix, error)
	Index(ctx context.Context, per, page int, search *types.SearchPromptPrefix) ([]*PromptPrefix, int, error)
	Update(ctx context.Context, prefix PromptPrefix) (*PromptPrefix, error)
	Create(ctx context.Context, prefix PromptPrefix) (*PromptPrefix, error)
	Delete(ctx context.Context, id int64) error
}

func NewPromptPrefixStore(cfg *config.Config) PromptPrefixStore {
	return &promptPrefixStoreImpl{
		db:                  defaultDB,
		dbDriver:            cfg.Database.Driver,
		searchConfiguration: cfg.Database.SearchConfiguration,
	}
}

func NewPromptPrefixStoreWithDB(db *DB, cfg *config.Config) PromptPrefixStore {
	return &promptPrefixStoreImpl{
		db:                  db,
		dbDriver:            cfg.Database.Driver,
		searchConfiguration: cfg.Database.SearchConfiguration,
	}
}

func (p *promptPrefixStoreImpl) Get(ctx context.Context, kind types.PromptPrefixKind) (*PromptPrefix, error) {
	var prefix PromptPrefix
	err := p.db.Operator.Core.NewSelect().Model(&prefix).Where("kind = ?", kind).Order("id desc").Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select latest prompt prefix: %w", err)
	}
	return &prefix, nil
}

func (p *promptPrefixStoreImpl) GetByID(ctx context.Context, id int64) (*PromptPrefix, error) {
	var prefix PromptPrefix
	err := p.db.Operator.Core.NewSelect().Model(&prefix).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select prompt prefix by id: %w", err)
	}
	return &prefix, nil
}

func (p *promptPrefixStoreImpl) Index(ctx context.Context, per, page int, search *types.SearchPromptPrefix) ([]*PromptPrefix, int, error) {
	var prefixes []*PromptPrefix
	offset := (page - 1) * per
	query := p.db.Operator.Core.NewSelect().Model(&prefixes)
	buildSearchPromptPrefixQuery(search, query, p.dbDriver, p.searchConfiguration)
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	query.Limit(per).Offset(offset)
	err = query.Scan(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("select batch llm config with search, %w", err)
	}
	return prefixes, total, nil
}

func (p *promptPrefixStoreImpl) Update(ctx context.Context, prefix PromptPrefix) (*PromptPrefix, error) {
	_, err := p.db.Operator.Core.NewUpdate().Model(&prefix).Where("id = ?", prefix.ID).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update prompt prefix, %w", err)
	}
	return &prefix, nil
}

func (p *promptPrefixStoreImpl) Create(ctx context.Context, prefix PromptPrefix) (*PromptPrefix, error) {
	res, err := p.db.Core.NewInsert().Model(&prefix).Exec(ctx, &prefix)
	if err != nil {
		return nil, fmt.Errorf("create prompt prefix, %w", err)
	}
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create prompt prefix, %w", err)
	}
	return &prefix, nil
}

func (p *promptPrefixStoreImpl) Delete(ctx context.Context, id int64) error {
	_, err := p.db.Operator.Core.NewDelete().Model((*PromptPrefix)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete prompt prefix, %w", err)
	}
	return nil
}

// BuildSearchPromptPrefixQuery constructs a PromptPrefix search query
// TODO: full-text search
func buildSearchPromptPrefixQuery(
	search *types.SearchPromptPrefix,
	q *bun.SelectQuery,
	dbDriver string,
	searchConfiguration string,
) {
	// If search option is not provided, skip search filtering
	if search == nil {
		return
	}
	if search.Keyword != "" {
		if dbDriver == "pg" {
			q.Where("prompt_prefix.search_vector @@ websearch_to_tsquery(?, ?)", searchConfiguration, search.Keyword)
		} else {
			q.Where("prompt_prefix.zh = ? OR prompt_prefix.en = ?", search.Keyword, search.Keyword)
		}
	}
	// Filter by Type if provided
	if search.Kind != "" {
		q.Where("prompt_prefix.kind = ?", search.Kind)
	}

}
