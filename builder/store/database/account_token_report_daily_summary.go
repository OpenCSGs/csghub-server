package database

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// AccountTokenReportDailySummary is the daily fact rollup for the admin token
// report (issue #3404). It aggregates account_statements rows of the
// serverless token scenes (SceneModelServerless/SceneMultiModalServerless) —
// revenue and cost are already frozen on the statements at charge time, so
// the nightly job only sums them. Read by /accounting/token-report instead of
// scanning the multi-million row account_statements table.
type AccountTokenReportDailySummary struct {
	ID                int64           `bun:",pk,autoincrement" json:"id"`
	StatDate          time.Time       `bun:"type:date,notnull" json:"stat_date"`
	NsUUID            string          `bun:",notnull" json:"ns_uuid"`
	TokenID           int64           `bun:",notnull,default:0" json:"token_id"`
	ResourceID        string          `bun:",notnull,default:''" json:"resource_id"`
	Provider          string          `bun:",notnull,default:''" json:"provider"`
	UpstreamID        int64           `bun:",notnull,default:0" json:"upstream_id"`
	Scene             types.SceneType `bun:",notnull" json:"scene"`
	CallCount         int64           `bun:",notnull,default:0" json:"call_count"`
	PromptToken       float64         `bun:",notnull,default:0" json:"prompt_token"`
	PromptCachedToken float64         `bun:",notnull,default:0" json:"prompt_cached_token"`
	CompletionToken   float64         `bun:",notnull,default:0" json:"completion_token"`
	Duration          float64         `bun:",notnull,default:0" json:"duration"`
	DataType          string          `bun:",notnull,default:''" json:"data_type"`
	Resolution        string          `bun:",notnull,default:''" json:"resolution"`
	RevenueAmount     float64         `bun:",notnull,default:0" json:"revenue_amount"`
	CostAmount        float64         `bun:",notnull,default:0" json:"cost_amount"`
	times
}

// TokenReportRow is one report-grain group (date × namespace × token × model ×
// provider) of the rollup; storage-only dims (upstream_id, scene, data_type,
// resolution) are collapsed. Money is cents.
type TokenReportRow struct {
	StatDate          time.Time `json:"stat_date"`
	NsUUID            string    `json:"ns_uuid"`
	TokenID           int64     `json:"token_id"`
	ResourceID        string    `json:"resource_id"`
	Provider          string    `json:"provider"`
	CallCount         int64     `json:"call_count"`
	PromptToken       float64   `json:"prompt_token"`
	PromptCachedToken float64   `json:"prompt_cached_token"`
	CompletionToken   float64   `json:"completion_token"`
	RevenueAmount     float64   `json:"revenue_amount"`
	CostAmount        float64   `json:"cost_amount"`
	TotalCount        int       `bun:"total_count" json:"-"`
}

// TokenReportTotals is the 合计 row over the same filters (no grouping).
type TokenReportTotals struct {
	CallCount         int64   `json:"call_count"`
	PromptToken       float64 `json:"prompt_token"`
	PromptCachedToken float64 `json:"prompt_cached_token"`
	CompletionToken   float64 `json:"completion_token"`
	RevenueAmount     float64 `json:"revenue_amount"`
	CostAmount        float64 `json:"cost_amount"`
}

type AccountTokenReportDailySummaryStore interface {
	// UpsertSummary aggregates one day of account_statements (serverless token
	// scenes) into the rollup table. Re-runs are idempotent (overwrite-per-day).
	UpsertSummary(ctx context.Context, date time.Time) (int64, error)
	// UpsertSummaryAndCheckpoint runs the daily rollup upsert and the
	// cron_checkpoints update in one transaction, so a crash never leaves
	// "data written but checkpoint not recorded".
	UpsertSummaryAndCheckpoint(ctx context.Context, date time.Time, jobName string) (int64, error)
	// ListReport groups the rollup at report grain with pagination.
	ListReport(ctx context.Context, req types.TokenReportReq) ([]TokenReportRow, int, error)
	// SumReport sums the rollup over the same filters for the totals row.
	SumReport(ctx context.Context, req types.TokenReportReq) (TokenReportTotals, error)
}

