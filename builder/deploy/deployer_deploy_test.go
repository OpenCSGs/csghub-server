package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockSender "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter/sender"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestDeployer_InstanceLastLogs(t *testing.T) {
	t.Run("returns merged and sorted log values", func(t *testing.T) {
		ctx := context.TODO()

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		sender := mockSender.NewMockLogSender(t)

		dbDeploy := &database.Deploy{
			ID: 1,
		}
		mockDeployTaskStore.EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)

		sender.EXPECT().GenerateLabelQuery(mock.AnythingOfType("map[string]string")).Return(`{app="test"}`)

		lokiResp := &loki.LokiQueryResponse{}
		lokiResp.Status = "success"
		lokiResp.Data.ResultType = "streams"
		lokiResp.Data.Result = []loki.LokiStream{
			{
				Stream: map[string]string{"app": "a"},
				Values: [][]string{
					{"3000000000", "log3"},
					{"1000000000", "log1"},
				},
			},
			{
				Stream: map[string]string{"app": "b"},
				Values: [][]string{
					{"2000000000", "log2"},
				},
			},
		}

		sender.EXPECT().QueryLast(ctx, mock.AnythingOfType("loki.QueryLastParams")).Return(lokiResp, nil)

		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			lokiClient:      sender,
		}

		dr := types.DeployRequest{
			DeployID: 1,
			Limit:    100,
		}

		resp, err := d.InstanceLastLogs(ctx, dr)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, "success", resp.Status)
		require.Equal(t, 2, len(resp.Data.Result))
	})

	t.Run("filters by instance name when provided", func(t *testing.T) {
		ctx := context.TODO()

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		sender := mockSender.NewMockLogSender(t)

		dbDeploy := &database.Deploy{
			ID: 2,
		}
		mockDeployTaskStore.EXPECT().GetDeployByID(ctx, int64(2)).Return(dbDeploy, nil)

		sender.EXPECT().GenerateLabelQuery(mock.AnythingOfType("map[string]string")).Return(`{app="test",instance="pod-1"}`)

		lokiResp := &loki.LokiQueryResponse{}
		lokiResp.Status = "success"
		lokiResp.Data.ResultType = "streams"
		lokiResp.Data.Result = []loki.LokiStream{}

		sender.EXPECT().QueryLast(ctx, mock.AnythingOfType("loki.QueryLastParams")).Return(lokiResp, nil)

		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			lokiClient:      sender,
		}

		dr := types.DeployRequest{
			DeployID:     2,
			InstanceName: "pod-1",
			Limit:        50,
		}

		resp, err := d.InstanceLastLogs(ctx, dr)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("filters by commit id when provided", func(t *testing.T) {
		ctx := context.TODO()

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		sender := mockSender.NewMockLogSender(t)

		dbDeploy := &database.Deploy{
			ID: 3,
		}
		mockDeployTaskStore.EXPECT().GetDeployByID(ctx, int64(3)).Return(dbDeploy, nil)

		sender.EXPECT().GenerateLabelQuery(mock.AnythingOfType("map[string]string")).Return(`{app="test",commit_id="abc123"}`)

		lokiResp := &loki.LokiQueryResponse{}
		lokiResp.Status = "success"
		lokiResp.Data.ResultType = "streams"
		lokiResp.Data.Result = []loki.LokiStream{}

		sender.EXPECT().QueryLast(ctx, mock.AnythingOfType("loki.QueryLastParams")).Return(lokiResp, nil)

		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			lokiClient:      sender,
		}

		dr := types.DeployRequest{
			DeployID: 3,
			CommitID: "abc123",
			Limit:    100,
		}

		resp, err := d.InstanceLastLogs(ctx, dr)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("returns error when deploy lookup fails", func(t *testing.T) {
		ctx := context.TODO()

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		sender := mockSender.NewMockLogSender(t)

		testErr := errors.New("deploy lookup failed")
		dbDeploy := &database.Deploy{ID: 99}
		mockDeployTaskStore.EXPECT().GetDeployByID(ctx, int64(99)).Return(dbDeploy, testErr)

		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			lokiClient:      sender,
		}

		dr := types.DeployRequest{
			DeployID: 99,
			Limit:    100,
		}

		resp, err := d.InstanceLastLogs(ctx, dr)
		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("returns error when loki query fails", func(t *testing.T) {
		ctx := context.TODO()

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		sender := mockSender.NewMockLogSender(t)

		dbDeploy := &database.Deploy{
			ID: 4,
		}
		mockDeployTaskStore.EXPECT().GetDeployByID(ctx, int64(4)).Return(dbDeploy, nil)
		sender.EXPECT().GenerateLabelQuery(mock.AnythingOfType("map[string]string")).Return(`{app="test"}`)

		lokiErr := errors.New("loki unavailable")
		sender.EXPECT().QueryLast(ctx, mock.AnythingOfType("loki.QueryLastParams")).Return(nil, lokiErr)

		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			lokiClient:      sender,
		}

		dr := types.DeployRequest{
			DeployID: 4,
			Limit:    100,
		}

		resp, err := d.InstanceLastLogs(ctx, dr)
		require.Error(t, err)
		require.Nil(t, resp)
	})
}
