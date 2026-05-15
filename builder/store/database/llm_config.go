package database

import (
	"context"
	"fmt"
	"log/slog"

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
	ID           int64  `bun:",pk,autoincrement" json:"id"`
	ModelName    string `bun:",notnull,unique" json:"model_name"`
	OfficialName string `bun:"official_name,nullzero" json:"official_name"`
	ApiEndpoint  string `bun:",notnull" json:"api_endpoint"`
	AuthHeader   string `bun:",notnull" json:"auth_header"`
	// Upstreams are stored in the relational ai_gateway_upstreams table.
	Upstreams     []Upstream          `bun:"rel:has-many,join:id=llm_config_id" json:"upstreams"`
	Type          int                 `bun:",notnull" json:"type"` // 1: optimization, 2: comparison, 4: summary readme, 8: mcp scan, 16: for aigateway call external llm
	Enabled       bool                `bun:",notnull" json:"enabled"`
	Provider      string              `bun:"," json:"provider"`
	RoutingPolicy types.RoutingPolicy `bun:",type:jsonb,nullzero" json:"routing_policy"`
	Metadata      map[string]any      `bun:",type:jsonb,nullzero" json:"metadata"`
	// NeedSensitiveCheck controls whether requests for this model should go
	// through sensitive content detection in aigateway. Set to false to skip
	// the check (e.g. for guard models or trusted internal models).
	NeedSensitiveCheck bool        `bun:",notnull,default:false" json:"need_sensitive_check"`
	RepoID             int64       `bun:",nullzero" json:"repo_id"`
	Repo               *Repository `bun:"rel:belongs-to,join:repo_id=id" json:"repo,omitempty"`
	times
}

// LLM types
const (
	LLMTypeOptimization  = 1
	LLMTypeComparison    = 2
	LLMTypeSummaryReadme = 4
	LLMTypeMCPScanner    = 8

	LLMTypeAigatewayExternal = 16
)

type LLMConfigStore interface {
	GetOptimization(ctx context.Context) (*LLMConfig, error)
	GetModelForSummaryReadme(ctx context.Context) (*LLMConfig, error)
	GetByType(ctx context.Context, llmType int) (*LLMConfig, error)
	GetByID(ctx context.Context, id int64) (*LLMConfig, error)
	Index(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*LLMConfig, int, error)
	IndexWithRepo(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*LLMConfig, int, error)
	Update(ctx context.Context, config LLMConfig) (*LLMConfig, error)
	Create(ctx context.Context, config LLMConfig) (*LLMConfig, error)
	Delete(ctx context.Context, id int64) error
	GetByModelName(ctx context.Context, modelName string) (*LLMConfig, error)
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
	err := s.db.Operator.Core.NewSelect().Model(&config).Relation("Upstreams").Where("(type & ?) > 0 and enabled = true", LLMTypeOptimization).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select optimization llm, %w", err)
	}
	config.populateDerivedFields()
	return &config, nil
}

func (s *lLMConfigStoreImpl) GetModelForSummaryReadme(ctx context.Context) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Relation("Upstreams").Where("(type & ?) > 0 and enabled = true", LLMTypeSummaryReadme).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select llm for summary readme, %w", err)
	}
	config.populateDerivedFields()
	return &config, nil
}

func (s *lLMConfigStoreImpl) GetByType(ctx context.Context, llmType int) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Relation("Upstreams").Where("(type & ?) > 0 and enabled = true", llmType).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select llm by type %d, %w", llmType, err)
	}
	config.populateDerivedFields()
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

	query := s.db.Operator.Core.NewSelect().Model(&configs).Relation("Upstreams.HealthState").Relation("Upstreams.CircuitState").Limit(per).Offset(offset)
	buildSearchLLMConfigQuery(search, query)
	err := query.Scan(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("select batch llm config with search, %w", err)
	}
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	for _, cfg := range configs {
		cfg.populateDerivedFields()
	}
	return configs, total, nil
}