type accountTokenReportDailySummaryStoreImpl struct {
	db *DB
}

func NewAccountTokenReportDailySummaryStore() AccountTokenReportDailySummaryStore {
	return &accountTokenReportDailySummaryStoreImpl{
		db: defaultDB,
	}
}

func NewAccountTokenReportDailySummaryStoreWithDB(db *DB) AccountTokenReportDailySummaryStore {
	return &accountTokenReportDailySummaryStoreImpl{
		db: db,
	}
}

const tokenReportSummaryUpsertSQL = `
INSERT INTO account_token_report_daily_summaries
  (stat_date, ns_uuid, token_id, resource_id, provider, upstream_id, scene, data_type, resolution,
   call_count, prompt_token, prompt_cached_token, completion_token, duration, revenue_amount, cost_amount)
SELECT
  ?::date AS stat_date, user_uuid AS ns_uuid, COALESCE(token_id, 0) AS token_id,
  COALESCE(resource_id, '') AS resource_id, COALESCE(provider, '') AS provider,
  COALESCE(upstream_id, 0) AS upstream_id, scene,
  COALESCE(data_type, '') AS data_type, COALESCE(resolution, '') AS resolution,
  COUNT(*) AS call_count,
  COALESCE(SUM(prompt_token), 0) AS prompt_token,
  COALESCE(SUM(prompt_cached_token), 0) AS prompt_cached_token,
  COALESCE(SUM(completion_token), 0) AS completion_token,
  COALESCE(SUM(duration), 0) AS duration,
  COALESCE(SUM(value), 0) AS revenue_amount,
  COALESCE(SUM(cost_amount), 0) AS cost_amount
FROM account_statements
WHERE created_at >= ? AND created_at < ?
  AND scene IN (?, ?)
GROUP BY user_uuid, COALESCE(token_id, 0), COALESCE(resource_id, ''), COALESCE(provider, ''),
  COALESCE(upstream_id, 0), scene, COALESCE(data_type, ''), COALESCE(resolution, '')
ON CONFLICT (stat_date, ns_uuid, token_id, resource_id, provider, upstream_id, scene, data_type, resolution)
DO UPDATE SET
  call_count = EXCLUDED.call_count,
  prompt_token = EXCLUDED.prompt_token,
  prompt_cached_token = EXCLUDED.prompt_cached_token,
  completion_token = EXCLUDED.completion_token,
  duration = EXCLUDED.duration,
  revenue_amount = EXCLUDED.revenue_amount,
  cost_amount = EXCLUDED.cost_amount,
  updated_at = current_timestamp;
`

func tokenReportDayRange(date time.Time) (time.Time, time.Time) {
	day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	return day, day.AddDate(0, 0, 1)
}

