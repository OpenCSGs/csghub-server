package database

import (
	"context"
	"fmt"
	"time"

	"opencsg.com/csghub-server/accounting/utils"
	"opencsg.com/csghub-server/common/types"
)

type accountStatementDailySummaryStoreImpl struct {
	db *DB
}

type AccountStatementDailySummaryStore interface {
	UpsertSummary(ctx context.Context, date time.Time) (int64, error)
	// UpsertSummaryAndCheckpoint aggregates one day of account_statements into
	// the daily summary table AND updates the cron_checkpoints row for jobName,
	// both inside a single transaction. Either both commit or both roll back,
	// so a crash never leaves "data written but checkpoint not recorded".
	UpsertSummaryAndCheckpoint(ctx context.Context, date time.Time, jobName string) (int64, error)
	ListByUserAndSku(ctx context.Context, req types.ActStatementsReq) ([]UserSkuStatement, int, error)
}

func NewAccountStatementDailySummaryStore() AccountStatementDailySummaryStore {
	return &accountStatementDailySummaryStoreImpl{
		db: defaultDB,
	}
}

func NewAccountStatementDailySummaryStoreWithDB(db *DB) AccountStatementDailySummaryStore {
	return &accountStatementDailySummaryStoreImpl{
		db: db,
	}
}

// AccountStatementDailySummary is the daily per-(user, sku, scene, instance)
// rollup of account_statements, populated by the nightly cron job. The endpoint
// /api/v1/accounting/statements reads this table instead of the multi-million
// row account_statements table to keep admin monthly reports fast.
type AccountStatementDailySummary struct {
	ID               int64           `bun:",pk,autoincrement" json:"id"`
	BillDate         time.Time       `bun:"type:date,notnull" json:"bill_date"`
	UserUUID         string          `bun:",notnull" json:"user_uuid"`
	SkuID            int64           `bun:",notnull,default:0" json:"sku_id"`
	Scene            types.SceneType `bun:",notnull" json:"scene"`
	CustomerID       string          `bun:",notnull,default:''" json:"customer_id"`
	TotalValue       float64         `bun:",nullzero" json:"total_value"`
	TotalConsumption float64         `bun:",nullzero" json:"total_consumption"`
	TotalCount       int64           `bun:",nullzero" json:"total_count"`
	MinID            int64           `bun:",nullzero" json:"min_id"`
	MinCreatedAt     time.Time       `bun:"type:timestamp,nullzero" json:"min_created_at"`
	times
}

// UpsertSummary aggregates one day of account_statements into the daily summary
// table. It scans only the target day's rows (using the existing created_at
// range index on account_statements) and upserts the results, so re-runs are
// idempotent and reversals (IsCancel) self-heal on the next run. Returns the
// number of summary rows written.
//
// Recharge scenes (ScenePortalCharge=1, SceneCashCharge=3) are excluded —
// this is a consumption report, and recharge rows have positive value (money
// added) which would distort the consumption totals. See
// accounting/utils.IsChargeScene for the source of truth.
func (s *accountStatementDailySummaryStoreImpl) UpsertSummary(ctx context.Context, date time.Time) (int64, error) {
	day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	nextDay := day.AddDate(0, 0, 1)

	// INSERT ... SELECT ... ON CONFLICT, executed as raw SQL because bun's
	// query builder does not cleanly express a SELECT insert source. Use ?
	// placeholders (pgdriver rewrites them to $N) to match the codebase style.
	res, err := s.db.Operator.Core.ExecContext(ctx, `
INSERT INTO account_statement_daily_summaries
  (bill_date, user_uuid, sku_id, scene, customer_id,
   total_value, total_consumption, total_count, min_id, min_created_at)
SELECT
  ?::date AS bill_date, user_uuid, COALESCE(sku_id, 0) AS sku_id, scene, COALESCE(customer_id, '') AS customer_id,
  SUM(value) AS total_value, SUM(consumption) AS total_consumption,
  COUNT(*) AS total_count, MIN(id) AS min_id, MIN(created_at) AS min_created_at
FROM account_statements
WHERE created_at >= ? AND created_at < ?
  AND scene NOT IN (?, ?)
GROUP BY user_uuid, COALESCE(sku_id, 0), scene, COALESCE(customer_id, '')
ON CONFLICT (bill_date, user_uuid, sku_id, scene, customer_id)
DO UPDATE SET
  total_value = EXCLUDED.total_value,
  total_consumption = EXCLUDED.total_consumption,
  total_count = EXCLUDED.total_count,
  min_id = EXCLUDED.min_id,
  min_created_at = EXCLUDED.min_created_at,
  updated_at = current_timestamp;
`, day, day, nextDay, utils.ChargeSceneValues()[0], utils.ChargeSceneValues()[1])
	if err != nil {
		return 0, fmt.Errorf("upsert daily summary for %s, error: %w", day.Format("2006-01-02"), err)
	}

	rows, _ := res.RowsAffected()
	return rows, nil
}

