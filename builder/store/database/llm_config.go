package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type lLMConfigStoreImpl struct {
	db                  *DB
	dbDriver            string
	searchConfiguration string
}

type LLMConfig struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	ModelName   string `bun:",notnull" json:"model_name"`
	ApiEndpoint string `bun:",notnull" json:"api_endpoint"`
	AuthHeader  string `bun:",notnull" json:"auth_header"`
	Type        int    `bun:",notnull" json:"type"` // 1: optimization, 2: comparison, 4: summary readme, 8: mcp scan
	Enabled     bool   `bun:",notnull" json:"enabled"`
	times
}

// LLM types
const (
	LLMTypeOptimization  = 1
	LLMTypeComparison    = 2
	LLMTypeSummaryReadme = 4
	LLMTypeMCPScanner    = 8
)

type LLMConfigStore interface {
	GetOptimization(ctx context.Context) (*LLMConfig, error)
	GetModelForSummaryReadme(ctx context.Context) (*LLMConfig, error)
	GetByType(ctx context.Context, llmType int) (*LLMConfig, error)
	GetByID(ctx context.Context, id int64) (*LLMConfig, error)
	Index(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*LLMConfig, int, error)
	Update(ctx context.Context, config LLMConfig) (*LLMConfig, error)
	Create(ctx context.Context, config LLMConfig) (*LLMConfig, error)
	Delete(ctx context.Context, id int64) error
}

func NewLLMConfigStore(cfg *config.Config) LLMConfigStore {
	return &lLMConfigStoreImpl{
		db:                  defaultDB,
		dbDriver:            cfg.Database.Driver,
		searchConfiguration: cfg.Database.SearchConfiguration,
	}
}

func NewLLMConfigStoreWithDB(db *DB, cfg *config.Config) LLMConfigStore {
	return &lLMConfigStoreImpl{
		db:                  db,
		dbDriver:            cfg.Database.Driver,
		searchConfiguration: cfg.Database.SearchConfiguration,
	}
}

func (s *lLMConfigStoreImpl) GetOptimization(ctx context.Context) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Where("(type & ?) > 0 and enabled = true", LLMTypeOptimization).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select optimization llm, %w", err)
	}
	return &config, nil
}

func (s *lLMConfigStoreImpl) GetModelForSummaryReadme(ctx context.Context) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Where("(type & ?) > 0 and enabled = true", LLMTypeSummaryReadme).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select llm for summary readme, %w", err)
	}
	return &config, nil
}

func (s *lLMConfigStoreImpl) GetByType(ctx context.Context, llmType int) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Where("(type & ?) > 0 and enabled = true", llmType).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select llm by type %d, %w", llmType, err)
	}
	return &config, nil
}

func (s *lLMConfigStoreImpl) Update(ctx context.Context, config LLMConfig) (*LLMConfig, error) {
	_, err := s.db.Core.NewUpdate().Model(&config).Where("id = ?", config.ID).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update llm config, %w", err)
	}
	return &config, nil
}

func (s *lLMConfigStoreImpl) Create(ctx context.Context, config LLMConfig) (*LLMConfig, error) {
	res, err := s.db.Core.NewInsert().Model(&config).Exec(ctx, &config)
	if err != nil {
		return nil, fmt.Errorf("create llm config, %w", err)
	}
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create llm config, %w", err)
	}
	return &config, nil
}

func (s *lLMConfigStoreImpl) Delete(ctx context.Context, id int64) error {
	_, err := s.db.Operator.Core.NewDelete().Model((*LLMConfig)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete llm config, %w", err)
	}
	return nil
}
func (s *lLMConfigStoreImpl) Index(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*LLMConfig, int, error) {
	var configs []*LLMConfig
	offset := (page - 1) * per

	query := s.db.Operator.Core.NewSelect().Model(&configs).Limit(per).Offset(offset)
	buildSearchLLMConfigQuery(search, query, s.dbDriver, s.searchConfiguration)
	err := query.Scan(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("select batch llm config with search, %w", err)
	}
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	return configs, total, nil
}
func (s *lLMConfigStoreImpl) GetByID(ctx context.Context, id int64) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Where("id = ?", id).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select llm config by id %d, %w", id, err)
	}
	return &config, nil
}

// BuildSearchLLMConfigQuery constructs a LLMConfig search query
// TODO: full-text search
func buildSearchLLMConfigQuery(
	search *types.SearchLLMConfig,
	q *bun.SelectQuery,
	dbDriver string, // dbDriver is used to determine the database type for specific query syntax
	searchConfiguration string, // searchConfiguration is used for full-text search configuration
) {
	// If search option is not provided, skip search filtering
	if search == nil {
		return
	}
	if search.Keyword != "" {
		if dbDriver == "pg" {
			q.Where("llm_config.search_vector @@ websearch_to_tsquery(?, ?)", searchConfiguration, search.Keyword)
		} else {
			q.Where("llm_config.model_name = ?", search.Keyword)
		}
	}
	// Filter by Type if provided
	if search.Type != nil {
		q.Where("llm_config.type = ?", *search.Type)
	}

}