func (s *accountTokenReportDailySummaryStoreImpl) UpsertSummary(ctx context.Context, date time.Time) (int64, error) {
	day, nextDay := tokenReportDayRange(date)
	res, err := s.db.Operator.Core.ExecContext(ctx, tokenReportSummaryUpsertSQL,
		day.Format("2006-01-02"), day, nextDay, types.SceneModelServerless, types.SceneMultiModalServerless)
	if err != nil {
		return 0, fmt.Errorf("upsert token report daily summary for %s, error: %w", day.Format("2006-01-02"), err)
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

func (s *accountTokenReportDailySummaryStoreImpl) UpsertSummaryAndCheckpoint(ctx context.Context, date time.Time, jobName string) (int64, error) {
	day, nextDay := tokenReportDayRange(date)

	var rowsAffected int64
	err := s.db.RunInTx(ctx, func(ctx context.Context, tx Operator) error {
		res, err := tx.Core.ExecContext(ctx, tokenReportSummaryUpsertSQL,
			day.Format("2006-01-02"), day, nextDay, types.SceneModelServerless, types.SceneMultiModalServerless)
		if err != nil {
			return fmt.Errorf("upsert token report daily summary for %s, error: %w", day.Format("2006-01-02"), err)
		}
		rowsAffected, _ = res.RowsAffected()

		cp := CronCheckpoint{
			JobName:  jobName,
			LastDate: day,
		}
		_, err = tx.Core.NewInsert().Model(&cp).
			On("CONFLICT (job_name) DO UPDATE").
			Set("last_date = EXCLUDED.last_date").
			Set("updated_at = current_timestamp").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("save checkpoint for %s, error: %w", jobName, err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return rowsAffected, nil
}

// applyTokenReportFilters adds the shared WHERE clauses of ListReport/SumReport.
func applyTokenReportFilters(q *bun.SelectQuery, req types.TokenReportReq) *bun.SelectQuery {
	if req.NsUUID != "" {
		q = q.Where("ns_uuid = ?", req.NsUUID)
	}
	if req.TokenID > 0 {
		q = q.Where("token_id = ?", req.TokenID)
	}
	if req.ResourceID != "" {
		q = q.Where("resource_id = ?", req.ResourceID)
	}
	if req.Provider != "" {
		q = q.Where("provider = ?", req.Provider)
	}
	if req.StartTime != "" && req.EndTime != "" {
		q = q.Where("stat_date >= ? AND stat_date < ?", req.StartTime, req.EndTime)
	}
	return q
}

func (s *accountTokenReportDailySummaryStoreImpl) ListReport(ctx context.Context, req types.TokenReportReq) ([]TokenReportRow, int, error) {
	var results []TokenReportRow
	baseQuery := s.db.Operator.Core.NewSelect().
		Model((*AccountTokenReportDailySummary)(nil)).
		Column("stat_date", "ns_uuid", "token_id", "resource_id", "provider").
		ColumnExpr("SUM(call_count) AS call_count").
		ColumnExpr("SUM(prompt_token) AS prompt_token").
		ColumnExpr("SUM(prompt_cached_token) AS prompt_cached_token").
		ColumnExpr("SUM(completion_token) AS completion_token").
		ColumnExpr("SUM(revenue_amount) AS revenue_amount").
		ColumnExpr("SUM(cost_amount) AS cost_amount")
	baseQuery = applyTokenReportFilters(baseQuery, req)
	baseQuery = baseQuery.Group("stat_date", "ns_uuid", "token_id", "resource_id", "provider")

	selectQuery := s.db.Operator.Core.NewSelect().
		TableExpr("(?) AS grouped", baseQuery).
		Column("stat_date", "ns_uuid", "token_id", "resource_id", "provider",
			"call_count", "prompt_token", "prompt_cached_token", "completion_token",
			"revenue_amount", "cost_amount").
		ColumnExpr("COUNT(*) OVER() AS total_count").
		OrderExpr("stat_date DESC, ns_uuid ASC, resource_id ASC")

	if req.Per > 0 {
		selectQuery = selectQuery.
			Limit(req.Per).
			Offset((req.Page - 1) * req.Per)
	}

	err := selectQuery.Scan(ctx, &results)
	if err != nil {
		return results, 0, fmt.Errorf("list token report grouped summaries error: %w", err)
	}

	totalCount := 0
	if len(results) > 0 {
		totalCount = results[0].TotalCount
	}
	return results, totalCount, nil
}

func (s *accountTokenReportDailySummaryStoreImpl) SumReport(ctx context.Context, req types.TokenReportReq) (TokenReportTotals, error) {
	var totals TokenReportTotals
	q := s.db.Operator.Core.NewSelect().
		Model((*AccountTokenReportDailySummary)(nil)).
		ColumnExpr("COALESCE(SUM(call_count), 0) AS call_count").
		ColumnExpr("COALESCE(SUM(prompt_token), 0) AS prompt_token").
		ColumnExpr("COALESCE(SUM(prompt_cached_token), 0) AS prompt_cached_token").
		ColumnExpr("COALESCE(SUM(completion_token), 0) AS completion_token").
		ColumnExpr("COALESCE(SUM(revenue_amount), 0) AS revenue_amount").
		ColumnExpr("COALESCE(SUM(cost_amount), 0) AS cost_amount")
	q = applyTokenReportFilters(q, req)
	err := q.Scan(ctx, &totals)
	if err != nil {
		return totals, fmt.Errorf("sum token report summaries error: %w", err)
	}
	return totals, nil
}