// UpsertSummaryAndCheckpoint runs the daily summary upsert and the checkpoint
// update in one transaction, so they commit atomically (resume-safety).
func (s *accountStatementDailySummaryStoreImpl) UpsertSummaryAndCheckpoint(ctx context.Context, date time.Time, jobName string) (int64, error) {
	day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	nextDay := day.AddDate(0, 0, 1)

	var rowsAffected int64
	err := s.db.RunInTx(ctx, func(ctx context.Context, tx Operator) error {
		res, err := tx.Core.ExecContext(ctx, `
INSERT INTO account_statement_daily_summaries
  (bill_date, user_uuid, sku_id, scene, customer_id,
   total_value, total_consumption, total_count, min_id, min_created_at)
SELECT
  ?::date AS bill_date, user_uuid, COALESCE(sku_id, 0) AS sku_id, scene, COALESCE(customer_id, '') AS customer_id,
  SUM(value) AS total_value, SUM(consumption) AS total_consumption,
  COUNT(*) AS total_count, MIN(id) AS min_id, MIN(created_at) AS min_created_at
FROM account_statements
WHERE created_at >= ? AND created_at < ?
  AND scene NOT IN (?, ?)
GROUP BY user_uuid, COALESCE(sku_id, 0), scene, COALESCE(customer_id, '')
ON CONFLICT (bill_date, user_uuid, sku_id, scene, customer_id)
DO UPDATE SET
  total_value = EXCLUDED.total_value,
  total_consumption = EXCLUDED.total_consumption,
  total_count = EXCLUDED.total_count,
  min_id = EXCLUDED.min_id,
  min_created_at = EXCLUDED.min_created_at,
  updated_at = current_timestamp;
`, day, day, nextDay, utils.ChargeSceneValues()[0], utils.ChargeSceneValues()[1])
		if err != nil {
			return fmt.Errorf("upsert daily summary for %s, error: %w", day.Format("2006-01-02"), err)
		}
		rowsAffected, _ = res.RowsAffected()

		// update checkpoint in the same transaction
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

// ListByUserAndSku mirrors accountStatementStoreImpl.ListStatementByUserAndSku
// (GROUP BY user_uuid, sku_id, scene, customer_id + SUM + COUNT(*) OVER()) but
// runs over the daily summary table, which is orders of magnitude smaller than
// account_statements. The request's month range is mapped to a bill_date range.
func (s *accountStatementDailySummaryStoreImpl) ListByUserAndSku(ctx context.Context, req types.ActStatementsReq) ([]UserSkuStatement, int, error) {
	var results []UserSkuStatement
	baseQuery := s.db.Operator.Core.NewSelect().
		Model((*AccountStatementDailySummary)(nil)).
		Column("user_uuid", "sku_id", "scene", "customer_id").
		ColumnExpr("MIN(min_id) AS id").
		ColumnExpr("MIN(min_created_at) AS created_at").
		ColumnExpr("SUM(total_value) AS total_value").
		ColumnExpr("SUM(total_consumption) AS total_consumption")

	if req.UserUUID != "" {
		baseQuery = baseQuery.Where("user_uuid = ?", req.UserUUID)
	}

	if req.Scene != 0 {
		baseQuery = baseQuery.Where("scene = ?", req.Scene)
	}

	if req.InstanceName != "" {
		baseQuery = baseQuery.Where("customer_id = ?", req.InstanceName)
	}

	if req.StartTime != "" && req.EndTime != "" {
		baseQuery = baseQuery.Where("bill_date >= ? AND bill_date < ?", req.StartTime, req.EndTime)
	}

	baseQuery = baseQuery.Group("user_uuid", "sku_id", "scene", "customer_id")

	selectQuery := s.db.Operator.Core.NewSelect().
		TableExpr("(?) AS grouped", baseQuery).
		Column("id", "user_uuid", "sku_id", "scene", "customer_id", "created_at", "total_value", "total_consumption").
		ColumnExpr("COUNT(*) OVER() AS total_count")

	if req.Per > 0 {
		selectQuery = selectQuery.
			Limit(req.Per).
			Offset((req.Page - 1) * req.Per)
	}

	err := selectQuery.Scan(ctx, &results)
	if err != nil {
		return results, 0, fmt.Errorf("list grouped daily summaries error: %w", err)
	}

	totalCount := 0
	if len(results) > 0 {
		totalCount = results[0].TotalCount
	}

	return results, totalCount, nil
}
