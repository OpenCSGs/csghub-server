//go:build saas

package component

import (
	"context"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"testing"
	"time"
)

func TestStatComponent_GetStatSnap(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestStatComponent(ctx, t)

	date := time.Now().Format("2006-01-02")
	req := types.StatSnapshotReq{
		TargetType:   types.StatTargetType("user"),
		DateType:     types.StatDateType("year"),
		SnapshotDate: date,
	}
	expectedSnapshot := database.StatSnapshot{
		ID:           1,
		TargetType:   "user",
		DateType:     "year",
		SnapshotDate: date,
		TrendData:    map[string]int{"1": 100},
		TotalCount:   200,
		NewCount:     50,
	}

	sc.mocks.stores.StatMock().EXPECT().
		Get(ctx, mock.MatchedBy(func(input types.StatSnapshotReq) bool {
			return input.TargetType == req.TargetType &&
				input.DateType == req.DateType &&
				input.SnapshotDate != ""
		})).
		Return(expectedSnapshot, nil)

	resp, err := sc.GetStatSnap(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, expectedSnapshot.ID, resp.ID)
	require.Equal(t, expectedSnapshot.TargetType, types.StatTargetType(resp.TargetType))
	require.Equal(t, expectedSnapshot.DateType, types.StatDateType(resp.DateType))
	require.Equal(t, expectedSnapshot.SnapshotDate, resp.SnapshotDate)
	require.Equal(t, expectedSnapshot.TotalCount, resp.TotalCount)
	require.Equal(t, expectedSnapshot.NewCount, resp.NewCount)
}

func TestStatComponent_StatRunningDeploys(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestStatComponent(ctx, t)

	mockDeploys := []database.Deploy{
		{
			Type:     1,
			Hardware: `{"cpu": {"num": "2"}, "gpu": {"num": "1"}}`,
		},
		{
			Type:     1,
			Hardware: `{"cpu": {"num": "4"}, "gpu": {"num": "0"}}`,
		},
		{
			Type:     2,
			Hardware: `{"cpu": {"num": "1"}, "gpu": {"num": "1"}}`,
		},
		{
			Type:     2,
			Hardware: `invalid-json`,
		},
	}

	sc.mocks.stores.DeployTaskMock().
		EXPECT().
		ListAllRunningDeploys(ctx).
		Return(mockDeploys, nil).
		Once()

	res, err := sc.StatRunningDeploys(ctx)

	require.NoError(t, err)

	require.NotNil(t, res)
	require.Len(t, res, 2)

	require.Equal(t, 2, res[1].DeployNum)
	require.Equal(t, 6, res[1].CPUNum) // 2 + 4
	require.Equal(t, 1, res[1].GPUNum) // 1 + 0

	require.Equal(t, 2, res[2].DeployNum)
	require.Equal(t, 1, res[2].CPUNum)
	require.Equal(t, 1, res[2].GPUNum)
}
