//go:build !ee && !saas

package workflow_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/store/database"
)

func TestSchedule_CalcRecomScoreWorkflow(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		tester, err := newWorkflowTester(t)
		require.NoError(t, err)

		tester.mocks.recom.EXPECT().CalculateRecomScore(mock.Anything, 0).Return(nil)
		tester.scheduler.Execute("calc-recom-score-schedule", tester.cronEnv)
		require.True(t, tester.cronEnv.IsWorkflowCompleted())
		require.NoError(t, tester.cronEnv.GetWorkflowError())
	})

	t.Run("error", func(t *testing.T) {
		tester, err := newWorkflowTester(t)
		require.NoError(t, err)

		tester.mocks.recom.EXPECT().CalculateRecomScore(mock.Anything, 0).Return(errors.New("error"))
		tester.scheduler.Execute("calc-recom-score-schedule", tester.cronEnv)
		require.True(t, tester.cronEnv.IsWorkflowCompleted())
		require.Error(t, tester.cronEnv.GetWorkflowError())
	})
}

func TestSchedule_SyncAsClient(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	tester.mocks.stores.SyncClientSettingMock().EXPECT().First(mock.Anything).Return(
		&database.SyncClientSetting{Token: "tk"}, nil,
	)
	tester.mocks.multisync.EXPECT().SyncAsClient(
		mock.Anything, multisync.FromOpenCSG("", "tk"),
	).Return(nil)

	tester.scheduler.Execute("sync-as-client-schedule", tester.cronEnv)
	require.True(t, tester.cronEnv.IsWorkflowCompleted())
	require.NoError(t, tester.cronEnv.GetWorkflowError())

}
