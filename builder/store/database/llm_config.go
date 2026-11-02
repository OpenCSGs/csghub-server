package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

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
	NeedSensitiveCheck bool        `bun:",notnull" json:"need_sensitive_check"`
	RepoID             int64       `bun:",nullzero" json:"repo_id"`
	Repo               *Repository `bun:"rel:belongs-to,join:repo_id=id" json:"repo,omitempty"`
	// ModelSizeB is the parameter size of the model in billions. 0 means unknown.
	ModelSizeB float64 `bun:"model_size_b,notnull,default:0" json:"model_size_b"`
	// AccessPolicy controls who can view and call the model: "public" allows
	// every authenticated caller, "whitelist" restricts access to the explicit
	// grants stored in llm_config_model_grants.
	AccessPolicy string `bun:"access_policy,notnull,default:'public'" json:"access_policy"`
	// IsPinned is a query-side flag for the gateway model listing: it reports
	// whether the caller pinned this model, computed by ListGatewayModels on
	// the same statement as the pinned-first ordering. Never persisted.
	IsPinned bool `bun:"-" json:"is_pinned,omitempty"`
	times
}

// llmConfigTasksKey is the metadata key under which a model's pipeline tasks are
// stored (a JSON array of strings).
const llmConfigTasksKey = "tasks"

// Tasks returns the pipeline tasks stored in the model metadata. The value is a
// JSON array of strings, surfaced as either []any (after a JSONB round trip) or
// []string (when set programmatically).
func (c *LLMConfig) Tasks() []string {
	if c == nil || c.Metadata == nil {
		return nil
	}
	raw, ok := c.Metadata[llmConfigTasksKey]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case []any:
		tasks := make([]string, 0, len(value))
		for _, item := range value {
			if task, ok := item.(string); ok {
				tasks = append(tasks, task)
			}
		}
		return tasks
	case []string:
		return value
	default:
		return nil
	}
}

// LLM types
const (
	LLMTypeOptimization  = 1
	LLMTypeComparison    = 2
	LLMTypeSummaryReadme = 4
	LLMTypeMCPScanner    = 8

	LLMTypeAigatewayExternal = 16
)

// LLMTypeFlags is the canonical list of single-bit LLM type flags.
// When adding a new LLM type, add its power-of-two constant here so validation and response splitting stay in sync.
var LLMTypeFlags = []int{
	LLMTypeOptimization,
	LLMTypeComparison,
	LLMTypeSummaryReadme,
	LLMTypeMCPScanner,
	LLMTypeAigatewayExternal,
}

var llmTypeMask = CombineLLMTypes(LLMTypeFlags)

// IsValidLLMType reports whether t is a non-empty bitmask composed only of known LLM type flags.
func IsValidLLMType(t int) bool {
	return t > 0 && t&^llmTypeMask == 0
}

// CombineLLMTypes folds a slice of individual LLM type flags into a single bitmask.
func CombineLLMTypes(types []int) int {
	mask := 0
	for _, typ := range types {
		mask |= typ
	}
	return mask
}

// SplitLLMType expands a type bitmask into the individual flags it contains.
func SplitLLMType(t int) []int {
	if t <= 0 {
		return nil
	}
	var result []int
	for _, bit := range LLMTypeFlags {
		if t&bit == bit {
			result = append(result, bit)
		}
	}
	return result
}

