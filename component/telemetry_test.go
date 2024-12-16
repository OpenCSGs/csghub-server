package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/telemetry"
)

func TestTelemetryComponent_SaveUsageData(t *testing.T) {
	ctx := context.TODO()
	tc := initializeTestTelemetryComponent(ctx, t)

	tc.mocks.stores.TelemetryMock().EXPECT().Save(ctx, &database.Telemetry{
		UUID:     "uid",
		Version:  "v1",
		Licensee: telemetry.Licensee{},
		Settings: telemetry.Settings{},
		Counts:   telemetry.Counts{},
	}).Return(nil)

	err := tc.SaveUsageData(ctx, telemetry.Usage{
		UUID:    "uid",
		Version: "v1",
	})
	require.Nil(t, err)

}

func TestTelemetryComponent_GenUsageData(t *testing.T) {
	ctx := context.TODO()
	tc := initializeTestTelemetryComponent(ctx, t)

	tc.mocks.stores.UserMock().EXPECT().CountUsers(ctx).Return(100, nil)
	tc.mocks.stores.RepoMock().EXPECT().CountByRepoType(ctx, types.ModelRepo).Return(10, nil)
	tc.mocks.stores.RepoMock().EXPECT().CountByRepoType(ctx, types.DatasetRepo).Return(20, nil)
	tc.mocks.stores.RepoMock().EXPECT().CountByRepoType(ctx, types.CodeRepo).Return(30, nil)
	tc.mocks.stores.RepoMock().EXPECT().CountByRepoType(ctx, types.SpaceRepo).Return(40, nil)

	data, err := tc.GenUsageData(ctx)
	require.Nil(t, err)

	require.Equal(t, 100, data.ActiveUserCount)
	require.Equal(t, 30, data.Counts.Codes)
	require.Equal(t, 20, data.Counts.Datasets)
	require.Equal(t, 10, data.Counts.Models)
	require.Equal(t, 40, data.Counts.Spaces)
	require.Equal(t, 100, data.Counts.TotalRepos)
	require.NotEmpty(t, data.UUID)
	require.GreaterOrEqual(t, time.Now(), data.RecordedAt)
	require.LessOrEqual(t, time.Now().Add(-5*time.Second), data.RecordedAt)
}