func (s *lLMConfigStoreImpl) IndexWithRepo(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*LLMConfig, int, error) {
	var configs []*LLMConfig
	offset := (page - 1) * per

	query := s.db.Operator.Core.NewSelect().Model(&configs).Relation("Repo").Relation("Upstreams").Limit(per).Offset(offset)
	buildSearchLLMConfigQuery(search, query)
	err := query.Scan(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("select batch llm config with repo, %w", err)
	}
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	for _, cfg := range configs {
		cfg.populateDerivedFields()
	}
	return configs, total, nil
}
func (s *lLMConfigStoreImpl) GetByModelName(ctx context.Context, modelName string) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Relation("Upstreams").Where("model_name = ?", modelName).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select llm config by model_name %s: %w", modelName, err)
	}
	config.populateDerivedFields()
	return &config, nil
}

func (s *lLMConfigStoreImpl) GetByID(ctx context.Context, id int64) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Relation("Upstreams").Relation("Upstreams.HealthState").Relation("Upstreams.CircuitState").Where("id = ?", id).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select llm config by id %d, %w", id, err)
	}
	config.populateDerivedFields()
	return &config, nil
}

// BuildSearchLLMConfigQuery constructs a LLMConfig search query
// TODO: full-text search
func buildSearchLLMConfigQuery(
	search *types.SearchLLMConfig,
	q *bun.SelectQuery,
) {
	// If search option is not provided, skip search filtering
	if search == nil {
		return
	}
	if search.Keyword != "" {
		q.Where("LOWER(llm_config.model_name) LIKE LOWER(?)", "%"+search.Keyword+"%")
	}
	// Filter by Type if provided
	if search.Type != nil {
		q.Where("llm_config.type = ?", *search.Type)
	}
	// Filter by Enabled if provided
	if search.Enabled != nil {
		q.Where("llm_config.enabled = ?", *search.Enabled)
	}
}

// populateDerivedFields fills ApiEndpoint, AuthHeader, Provider from the best available upstream.
// Prefers healthy enabled upstreams; falls back to the first enabled upstream.
// If no upstream is available at all, uses upstream[0] and logs a warning.
func (c *LLMConfig) populateDerivedFields() {
	if len(c.Upstreams) == 0 {
		return
	}
	// Prefer a healthy, enabled upstream.
	for _, u := range c.Upstreams {
		if u.Enabled && u.URL != "" && u.isHealthy() {
			c.ApiEndpoint = u.URL
			c.AuthHeader = u.AuthHeader
			c.Provider = u.Provider
			return
		}
	}
	// Fallback: first enabled upstream.
	for _, u := range c.Upstreams {
		if u.Enabled && u.URL != "" {
			c.ApiEndpoint = u.URL
			c.AuthHeader = u.AuthHeader
			c.Provider = u.Provider
			slog.Warn("no healthy upstream available, using first enabled upstream",
				"model_name", c.ModelName, "upstream_id", u.ID, "url", u.URL)
			return
		}
	}
	// Last resort: upstream[0] even if disabled.
	u := c.Upstreams[0]
	c.ApiEndpoint = u.URL
	c.AuthHeader = u.AuthHeader
	c.Provider = u.Provider
	slog.Error("no enabled upstream available, using upstream[0]",
		"model_name", c.ModelName, "upstream_id", u.ID, "url", u.URL)
}

// isHealthy checks whether this upstream has a healthy health state.
func (u *Upstream) isHealthy() bool {
	if u.HealthState == nil {
		return true // no health state yet, assume healthy
	}
	return u.HealthState.HealthState == "healthy"
}

// PrimaryEndpoint returns the URL of the best available upstream.
// Call populateDerivedFields() first after querying from DB.
func (c *LLMConfig) PrimaryEndpoint() string {
	return c.ApiEndpoint
}

// PrimaryAuthHeader returns the AuthHeader of the best available upstream.
func (c *LLMConfig) PrimaryAuthHeader() string {
	return c.AuthHeader
}

// PrimaryProvider returns the Provider of the best available upstream.
func (c *LLMConfig) PrimaryProvider() string {
	return c.Provider
}

// PrimaryOfficialName returns the ModelName of the best available upstream (or the config ModelName).
func (c *LLMConfig) PrimaryOfficialName() string {
	for _, u := range c.Upstreams {
		if u.Enabled && u.URL != "" && u.ModelName != "" {
			return u.ModelName
		}
	}
	return c.ModelName
}