type LLMConfigStore interface {
	GetOptimization(ctx context.Context) (*LLMConfig, error)
	GetModelForSummaryReadme(ctx context.Context) (*LLMConfig, error)
	GetByType(ctx context.Context, llmType int) (*LLMConfig, error)
	GetByID(ctx context.Context, id int64) (*LLMConfig, error)
	Index(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*LLMConfig, int, error)
	IndexWithRepo(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*LLMConfig, int, error)
	// ListGatewayModels serves the aigateway /v1/models listing: enabled
	// configs with keyword/type/task/association filters, internal-model
	// visibility, upstream availability, price-configured and pinned-first
	// ordering all pushed down to SQL. limit < 0 disables the row window (returns all rows); limit >= 0 applies
	// a plain LIMIT (0 returns no rows) shifted by offset. Rows carry the Repo
	// and Upstreams (health/circuit) relations and the second result is the
	// total count of rows matching the filters.
	ListGatewayModels(ctx context.Context, limit, offset int, q *types.GatewayModelListQuery) ([]*LLMConfig, int, error)
	Update(ctx context.Context, config LLMConfig) (*LLMConfig, error)
	Create(ctx context.Context, config LLMConfig) (*LLMConfig, error)
	Delete(ctx context.Context, id int64) error
	GetByModelName(ctx context.Context, modelName string) (*LLMConfig, error)
	// GetByRepoPath finds the LLM config whose linked repository has the given
	// path (e.g. "ns/name"); returns (nil, nil) when none exists.
	GetByRepoPath(ctx context.Context, repoPath string) (*LLMConfig, error)
	// GetByModelNameForCaller behaves like GetByModelName but hides whitelist
	// models from callers without a matching grant (single query).
	GetByModelNameForCaller(ctx context.Context, modelName string, caller *types.GatewayWhitelistCaller) (*LLMConfig, error)
	// IndexWithRepoForCaller behaves like IndexWithRepo but hides whitelist
	// models from callers without a matching grant (single query).
	IndexWithRepoForCaller(ctx context.Context, per, page int, search *types.SearchLLMConfig, caller *types.GatewayWhitelistCaller) ([]*LLMConfig, int, error)
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

// ResolveLLMTasks returns the pipeline tasks of the LLM config identified by
// llmConfigID, or nil when the store is nil or the config cannot be loaded.
func ResolveLLMTasks(ctx context.Context, store LLMConfigStore, llmConfigID int64) []string {
	if store == nil {
		return nil
	}
	cfg, err := store.GetByID(ctx, llmConfigID)
	if err != nil || cfg == nil {
		return nil
	}
	return cfg.Tasks()
}

func (s *lLMConfigStoreImpl) GetOptimization(ctx context.Context) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Relation("Upstreams").Relation("Upstreams.HealthState").Where("(type & ?) > 0 and enabled = true", LLMTypeOptimization).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select optimization llm, %w", err)
	}
	return &config, nil
}

func (s *lLMConfigStoreImpl) GetModelForSummaryReadme(ctx context.Context) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Relation("Upstreams").Relation("Upstreams.HealthState").Where("(type & ?) > 0 and enabled = true", LLMTypeSummaryReadme).Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select llm for summary readme, %w", err)
	}
	return &config, nil
}

func (s *lLMConfigStoreImpl) GetByType(ctx context.Context, llmType int) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Relation("Upstreams").Relation("Upstreams.HealthState").Where("(type & ?) > 0 and enabled = true", llmType).Limit(1).Scan(ctx)
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
	return configs, total, nil
}

func (s *lLMConfigStoreImpl) IndexWithRepo(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*LLMConfig, int, error) {
	var configs []*LLMConfig
	offset := (page - 1) * per

	query := s.db.Operator.Core.NewSelect().Model(&configs).Relation("Repo").Relation("Upstreams.HealthState").Relation("Upstreams.CircuitState").Limit(per).Offset(offset)
	buildSearchLLMConfigQuery(search, query)
	err := query.Scan(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("select batch llm config with repo, %w", err)
	}
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return configs, total, nil
}

// gatewayModelListRow is the scan target for ListGatewayModels: the catalogue
// view row extended with the window-computed total count of the filtered set.
type gatewayModelListRow struct {
	LLMConfig `bun:",extend"`
	// TotalCount is COUNT(*) OVER(): the number of rows matching the filters
	// regardless of the applied window, computed on the same snapshot as the
	// page rows.
	TotalCount int64 `bun:"total_count"`
	// IsPinned mirrors the pinned preference join, so the row ordering and
	// the pinned flag come from the same statement. The column is only
	// selected when the pinned join is present.
	IsPinned bool `bun:"is_pinned"`
}

