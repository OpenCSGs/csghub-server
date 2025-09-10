//go:build saas || ee

package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"log/slog"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"strconv"
	"time"
)

func (sc *statComponentImpl) GetStatSnap(ctx context.Context, req types.StatSnapshotReq) (*types.StatSnapshotResp, error) {
	req.SnapshotDate = time.Now().Format("2006-01-02")
	statSnapshot, err := sc.statSnapStore.Get(ctx, req)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &types.StatSnapshotResp{}, nil
		}
		return nil, fmt.Errorf("failed to get stat snapshot:%w", err)
	}

	resp := &types.StatSnapshotResp{
		ID:           statSnapshot.ID,
		TargetType:   string(statSnapshot.TargetType),
		DateType:     string(statSnapshot.DateType),
		SnapshotDate: statSnapshot.SnapshotDate,
		TrendData:    statSnapshot.TrendData,
		TotalCount:   statSnapshot.TotalCount,
		NewCount:     statSnapshot.NewCount,
	}
	return resp, err
}

type SnapTask struct {
	TargetType   types.StatTargetType
	TimeField    string
	ByTime       database.TimeGranularity
	PeriodCount  int
	StatDateType types.StatDateType
}

func (sc *statComponentImpl) MakeStatSnap(ctx context.Context) error {
	var tasks []SnapTask
	for _, targetType := range types.AllStatTargetTypes {
		// Other TimeField fields may be used and need to be compatible
		tasks = append(tasks,
			SnapTask{targetType, "created_at", database.ByMonth, 12, types.StatDateYear},
			SnapTask{targetType, "created_at", database.ByDay, 30, types.StatDateMonth},
			SnapTask{targetType, "created_at", database.ByDay, 7, types.StatDateWeek},
			SnapTask{targetType, "created_at", database.ByHour, 24, types.StatDateDay},
		)
	}

	g, ctx := errgroup.WithContext(ctx)
	sem := semaphore.NewWeighted(5)

	for _, t := range tasks {
		t := t
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)
			err := sc.buildStatSnapData(ctx, t)
			if err != nil {
				slog.Error("failed to make stat snapshot",
					slog.Any("error", err),
					slog.String("table", string(t.TargetType)),
					slog.String("by_time", string(t.ByTime)),
					slog.Int("period_count", t.PeriodCount),
				)
			}
			return err
		})
	}
	return g.Wait()
}

func (sc *statComponentImpl) buildStatSnapData(ctx context.Context, t SnapTask) error {
	nowDate := time.Now().Format("2006-01-02")
	seriesPoints, err := sc.statSnapStore.QueryCumulativeCountByTime(ctx, string(t.TargetType), t.TimeField, t.ByTime, t.PeriodCount)
	if err != nil {
		return err
	}
	trend := make(database.Trend)
	if len(seriesPoints) == 0 {
		return fmt.Errorf("failed to build stat snapshot")
	}
	for _, p := range seriesPoints {
		key := p.Period.Format("2006-01-02 15:04:05")
		trend[key] = p.TotalCount
	}
	totalCount := seriesPoints[len(seriesPoints)-1].TotalCount
	newCount := totalCount - seriesPoints[0].TotalCount
	if newCount < 0 {
		newCount = 0
	}
	statSnapshotData := &database.StatSnapshot{
		TargetType:   t.TargetType,
		DateType:     t.StatDateType,
		SnapshotDate: nowDate,
		TrendData:    trend,
		TotalCount:   totalCount,
		NewCount:     newCount,
	}

	err = sc.statSnapStore.Add(ctx, statSnapshotData)
	if err != nil {
		return err
	}

	return nil
}

func (sc *statComponentImpl) StatRunningDeploys(ctx context.Context) (map[int]*types.StatRunningDeploy, error) {
	res := make(map[int]*types.StatRunningDeploy)

	allRunningDeploys, err := sc.deployTaskStore.ListAllRunningDeploys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all running deploys: %w", err)
	}

	for _, deploy := range allRunningDeploys {
		stat, ok := res[deploy.Type]
		if !ok {
			stat = &types.StatRunningDeploy{}
			res[deploy.Type] = stat
		}
		stat.DeployNum += 1

		var hw types.HardWare
		if err := json.Unmarshal([]byte(deploy.Hardware), &hw); err != nil {
			slog.Warn("failed to unmarshal hardware", slog.Any("hardware", deploy.Hardware))
			continue
		}

		parseAndAdd := func(numStr string) int {
			n, err := strconv.Atoi(numStr)
			if err != nil {
				return 0
			}
			return n
		}

		stat.CPUNum += parseAndAdd(hw.Cpu.Num)
		stat.GPUNum += parseAndAdd(hw.Gpu.Num)
		stat.NpuNum += parseAndAdd(hw.Npu.Num)
		stat.GcuNum += parseAndAdd(hw.Gcu.Num)
		stat.MluNum += parseAndAdd(hw.Mlu.Num)
		stat.DcuNum += parseAndAdd(hw.Dcu.Num)
		stat.GPGpuNum += parseAndAdd(hw.GPGpu.Num)
	}

	return res, nil
}
