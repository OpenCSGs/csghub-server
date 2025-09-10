package database

import (
	"context"
	"fmt"
	"opencsg.com/csghub-server/common/types"
	"time"
)

type statSnapImpl struct {
	db *DB
}

type StatSnapStore interface {
	Add(ctx context.Context, snapshot *StatSnapshot) error
	Get(ctx context.Context, req types.StatSnapshotReq) (StatSnapshot, error)

	QueryCumulativeCountByTime(
		ctx context.Context,
		table string,
		timeField string,
		granularity TimeGranularity,
		periodCount int, // e.g. 12 months, 30 days, 24 hours
	) ([]TimeSeriesPoint, error)
}

func NewStatSnapStore() StatSnapStore {
	return &statSnapImpl{
		db: defaultDB,
	}
}

func NewStatSnapStoreWithDB(db *DB) StatSnapStore {
	return &statSnapImpl{
		db: db,
	}
}

type Trend map[string]int

type StatSnapshot struct {
	ID           int64                `bun:",pk,autoincrement" json:"id"`
	TargetType   types.StatTargetType `bun:",notnull" json:"target_type"`
	DateType     types.StatDateType   `bun:",notnull" json:"date_type"`
	SnapshotDate string               `bun:",notnull" json:"snapshot_date"`
	TrendData    Trend                `bun:",type:jsonb" json:"trend_data"`
	TotalCount   int                  `bun:",notnull" json:"total_count"`
	NewCount     int                  `bun:",notnull" json:"new_count"`
	times
}

// UNIQUE(snapshot_date, target_type, data_type)

func (s *statSnapImpl) Add(ctx context.Context, snapshot *StatSnapshot) error {
	_, err := s.db.Operator.Core.NewInsert().
		Model(snapshot).
		On("CONFLICT (snapshot_date, target_type, date_type) DO UPDATE").
		Set("trend_data = EXCLUDED.trend_data").
		Set("total_count = EXCLUDED.total_count").
		Set("new_count = EXCLUDED.new_count").
		Exec(ctx)

	return err
}

func (s *statSnapImpl) Get(ctx context.Context, req types.StatSnapshotReq) (StatSnapshot, error) {
	var snapshot StatSnapshot

	err := s.db.Operator.Core.NewSelect().
		Model(&snapshot).
		Where("target_type = ?", req.TargetType).
		Where("date_type = ?", req.DateType).
		Where("snapshot_date = ?", req.SnapshotDate).
		Limit(1).
		Scan(ctx)

	return snapshot, err
}

type TimeSeriesPoint struct {
	Period     time.Time `json:"period"`      // Time point (e.g. 2025-04-30 23:59:59)
	TotalCount int       `json:"total_count"` // Cumulative count
}

type TimeGranularity string

const (
	ByMonth TimeGranularity = "month"
	ByDay   TimeGranularity = "day"
	ByHour  TimeGranularity = "hour"
)

func (s *statSnapImpl) QueryCumulativeCountByTime(
	ctx context.Context,
	table string,
	timeField string,
	granularity TimeGranularity,
	periodCount int, // e.g. 12 months, 30 days, 24 hours
) ([]TimeSeriesPoint, error) {

	var (
		stepInterval string // e.g. '1 month'
		truncUnit    string // e.g. 'month'
		offsetExpr   string // how to get end of bucket
	)

	switch granularity {
	case ByMonth:
		truncUnit = "month"
		stepInterval = "1 month"
		offsetExpr = " + interval '1 month - 1 second'"
	case ByDay:
		truncUnit = "day"
		stepInterval = "1 day"
		offsetExpr = " + interval '1 day - 1 second'"
	case ByHour:
		truncUnit = "hour"
		stepInterval = "1 hour"
		offsetExpr = " + interval '1 hour - 1 second'"
	default:
		return nil, fmt.Errorf("unsupported granularity: %s", granularity)
	}
	sql := fmt.Sprintf(`
	WITH time_points AS (
		SELECT 
			generate_series(
				date_trunc('%[1]s', now()) - interval '%[2]d %[1]s',
				date_trunc('%[1]s', now()),
				interval '%[3]s'
			)%[4]s AS period
	),
	counts AS (
		SELECT 
			tp.period,
			(
				SELECT COUNT(*) FROM %[5]s u
				WHERE u.%[6]s <= tp.period
			) AS total_count
		FROM time_points tp
	)
	SELECT * FROM counts ORDER BY period;
`, truncUnit, periodCount-1, stepInterval, offsetExpr, table, timeField)

	var result []TimeSeriesPoint
	if err := s.db.Core.NewRaw(sql).Scan(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}