func (s *lLMConfigStoreImpl) ListGatewayModels(ctx context.Context, limit, offset int, q *types.GatewayModelListQuery) ([]*LLMConfig, int, error) {
	// The catalogue view (see its migration for the full derivation:
	// classification, tasks, availability, priced) carries every
	// caller-independent rule; this statement adds only the
	// request-dependent filters, the pinned join and the pagination window.
	// Repo is NOT loaded through Relation("Repo"): bun inlines belongs-to
	// relations as a join against the model table, which the view replaces —
	// the page's repositories are fetched separately below with the same
	// semantics.
	// withPinned gates both the pinned join and the pinned ordering: they
	// reference the same alias, and the component only enables them after a
	// successful preference probe (degradation lives there, since an in-store
	// retry would run on a connection poisoned by the failed statement).
	withPinned := q.PinnedUserUUID != ""
	rows := []gatewayModelListRow{}
	query := s.db.Operator.Core.NewSelect().Model(&rows).
		ModelTableExpr("(" + gatewayCatalogQuery + ") AS llm_config").
		ColumnExpr("llm_config.*, COUNT(*) OVER() AS total_count").
		Relation("Upstreams.HealthState").
		Relation("Upstreams.CircuitState")
	if withPinned {
		// The pinned flag rides on the same statement as the pinned-first
		// ordering, so a concurrent pin/unpin cannot produce a page that
		// is ordered by pinned but unflagged (or the reverse).
		query = query.ColumnExpr("pup.pinned_id IS NOT NULL AS is_pinned").
			Join(gatewayPinnedPreferenceJoin,
				q.PinnedUserUUID,
				types.AgentUserPreferenceEntityTypeAgentModel,
				types.AgentUserPreferenceActionPin)
	}
	applyGatewayModelListQuery(query, q, withPinned)
	if limit == 0 {
		// A real LIMIT 0: no rows, but the caller still needs the count
		// (e.g. the virtual auto model reserving the whole first page).
		// Bun omits .Limit(0), so the row query is skipped and only the
		// count runs. There are no page rows to keep a snapshot contract
		// with; the count is simply the freshest value at response time.
		total, err := query.Count(ctx)
		return nil, total, err
	}
	if limit > 0 {
		query = query.Limit(limit).Offset(max(offset, 0))
	}
	if err := query.Scan(ctx); err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		// An empty page carries no window count (empty result set): fall
		// back to a dedicated count query.
		total, err := query.Count(ctx)
		return nil, total, err
	}
	configs := make([]*LLMConfig, 0, len(rows))
	for i := range rows {
		// The pinned flag rides on the list statement when the pinned join is
		// present (see gatewayModelListRow); never persisted.
		rows[i].LLMConfig.IsPinned = rows[i].IsPinned
		configs = append(configs, &rows[i].LLMConfig)
	}
	if err := s.loadGatewayPageRepos(ctx, configs); err != nil {
		return nil, 0, err
	}
	return configs, int(rows[0].TotalCount), nil
}

// loadGatewayPageRepos populates the Repo relation of the page rows the same
// way Relation("Repo") would: one query for the page's repository ids, with
// Repository's soft-delete tag excluding deleted rows, so a soft-deleted or
// missing repository leaves cfg.Repo nil exactly like the relation loading.
func (s *lLMConfigStoreImpl) loadGatewayPageRepos(ctx context.Context, configs []*LLMConfig) error {
	repoIDs := make([]int64, 0, len(configs))
	seen := make(map[int64]struct{}, len(configs))
	for _, cfg := range configs {
		if cfg.RepoID == 0 {
			continue
		}
		if _, dup := seen[cfg.RepoID]; dup {
			continue
		}
		seen[cfg.RepoID] = struct{}{}
		repoIDs = append(repoIDs, cfg.RepoID)
	}
	if len(repoIDs) == 0 {
		return nil
	}
	var repos []*Repository
	err := s.db.Operator.Core.NewSelect().Model(&repos).
		Where("id IN (?)", bun.In(repoIDs)).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("load gateway page repositories, %w", err)
	}
	byID := make(map[int64]*Repository, len(repos))
	for _, repo := range repos {
		byID[repo.ID] = repo
	}
	for _, cfg := range configs {
		if cfg.RepoID != 0 {
			cfg.Repo = byID[cfg.RepoID]
		}
	}
	return nil
}

func (s *lLMConfigStoreImpl) GetByModelName(ctx context.Context, modelName string) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).
		Relation("Repo").
		Relation("Upstreams.HealthState").
		Relation("Upstreams.CircuitState").
		Where("model_name = ?", modelName).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select llm config by model_name %s: %w", modelName, err)
	}
	return &config, nil
}

// GetByModelNameForCaller behaves like GetByModelName but hides whitelist
// models from callers without a matching grant: the whitelist predicate is
// part of the same statement, so model lookup and access check cost one
// query. An unauthorized model surfaces as sql.ErrNoRows, i.e. (nil, nil),
// exactly like a nonexistent one.
func (s *lLMConfigStoreImpl) GetByModelNameForCaller(ctx context.Context, modelName string, caller *types.GatewayWhitelistCaller) (*LLMConfig, error) {
	var config LLMConfig
	query := s.db.Operator.Core.NewSelect().Model(&config).
		Relation("Repo").
		Relation("Upstreams.HealthState").
		Relation("Upstreams.CircuitState").
		Where("model_name = ?", modelName)
	if join, joinArgs := gatewayWhitelistJoin(caller); join != "" {
		query = query.Join(join, joinArgs...)
	}
	if predicate := gatewayWhitelistPredicate(caller); predicate != "" {
		query = query.Where(predicate)
	}
	err := query.Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select llm config by model_name %s: %w", modelName, err)
	}
	return &config, nil
}

