package database

import (
	"context"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AccountStatistics struct {
	ID                int64           `bun:",pk,autoincrement" json:"id"`
	EventDate         time.Time       `bun:"type:date" json:"event_date"`
	UserUUID          string          `bun:",notnull" json:"user_uuid"`
	Scene             types.SceneType `bun:",notnull" json:"scene"`
	CustomerID        string          `bun:",notnull" json:"customer_id"`
	Consumption       float64         `bun:",notnull" json:"consumption"`
	PromptToken       float64         `bun:",notnull" json:"prompt_token"`
	PromptCachedToken float64         `bun:",notnull" json:"prompt_cached_token"`
	CompletionToken   float64         `bun:",notnull" json:"completion_token"`
	Count             float64         `bun:",notnull,default:0" json:"count"`
	TokenID           int64           `bun:",notnull,default:0" json:"token_id"`
	DataType          string          `bun:",notnull,default:''" json:"data_type"`
	Resolution        string          `bun:",notnull,default:''" json:"resolution"`
	Duration          float64         `bun:",notnull,default:0" json:"duration"`
	times
}

type StatisticsTotalResult struct {
	TotalConsumption       float64 `bun:"total_consumption"`
	TotalPromptToken       float64 `bun:"total_prompt_token"`
	TotalPromptCachedToken float64 `bun:"total_prompt_cached_token"`
	TotalCompletionToken   float64 `bun:"total_completion_token"`
	TotalDuration          float64 `bun:"total_duration"`
	TotalCount             float64 `bun:"total_count"`
}

type accountStatisticsStoreImpl struct {
	db *DB
}

type AccountStatisticsStore interface {
	ListByUserIDAndDate(ctx context.Context, req types.AcctBillsReq) (AccountStatisticsRes, error)
	ListStatisticsDetailByUserID(ctx context.Context, req types.AcctBillsDetailReq) (AccountStatisticsDetailRes, error)
	SummaryByUserIDAndDate(ctx context.Context, req types.AcctBillsReq) (AccountStatisticsSummaryRes, error)
}

func NewAccountStatisticsStore() AccountStatisticsStore {
	return &accountStatisticsStoreImpl{
		db: defaultDB,
	}
}

func NewAccountStatisticsStoreWithDB(db *DB) AccountStatisticsStore {
	return &accountStatisticsStoreImpl{
		db: db,
	}
}

type AccountStatisticsRes struct {
	Data []types.ITEM `json:"data"`
	types.AcctSummary
}

type AccountStatisticsDetailRes struct {
	Data  []AccountStatistics `json:"data"`
	Total int                 `json:"total"`
}

type SceneSummary struct {
	Scene                  types.SceneType `json:"scene"`
	TotalConsumption       float64         `json:"total_consumption"`
	TotalPromptToken       float64         `json:"total_prompt_token"`
	TotalPromptCachedToken float64         `json:"total_prompt_cached_token"`
	TotalCompletionToken   float64         `json:"total_completion_token"`
	TotalCount             float64         `json:"total_count"`
	TotalDuration          float64         `json:"total_duration"`
}

type AccountStatisticsSummaryRes struct {
	Data []SceneSummary `json:"data"`
}

func (s *accountStatisticsStoreImpl) ListByUserIDAndDate(ctx context.Context, req types.AcctBillsReq) (AccountStatisticsRes, error) {
	var stats []AccountStatistics
	var res []types.ITEM
	q := s.db.Operator.Core.NewSelect().Model(&stats).
		ColumnExpr("customer_id as instance_name").
		ColumnExpr("data_type").
		ColumnExpr("resolution").
		ColumnExpr("sum(consumption) as consumption").
		ColumnExpr("sum(prompt_token) as prompt_token").
		ColumnExpr("sum(prompt_cached_token) as prompt_cached_token").
		ColumnExpr("sum(completion_token) as completion_token").
		ColumnExpr("sum(duration) as duration").
		ColumnExpr("sum(count) as count")

	switch req.Scene {
	case types.SceneEvaluation:
		q = q.ColumnExpr("d.submit_time as created_at")
		q = q.Join("LEFT JOIN argo_workflows d ON customer_id = d.task_id")
	case types.SceneSpace, types.SceneModelInference, types.SceneModelFinetune:
		q = q.ColumnExpr("d.created_at as created_at")
		q = q.Join("LEFT JOIN deploys d ON customer_id = d.svc_name")
	}

	q = q.Where("account_statistics.event_date >= ? and account_statistics.event_date <= ?", req.StartDate, req.EndDate).
		Where("account_statistics.user_uuid = ?", req.TargetUUID).
		Where("account_statistics.scene = ?", req.Scene)

	if len(req.InstanceName) > 0 {
		q = q.Where("LOWER(customer_id) LIKE ?", "%"+strings.ToLower(req.InstanceName)+"%")
	}
	switch req.Scene {
	case types.SceneEvaluation:
		q = q.Group("customer_id", "data_type", "resolution", "d.submit_time")
	case types.SceneSpace, types.SceneModelInference, types.SceneModelFinetune:
		q = q.Group("customer_id", "data_type", "resolution", "d.created_at")
	default:
		q = q.Group("customer_id", "data_type", "resolution")
	}

	count, err := q.Count(ctx)
	if err != nil {
		return AccountStatisticsRes{}, errorx.HandleDBError(err, nil)
	}

	var totalResult StatisticsTotalResult

	err = s.db.Operator.Core.NewSelect().With("grouped_items", q).TableExpr("grouped_items").
		ColumnExpr("SUM(consumption) as total_consumption").
		ColumnExpr("SUM(prompt_token) as total_prompt_token").
		ColumnExpr("SUM(prompt_cached_token) as total_prompt_cached_token").
		ColumnExpr("SUM(completion_token) as total_completion_token").
		ColumnExpr("SUM(duration) as total_duration").
		ColumnExpr("SUM(count) as total_count").
		Scan(ctx, &totalResult)
	if err != nil {
		return AccountStatisticsRes{}, errorx.HandleDBError(err, nil)
	}

	// Pagination: never touch q (the CTE source); clone it.
	pageQ := q.Clone()

	switch req.Scene {
	case types.SceneEvaluation:
		pageQ = pageQ.Order("d.submit_time DESC NULLS LAST")
	case types.SceneSpace, types.SceneModelInference, types.SceneModelFinetune:
		pageQ = pageQ.Order("d.created_at DESC NULLS LAST")
	}
	pageQ = pageQ.Order("customer_id")
	err = pageQ.Limit(req.Per).Offset((req.Page-1)*req.Per).Scan(ctx, &res)
	if err != nil {
		return AccountStatisticsRes{}, errorx.HandleDBError(err, nil)
	}
	return AccountStatisticsRes{
		Data: res,
		AcctSummary: types.AcctSummary{
			Total:                  count,
			TotalConsumption:       totalResult.TotalConsumption,
			TotalPromptToken:       totalResult.TotalPromptToken,
			TotalPromptCachedToken: totalResult.TotalPromptCachedToken,
			TotalCompletionToken:   totalResult.TotalCompletionToken,
			TotalDuration:          totalResult.TotalDuration,
			TotalCount:             totalResult.TotalCount,
		},
	}, err
}

func (s *accountStatisticsStoreImpl) ListStatisticsDetailByUserID(ctx context.Context, req types.AcctBillsDetailReq) (AccountStatisticsDetailRes, error) {
	var stats []AccountStatistics

	q := s.db.Operator.Core.NewSelect().Model(&stats).
		Where("event_date >= ? and event_date <= ? and user_uuid = ? and scene = ?", req.StartDate, req.EndDate, req.TargetUUID, req.Scene)

	if req.InstanceName != "" {
		q = q.Where("customer_id = ?", req.InstanceName)
	}

	count, err := q.Count(ctx)
	if err != nil {
		return AccountStatisticsDetailRes{}, errorx.HandleDBError(err, nil)
	}

	err = q.Order("customer_id").Order("event_date DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Scan(ctx, &stats)
	if err != nil {
		return AccountStatisticsDetailRes{}, errorx.HandleDBError(err, nil)
	}

	return AccountStatisticsDetailRes{
		Data:  stats,
		Total: count,
	}, nil
}

func (s *accountStatisticsStoreImpl) SummaryByUserIDAndDate(ctx context.Context, req types.AcctBillsReq) (AccountStatisticsSummaryRes, error) {
	var summaries []SceneSummary

	q := s.db.Operator.Core.NewSelect().Model((*AccountStatistics)(nil)).
		ColumnExpr("scene").
		ColumnExpr("COALESCE(SUM(consumption), 0) as total_consumption").
		ColumnExpr("COALESCE(SUM(prompt_token), 0) as total_prompt_token").
		ColumnExpr("COALESCE(SUM(prompt_cached_token), 0) as total_prompt_cached_token").
		ColumnExpr("COALESCE(SUM(completion_token), 0) as total_completion_token").
		ColumnExpr("COALESCE(SUM(count), 0) as total_count").
		ColumnExpr("COALESCE(SUM(duration), 0) as total_duration").
		Where("event_date >= ? and event_date <= ?", req.StartDate, req.EndDate).
		Where("user_uuid = ?", req.TargetUUID)

	if req.Scene > 0 {
		q = q.Where("scene = ?", req.Scene)
	}

	q = q.Group("scene")

	err := q.Scan(ctx, &summaries)
	if err != nil {
		return AccountStatisticsSummaryRes{}, errorx.HandleDBError(err, nil)
	}

	return AccountStatisticsSummaryRes{
		Data: summaries,
	}, nil
}
