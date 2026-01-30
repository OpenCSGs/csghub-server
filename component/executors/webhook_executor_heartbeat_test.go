package executors

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewTestHeartBeatExecutor(config *config.Config, cs database.ClusterInfoStore) WebHookExecutor {
	executor := &heartbeatExecutorImpl{
		cfg:          config,
		clusterStore: cs,
	}
	return executor
}

func TestWebHookExecutorHeartbeat_ProcessEvent(t *testing.T) {
	ctx := context.TODO()

	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	eventData := []*types.ClusterRes{
		{
			ClusterID: "cluster1",
		},
	}

	buf, err := json.Marshal(eventData)
	require.Nil(t, err)

	event := &types.WebHookRecvEvent{
		WebHookHeader: types.WebHookHeader{
			EventType: types.RunnerHeartbeat,
			EventTime: time.Now().Unix(),
			DataType:  types.WebHookDataTypeArray,
		},
		Data: buf,
	}

	cs := mockdb.NewMockClusterInfoStore(t)
	cs.EXPECT().BatchUpdateStatus(ctx, eventData).Return(nil)

	exec := NewTestHeartBeatExecutor(cfg, cs)

	err = exec.ProcessEvent(ctx, event)
	require.Nil(t, err)
}