// IndexWithRepoForCaller behaves like IndexWithRepo but enforces the model
// access whitelist in the same statement via a LEFT JOIN LATERAL grant probe:
// whitelist rows without a matching grant are hidden, everything else passes.
// Model listing and access check therefore cost one query.
func (s *lLMConfigStoreImpl) IndexWithRepoForCaller(ctx context.Context, per, page int, search *types.SearchLLMConfig, caller *types.GatewayWhitelistCaller) ([]*LLMConfig, int, error) {
	var configs []*LLMConfig
	offset := (page - 1) * per

	query := s.db.Operator.Core.NewSelect().Model(&configs).Relation("Repo").Relation("Upstreams.HealthState").Relation("Upstreams.CircuitState").Limit(per).Offset(offset)
	buildSearchLLMConfigQuery(search, query)
	if join, joinArgs := gatewayWhitelistJoin(caller); join != "" {
		query = query.Join(join, joinArgs...)
	}
	if predicate := gatewayWhitelistPredicate(caller); predicate != "" {
		query = query.Where(predicate)
	}
	err := query.Scan(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("select batch llm config with repo for caller, %w", err)
	}
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return configs, total, nil
}

// GetByRepoPath finds the LLM config whose linked repository has the given
// path (e.g. "ns/name"); returns (nil, nil) when none exists.
func (s *lLMConfigStoreImpl) GetByRepoPath(ctx context.Context, repoPath string) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).
		Join("JOIN repositories r ON r.id = llm_config.repo_id").
		Where("r.path = ?", repoPath).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select llm config by repo path %s: %w", repoPath, err)
	}
	return &config, nil
}

func (s *lLMConfigStoreImpl) GetByID(ctx context.Context, id int64) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Relation("Upstreams").Relation("Upstreams.HealthState").Relation("Upstreams.CircuitState").Where("id = ?", id).Limit(1).Scan(ctx)
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
) {
	// If search option is not provided, skip search filtering
	if search == nil {
		return
	}
	if search.Keyword != "" {
		q.Where("LOWER(llm_config.model_name) LIKE LOWER(?)", "%"+search.Keyword+"%")
	}
	// Filter by Types if provided
	if len(search.Types) > 0 {
		mask := CombineLLMTypes(search.Types)
		if mask > 0 {
			q.Where("(llm_config.type & ?) > 0", mask)
		}
	}
	// Filter by Enabled if provided
	if search.Enabled != nil {
		q.Where("llm_config.enabled = ?", *search.Enabled)
	}
	applyLLMConfigSort(q, search.SortBy, search.SortOrder, search.AvailableFirst)
}

// llmAvailableSQL is the SQL mirror of the component-level availability
// computation (computeUpstreamAvailability + computeLLMAvailability): an LLM
// config is available when it is enabled and (it has no upstream rows at all —
// the Go side returns available for an empty upstream list — or at least one
// enabled upstream that is not explicitly bad: an unhealthy health state or an
// open circuit). Missing state records mean "not yet probed" and do not block
// availability, matching the Go-side "unknown does not block" rule.
const llmAvailableSQL = `(llm_config.enabled AND (
	NOT EXISTS (SELECT 1 FROM ai_gateway_upstreams u0 WHERE u0.llm_config_id = llm_config.id)
	OR EXISTS (
		SELECT 1 FROM ai_gateway_upstreams u
		LEFT JOIN ai_gateway_upstream_health_states h ON h.upstream_id = u.id
		LEFT JOIN ai_gateway_upstream_circuit_states c ON c.upstream_id = u.id
		WHERE u.llm_config_id = llm_config.id
			AND u.enabled
			AND (NOT u.health_check_enabled OR COALESCE(h.health_state, 'unknown') <> 'unhealthy')
			AND (NOT u.circuit_breaker_enabled OR COALESCE(c.circuit_state, 'unknown') <> 'open'))))`

// applyLLMConfigSort applies an allow-listed sort to the query.
// When availableFirst is set (admin LLM config listing only), available configs
// surface first (the boolean availability predicate sorts before the
// allow-listed key) and the rest follows the requested sort.
// Defaults to updated_at DESC so that the most recently changed configs surface first.
//
// The primary key always closes the sort so that the ordering is total. Without a
// unique tie-breaker, rows sharing the sort key (every config the deploy sync
// creates has model_size_b = 0) come back in whatever order the planner produces
// for that query, and pages read separately can order the same tie group
// differently: LIMIT/OFFSET windows then overlap, listing one config twice while
// skipping another.
func applyLLMConfigSort(q *bun.SelectQuery, sortBy, sortOrder string, availableFirst bool) {
	if availableFirst {
		q.OrderExpr(llmAvailableSQL + " DESC")
	}
	column := "llm_config.updated_at"
	switch sortBy {
	case "model_size_b":
		column = "llm_config.model_size_b"
	case "updated_at", "":
		column = "llm_config.updated_at"
	}
	direction := "DESC"
	if strings.EqualFold(sortOrder, "ASC") {
		direction = "ASC"
	}
	q.OrderExpr(column + " " + direction)
	q.OrderExpr("llm_config.id ASC")
}

// gatewayModelPricesQuery is the latest enabled aigateway price per billing
// tier: token kinds (4, 5) take the newest enabled row per resource, the
// multimodal kind (7) takes one row per (resource, unit type, resolution)
// tier, because a resource may carry several concurrently-enabled tiers.
// It is inlined (as a subquery) everywhere prices are consumed — the
// /v1/models is_priced filter below and AccountPriceStore.
// ListGatewayModelPrices — so price-tier logic changes are ordinary code
// diffs instead of view-migration SQL files.
const gatewayModelPricesQuery = `(SELECT DISTINCT ON (resource_id, sku_kind)
	resource_id, sku_kind, sku_price, sku_unit, sku_price_currency,
	sku_cached_price, sku_unit_type, resolution, sku_desc
	FROM account_prices
	WHERE sku_type = 1
		AND sku_kind IN (4, 5)
		AND sku_status = 1
	ORDER BY resource_id, sku_kind, created_at DESC)
UNION ALL
(SELECT DISTINCT ON (resource_id, COALESCE(sku_unit_type, ''), COALESCE(resolution, ''))
	resource_id, sku_kind, sku_price, sku_unit, sku_price_currency,
	sku_cached_price, sku_unit_type, resolution, sku_desc
	FROM account_prices
	WHERE sku_type = 1
		AND sku_kind = 7
		AND sku_status = 1
	ORDER BY resource_id, COALESCE(sku_unit_type, ''), COALESCE(resolution, ''), created_at DESC)`

// gatewayCatalogQuery materializes every caller-independent derivation of
// the aigateway model catalogue (classification, tasks, availability,
// priced). It is inlined as a subquery of the /v1/models listing statement —
// NOT kept as a database view — so model-logic changes (new columns, new
// filters, classification tweaks) are ordinary code diffs in this file
// instead of view-migration SQL files, keeping the evolution reviewable and
// version-controlled in one place.
//
// Classification mirrors llmConfigToModel in aigateway/component: the first
// (lowest-id) csghub-sourced upstream decides; a config whose first csghub
// upstream carries no internal model info produces no model at all and is
// filtered out here (is_convertible), as are disabled configs.
//
// Column semantics (all mirror the Go-side conversion):
//
//	provider_type   external_llm | serverless | inference
//	public_model_id the public model id (keyword search target)
//	csghub_model_id the linked csghub repo path (association)
//	tasks           comma-joined pipeline task string (task filter);
//	                internal models prefer the expanded internal task labels
//	                (mirrors InternalModelInfo.TaskLabels), falling back to
//	                the raw single task
//	svc_type / secure_level / owner_uuid
//	                raw visibility inputs (private internal models are
//	                owner-only; the caller-dependent predicate stays in the
//	                listing query, the subquery takes no parameters)
//	is_available    at least one routable upstream (IsUpstreamUnavailable)
//	is_priced       a configured enabled price (sku_unit > 0) exists for the
//	                metering resource id of this model
const gatewayCatalogQuery = `(SELECT
	lc.*,
	CASE
		WHEN cu.id IS NULL THEN 'external_llm'
		WHEN cu.metadata->'internal_model_info'->>'svc_type' = '3' THEN 'serverless'
		ELSE 'inference'
	END AS provider_type,
	CASE WHEN cu.id IS NULL THEN lc.model_name
		ELSE cu.metadata->'internal_model_info'->>'legacy_model_id' END AS public_model_id,
	cu.metadata->'internal_model_info'->>'csghub_model_id' AS csghub_model_id,
	CASE
		WHEN cu.id IS NOT NULL THEN COALESCE(
			(SELECT string_agg(elem #>> '{}', ',')
				FROM jsonb_array_elements(
					CASE WHEN jsonb_typeof(cu.metadata->'internal_model_info'->'tasks') = 'array'
						THEN cu.metadata->'internal_model_info'->'tasks'
						ELSE '[]'::jsonb END
				) AS elem
				WHERE jsonb_typeof(elem) = 'string'),
			COALESCE(cu.metadata->'internal_model_info'->>'task', ''))
		ELSE (SELECT string_agg(elem #>> '{}', ',')
			FROM jsonb_array_elements(
				CASE WHEN jsonb_typeof(lc.metadata->'tasks') = 'array'
					THEN lc.metadata->'tasks'
					ELSE '[]'::jsonb END
			) AS elem
			WHERE jsonb_typeof(elem) = 'string')
	END AS tasks,
	cu.metadata->'internal_model_info'->>'svc_type'     AS svc_type,
	cu.metadata->'internal_model_info'->>'secure_level' AS secure_level,
	cu.metadata->'internal_model_info'->>'owner_uuid'   AS owner_uuid,
	(cu.id IS NOT NULL) AS is_internal,
	(cu.id IS NULL OR cu.metadata->'internal_model_info' IS NOT NULL) AS is_convertible,
	EXISTS (
		SELECT 1 FROM ai_gateway_upstreams u
		LEFT JOIN ai_gateway_upstream_health_states h ON h.upstream_id = u.id
		LEFT JOIN ai_gateway_upstream_circuit_states c ON c.upstream_id = u.id
		WHERE u.llm_config_id = lc.id AND u.enabled
			AND (NOT u.health_check_enabled OR COALESCE(h.health_state, 'unknown') <> 'unhealthy')
			AND (NOT u.circuit_breaker_enabled OR COALESCE(c.circuit_state, 'unknown') <> 'open')
	) AS is_available,
	EXISTS (
		SELECT 1 FROM (` + gatewayModelPricesQuery + `) vp
		WHERE vp.resource_id = CASE
				WHEN cu.id IS NOT NULL AND cu.metadata->'internal_model_info'->>'svc_type' = '3'
					THEN 'csghub://serverless/' || (cu.metadata->'internal_model_info'->>'csghub_model_id')
				WHEN cu.id IS NULL
					THEN 'thirdparty://' || lc.model_name
				ELSE NULL
			END
		AND vp.sku_unit > 0
	) AS is_priced
FROM llm_configs lc
LEFT JOIN LATERAL (
	SELECT u.* FROM ai_gateway_upstreams u
	WHERE u.llm_config_id = lc.id AND u.source = 'csghub'
	ORDER BY u.id
	LIMIT 1
) cu ON true
WHERE lc.enabled
	AND (cu.id IS NULL OR cu.metadata->'internal_model_info' IS NOT NULL))`

// gatewayPinnedPreferenceJoin exposes, as "pup", the pinned preference row
// matching a config's public model key (optional join, bound to the caller's
// user UUID). The key mirrors pinnedModelKey in aigateway/component: internal
// models use the public model ID (legacy_model_id), external models append
// the provider unless empty.
const gatewayPinnedPreferenceJoin = `LEFT JOIN LATERAL (
	SELECT pup.created_at AS pinned_at, pup.id AS pinned_id
	FROM agent_user_preferences pup
	WHERE pup.user_uuid = ?
		AND pup.entity_type = ?
		AND pup.action = ?
		AND pup.entity_id = CASE
			WHEN is_internal THEN public_model_id
			WHEN COALESCE(llm_config.provider, '') <> '' THEN llm_config.model_name || ':' || llm_config.provider
			ELSE llm_config.model_name
		END
	LIMIT 1
) pup ON true`

// gatewayWhitelistJoin returns a LEFT JOIN LATERAL fragment that probes the
// caller's grant for each candidate row (at most one match), plus its bound
// arguments. Empty for callers that need no join (platform bypass and
// anonymous — a nil caller is anonymous — which carry no identity to match). The LATERAL + LIMIT 1 shape
// mirrors the pinned-preference join and keeps a config with multiple
// matching grants to a single row. The org expansion is an inline subquery
// so a listing or a single-model lookup never needs a second round trip.
func gatewayWhitelistJoin(caller *types.GatewayWhitelistCaller) (string, []any) {
	switch {
	case caller == nil || caller.Bypass || (caller.NamespaceUUID == "" && caller.UserUUID == ""):
		return "", nil
	case caller.NamespaceUUID != "":
		return `LEFT JOIN LATERAL (
			SELECT g.id FROM llm_config_model_grants g
			WHERE g.llm_config_id = llm_config.id
				AND g.namespace_uuid = ?
			LIMIT 1
		) gw ON true`, []any{caller.NamespaceUUID}
	default:
		return `LEFT JOIN LATERAL (
			SELECT g.id FROM llm_config_model_grants g
			WHERE g.llm_config_id = llm_config.id
				AND g.namespace_uuid IN (
					SELECT ns.uuid FROM namespaces ns
					WHERE ns.user_id = (SELECT u.id FROM users u WHERE u.uuid = ?)
						AND ns.deleted_at IS NULL
					UNION
					SELECT ns.uuid FROM members m
					JOIN organizations o ON o.id = m.organization_id AND o.deleted_at IS NULL
					JOIN namespaces ns ON ns.id = o.namespace_id
					WHERE m.user_id = (SELECT u.id FROM users u WHERE u.uuid = ?)
						AND m.deleted_at IS NULL
				)
			LIMIT 1
		) gw ON true`, []any{caller.UserUUID, caller.UserUUID}
	}
}

// gatewayWhitelistPredicate returns the WHERE fragment enforcing the model
// access whitelist, paired with gatewayWhitelistJoin: rows with a joined
// grant (gw.id) pass, whitelist rows without one are hidden, non-whitelist
// rows always pass. Callers must add the join iff the caller carries an
// identity (the gw alias only exists when the join was added).
func gatewayWhitelistPredicate(caller *types.GatewayWhitelistCaller) string {
	switch {
	case caller != nil && caller.Bypass:
		return ""
	case caller == nil || (caller.NamespaceUUID == "" && caller.UserUUID == ""):
		// Anonymous (including a missing caller): fail closed, whitelist
		// models are hidden entirely.
		return "access_policy <> 'whitelist'"
	default:
		return "(access_policy <> 'whitelist' OR gw.id IS NOT NULL)"
	}
}

// applyGatewayModelListQuery adds the request-dependent filters of the
// /v1/models listing on top of the catalogue view's flat columns. The
// caller-independent rules (classification, tasks, availability, priced,
// enabled) live in the view itself. LIKE patterns are bound as parameters,
// so Postgres' default backslash LIKE escape applies to the
// escapeLikePattern-escaped values. withPinned must match whether the
// pinned preference join was added to the query: the pinned ordering
// references the join's alias and cannot be generated without it.
func applyGatewayModelListQuery(q *bun.SelectQuery, query *types.GatewayModelListQuery, withPinned bool) {
	if query.Keyword != "" {
		kw := "%" + escapeLikePattern(strings.ToLower(query.Keyword)) + "%"
		q.Where("LOWER(public_model_id) LIKE ?", kw)
	}
	if len(query.LLMTypes) > 0 {
		requested := make([]string, 0, len(query.LLMTypes))
		for _, t := range query.LLMTypes {
			if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
				requested = append(requested, t)
			}
		}
		if len(requested) > 0 {
			q.Where("provider_type IN (?)", bun.In(requested))
		}
	}
	if task := strings.ToLower(strings.TrimSpace(query.Task)); task != "" {
		pattern := "%," + escapeLikePattern(task) + ",%"
		q.Where("(',' || LOWER(COALESCE(tasks, '')) || ',') LIKE ?", pattern)
	}
	if query.HasAssociatedModel != nil {
		// The repositories EXISTS mirrors Bun's Relation("Repo") loading,
		// which appends the soft-delete predicate for the belongs-to
		// relation: buildExternalModel only writes metadata repo_path when
		// the loaded repo row survives, so a soft-deleted repository means
		// "not associated" exactly like a missing one. Internal and external
		// models branch exactly like the response conversion: internal via
		// csghub_model_id, external via a live linked repository.
		associated := `((is_internal AND COALESCE(csghub_model_id, '') <> '')
			OR (NOT is_internal AND llm_config.repo_id IS NOT NULL AND llm_config.repo_id <> 0
				AND EXISTS (SELECT 1 FROM repositories r WHERE r.id = llm_config.repo_id
					AND r.deleted_at IS NULL AND COALESCE(r.path, '') <> '')))`
		if *query.HasAssociatedModel {
			q.Where(associated)
		} else {
			q.Where("NOT " + associated)
		}
	}
	// Visibility: internal (csghub-sourced) models are public when serverless
	// or explicitly public; other internal models are owner-only. External
	// models are always visible. Caller-dependent, so it cannot live in the
	// catalogue view.
	q.Where(`(provider_type = ?
		OR svc_type = '3'
		OR secure_level = '1'
		OR owner_uuid = ?)`, types.ProviderTypeExternalLLM, query.CallerUUID)
	// Model access whitelist: whitelist-mode models are visible only to
	// callers holding a matching grant. Caller-dependent, so it cannot live
	// in the catalogue view either; the predicate sits before LIMIT/OFFSET so
	// COUNT(*) OVER() keeps the pagination exact.
	if join, joinArgs := gatewayWhitelistJoin(query.WhitelistCaller); join != "" {
		q.Join(join, joinArgs...)
	}
	if predicate := gatewayWhitelistPredicate(query.WhitelistCaller); predicate != "" {
		q.Where(predicate)
	}
	// Only routable models are listed; the view computes the availability
	// from the upstream health/circuit states.
	q.Where("is_available")
	if query.RequireConfiguredPrice {
		// SaaS: unpriced serverless and external models are hidden;
		// inference models meter without prices and pass unconditionally.
		q.Where("(provider_type = ? OR is_priced)", types.ProviderTypeInference)
	}

	if query.PinnedUserUUID != "" && withPinned {
		// Pinned-first ordering, newest pin first — the same order
		// ListByUserAndType (created_at DESC) produces for the in-memory path.
		// Only generated when the pinned join is present: without it the pup
		// alias does not exist and the statement would fail.
		q.OrderExpr("pup.pinned_at DESC NULLS LAST")
		q.OrderExpr("pup.pinned_id DESC NULLS LAST")
	}
	q.OrderExpr("llm_config.model_size_b DESC")
	q.OrderExpr("llm_config.id ASC")
}

// PopulateDerivedFields fills ApiEndpoint, AuthHeader, Provider, and ModelName from the best available upstream.
// Prefers healthy enabled upstreams; falls back to the first enabled upstream.
// If no upstream is available at all, uses upstream[0] and logs a warning.
// Callers that use the LLMConfig to make llm.NewClient() requests should call this
// method after querying from DB to ensure the derived fields are populated from the
// best available upstream.
func (c *LLMConfig) PopulateDerivedFields() {
	if len(c.Upstreams) == 0 {
		return
	}
	// Prefer a healthy, enabled upstream.
	for _, u := range c.Upstreams {
		if u.Enabled && u.URL != "" && u.isHealthy() {
			c.ApiEndpoint = u.URL
			c.AuthHeader = u.AuthHeader
			c.Provider = u.Provider
			c.applyUpstreamModelName(u)
			return
		}
	}
	// Fallback: first enabled upstream.
	for _, u := range c.Upstreams {
		if u.Enabled && u.URL != "" {
			c.ApiEndpoint = u.URL
			c.AuthHeader = u.AuthHeader
			c.Provider = u.Provider
			c.applyUpstreamModelName(u)
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
	c.applyUpstreamModelName(u)
	slog.Error("no enabled upstream available, using upstream[0]",
		"model_name", c.ModelName, "upstream_id", u.ID, "url", u.URL)
}

// applyUpstreamModelName overrides the LLMConfig ModelName with the upstream's
// ModelName when the upstream defines one. This ensures that requests are sent
// with the model name expected by the upstream endpoint.
func (c *LLMConfig) applyUpstreamModelName(u Upstream) {
	if u.ModelName != "" {
		c.ModelName = u.ModelName
	}
}

// isHealthy checks whether this upstream has a healthy health state.
// An upstream is considered healthy if it is not explicitly marked unhealthy.
func (u *Upstream) isHealthy() bool {
	if u.HealthState == nil {
		return true // no health state yet, assume healthy
	}
	return u.HealthState.HealthState != "unhealthy"
}

// IsHealthy reports whether this upstream has a healthy health state.
// An upstream is considered healthy if it is not explicitly marked unhealthy.
func (u *Upstream) IsHealthy() bool {
	return u.isHealthy()
}

// PrimaryEndpoint returns the URL of the best available upstream.
// Call PopulateDerivedFields() first after querying from DB.
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
