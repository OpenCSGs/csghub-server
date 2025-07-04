package component

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	prometheus_mock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/prometheus"
	mock_rpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	deployer "opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/prometheus"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewTestMonitorComponent(cfg *config.Config,
	client prometheus.PrometheusClient,
	usc rpc.UserSvcClient,
	deployTaskStore database.DeployTaskStore,
	repoStore database.RepoStore,
	deployer deployer.Deployer,
) (MonitorComponent, error) {
	domainParts := strings.SplitN(cfg.Space.InternalRootDomain, ".", 2)
	return &monitorComponentImpl{
		k8sNameSpace:    domainParts[0],
		client:          client,
		userSvcClient:   usc,
		deployTaskStore: deployTaskStore,
		repoStore:       repoStore,
		deployer:        deployer,
	}, nil
}

func TestMonitor_RequestLatency(t *testing.T) {
	ctx := context.TODO()

	req := &types.MonitorReq{
		CurrentUser:  "user",
		Namespace:    "ns",
		Name:         "n",
		RepoType:     types.SpaceRepo,
		DeployID:     1,
		Instance:     "test-instance",
		LastDuration: "30m",
	}

	cfg := &config.Config{}

	usc := mock_rpc.NewMockUserSvcClient(t)
	client := prometheus_mock.NewMockPrometheusClient(t)
	deployTaskStore := mockdb.NewMockDeployTaskStore(t)
	repoStore := mockdb.NewMockRepoStore(t)

	usc.EXPECT().GetUserInfo(ctx, req.CurrentUser, req.CurrentUser).Return(&rpc.User{
		ID:       1,
		Username: req.CurrentUser,
		Roles:    []string{"person"},
	}, nil)

	repoStore.EXPECT().FindByPath(ctx, req.RepoType, req.Namespace, req.Name).Return(&database.Repository{
		ID: 1,
	}, nil)

	deployTaskStore.EXPECT().GetDeployByID(ctx, req.DeployID).Return(&database.Deploy{
		ID:      1,
		RepoID:  1,
		SvcName: "test",
		UserID:  1,
	}, nil)

	query := fmt.Sprintf("sum(increase(revision_app_request_latencies_bucket{pod_name='%s',namespace='%s'}[%s:])) by (le)",
		req.Instance, "", req.LastDuration)

	client.EXPECT().SerialData(query).Return(&types.PrometheusResponse{
		Data: types.PrometheusData{
			ResultType: "vector",
			Result: []types.PrometheusResult{
				{
					Metric: map[string]string{
						"le": "0.005",
					},
					Value: []any{
						1678617600.0,
						"100",
					},
				},
			},
		},
	}, nil)

	mon, err := NewTestMonitorComponent(cfg, client, usc, deployTaskStore, repoStore, nil)
	require.Nil(t, err)

	resp, err := mon.RequestLatency(ctx, req)
	require.Nil(t, err)
	require.Equal(t, resp, &types.MonitorRequestLatencyResp{
		ResultType: "vector",
		Result: []types.MonitorData{
			{
				Metric: map[string]string{
					"le": "0.005",
				},
				Value: types.MonitorValue{
					Timestamp: 1678617600.0,
					Value:     100,
				},
			},
		},
	})
}

func TestMonitor_RequestCount(t *testing.T) {
	ctx := context.TODO()

	req := &types.MonitorReq{
		CurrentUser:  "user",
		Namespace:    "ns",
		Name:         "n",
		RepoType:     types.SpaceRepo,
		DeployID:     1,
		Instance:     "test-instance",
		LastDuration: "30m",
	}

	cfg := &config.Config{}

	usc := mock_rpc.NewMockUserSvcClient(t)
	client := prometheus_mock.NewMockPrometheusClient(t)
	deployTaskStore := mockdb.NewMockDeployTaskStore(t)
	repoStore := mockdb.NewMockRepoStore(t)

	usc.EXPECT().GetUserInfo(ctx, req.CurrentUser, req.CurrentUser).Return(&rpc.User{
		ID:       1,
		Username: req.CurrentUser,
		Roles:    []string{"person"},
	}, nil)

	repoStore.EXPECT().FindByPath(ctx, req.RepoType, req.Namespace, req.Name).Return(&database.Repository{
		ID: 1,
	}, nil)

	deployTaskStore.EXPECT().GetDeployByID(ctx, req.DeployID).Return(&database.Deploy{
		ID:      1,
		RepoID:  1,
		SvcName: "test",
		UserID:  1,
	}, nil)

	query := fmt.Sprintf("avg_over_time(revision_request_count{pod_name='%s',namespace='%s'}[%s:])[%s:%s]",
		req.Instance, "", req.LastDuration, req.LastDuration, req.TimeRange)

	client.EXPECT().SerialData(query).Return(&types.PrometheusResponse{
		Data: types.PrometheusData{
			ResultType: "vector",
			Result: []types.PrometheusResult{
				{
					Metric: map[string]string{
						"le": "200",
					},
					Values: [][]any{
						{
							1678617600.0,
							"100",
						},
					},
				},
			},
		},
	}, nil)

	mon, err := NewTestMonitorComponent(cfg, client, usc, deployTaskStore, repoStore, nil)
	require.Nil(t, err)

	resp, err := mon.RequestCount(ctx, req)
	require.Nil(t, err)
	require.Equal(t, resp, &types.MonitorRequestCountResp{
		ResultType: "vector",
		Result: []types.MonitorData{
			{
				Metric: map[string]string{
					"le": "200",
				},
				Values: []types.MonitorValue{
					{
						Timestamp: 1678617600,
						Value:     0,
					},
				},
			},
		},
		TotalRequestCount: 0,
	})

}

func TestMonitor_MemoryUsage(t *testing.T) {
	ctx := context.TODO()

	req := &types.MonitorReq{
		CurrentUser:  "user",
		Namespace:    "ns",
		Name:         "n",
		RepoType:     types.SpaceRepo,
		DeployID:     1,
		Instance:     "test-instance",
		LastDuration: "30m",
	}

	cfg := &config.Config{}

	usc := mock_rpc.NewMockUserSvcClient(t)
	client := prometheus_mock.NewMockPrometheusClient(t)
	deployTaskStore := mockdb.NewMockDeployTaskStore(t)
	repoStore := mockdb.NewMockRepoStore(t)

	usc.EXPECT().GetUserInfo(ctx, req.CurrentUser, req.CurrentUser).Return(&rpc.User{
		ID:       1,
		Username: req.CurrentUser,
		Roles:    []string{"person"},
	}, nil)

	repoStore.EXPECT().FindByPath(ctx, req.RepoType, req.Namespace, req.Name).Return(&database.Repository{
		ID: 1,
	}, nil)

	deployTaskStore.EXPECT().GetDeployByID(ctx, req.DeployID).Return(&database.Deploy{
		ID:      1,
		RepoID:  1,
		SvcName: "test",
		UserID:  1,
	}, nil)

	query := fmt.Sprintf("avg_over_time(container_memory_usage_bytes{pod='%s',namespace='%s',container='user-container'}[%s:])[%s:%s]",
		req.Instance, "", req.LastDuration, req.LastDuration, req.TimeRange)

	client.EXPECT().SerialData(query).Return(&types.PrometheusResponse{
		Data: types.PrometheusData{
			ResultType: "vector",
			Result: []types.PrometheusResult{
				{
					Metric: map[string]string{
						"pod": "test-instance",
					},
					Values: [][]any{
						{
							1678617600.0,
							(1024 * 1024 * 1024),
						},
					},
				},
			},
		},
	}, nil)

	mon, err := NewTestMonitorComponent(cfg, client, usc, deployTaskStore, repoStore, nil)
	require.Nil(t, err)

	resp, err := mon.MemoryUsage(ctx, req)
	require.Nil(t, err)
	require.Equal(t, resp, &types.MonitorMemoryResp{
		ResultType: "vector",
		Result: []types.MonitorData{
			{
				Metric: map[string]string{
					"instance": "test-instance",
				},
				Values: []types.MonitorValue{
					{
						Timestamp: 1678617600.0,
						Value:     1,
					},
				},
			},
		},
	})
}

func TestMonitor_MemoryUsage_Evaluation(t *testing.T) {
	ctx := context.TODO()

	req := &types.MonitorReq{
		CurrentUser:  "user",
		Namespace:    "ns",
		Name:         "n",
		RepoType:     types.SpaceRepo,
		DeployType:   "evaluation",
		DeployID:     1,
		Instance:     "test-instance",
		LastDuration: "30m",
	}

	cfg := &config.Config{}

	usc := mock_rpc.NewMockUserSvcClient(t)
	client := prometheus_mock.NewMockPrometheusClient(t)
	deployTaskStore := mockdb.NewMockDeployTaskStore(t)
	repoStore := mockdb.NewMockRepoStore(t)
	mockDeployer := deploy.NewMockDeployer(t)

	usc.EXPECT().GetUserInfo(ctx, req.CurrentUser, req.CurrentUser).Return(&rpc.User{
		ID:       1,
		Username: req.CurrentUser,
		Roles:    []string{"person"},
	}, nil)
	req2 := types.EvaluationGetReq{
		ID:       1,
		Username: "user",
	}
	mockDeployer.EXPECT().GetEvaluation(ctx, req2).Return(&types.ArgoWorkFlowRes{
		ID:        1,
		RepoIds:   []string{"Rowan/hellaswag"},
		Datasets:  []string{"Rowan/hellaswag"},
		RepoType:  "model",
		Username:  "user",
		TaskName:  "test",
		TaskId:    "test",
		TaskType:  "evaluation",
		Status:    "Succeed",
		Namespace: "",
	}, nil)

	query := fmt.Sprintf("avg_over_time(container_memory_usage_bytes{pod='%s',namespace='%s',container='main'}[%s:])[%s:%s]",
		req.Instance, "", req.LastDuration, req.LastDuration, req.TimeRange)

	client.EXPECT().SerialData(query).Return(&types.PrometheusResponse{
		Data: types.PrometheusData{
			ResultType: "vector",
			Result: []types.PrometheusResult{
				{
					Metric: map[string]string{
						"pod": "test-instance",
					},
					Values: [][]any{
						{
							1678617600.0,
							(1024 * 1024 * 1024),
						},
					},
				},
			},
		},
	}, nil)

	mon, err := NewTestMonitorComponent(cfg, client, usc, deployTaskStore, repoStore, mockDeployer)
	require.Nil(t, err)

	resp, err := mon.MemoryUsage(ctx, req)
	require.Nil(t, err)
	require.Equal(t, resp, &types.MonitorMemoryResp{
		ResultType: "vector",
		Result: []types.MonitorData{
			{
				Metric: map[string]string{
					"instance": "test-instance",
				},
				Values: []types.MonitorValue{
					{
						Timestamp: 1678617600.0,
						Value:     1,
					},
				},
			},
		},
	})
}

func TestMonitor_CPUUsage(t *testing.T) {
	ctx := context.TODO()

	req := &types.MonitorReq{
		CurrentUser:  "user",
		Namespace:    "ns",
		Name:         "n",
		RepoType:     types.SpaceRepo,
		DeployID:     1,
		Instance:     "test-instance",
		LastDuration: "30m",
	}

	cfg := &config.Config{}

	usc := mock_rpc.NewMockUserSvcClient(t)
	client := prometheus_mock.NewMockPrometheusClient(t)
	deployTaskStore := mockdb.NewMockDeployTaskStore(t)
	repoStore := mockdb.NewMockRepoStore(t)

	usc.EXPECT().GetUserInfo(ctx, req.CurrentUser, req.CurrentUser).Return(&rpc.User{
		ID:       1,
		Username: req.CurrentUser,
		Roles:    []string{"person"},
	}, nil)

	repoStore.EXPECT().FindByPath(ctx, req.RepoType, req.Namespace, req.Name).Return(&database.Repository{
		ID: 1,
	}, nil)

	deployTaskStore.EXPECT().GetDeployByID(ctx, req.DeployID).Return(&database.Deploy{
		ID:      1,
		RepoID:  1,
		SvcName: "test",
		UserID:  1,
	}, nil)

	query := fmt.Sprintf("kube_pod_container_resource_limits{pod='%s',namespace='%s',resource='cpu'}", req.Instance, "")

	client.EXPECT().SerialData(query).Return(&types.PrometheusResponse{
		Data: types.PrometheusData{
			ResultType: "vector",
			Result: []types.PrometheusResult{
				{
					Metric: map[string]string{
						"pod": "test-instance",
					},
					Value: []any{
						1678617600.0,
						"1",
					},
				},
			},
		},
	}, nil)

	query = fmt.Sprintf("avg_over_time(rate(container_cpu_usage_seconds_total{pod='%s',namespace='%s',container='user-container'}[1m])[%s:])[%s:%s]", req.Instance, "", req.LastDuration, req.LastDuration, req.TimeRange)

	client.EXPECT().SerialData(query).Return(&types.PrometheusResponse{
		Data: types.PrometheusData{
			ResultType: "vector",
			Result: []types.PrometheusResult{
				{
					Metric: map[string]string{
						"pod": "test-instance",
					},
					Values: [][]any{
						{
							1678617600.0,
							"1",
						},
					},
				},
			},
		},
	}, nil)

	mon, err := NewTestMonitorComponent(cfg, client, usc, deployTaskStore, repoStore, nil)
	require.Nil(t, err)

	resp, err := mon.CPUUsage(ctx, req)
	require.Nil(t, err)
	require.Equal(t, resp, &types.MonitorCPUResp{
		ResultType: "vector",
		Result: []types.MonitorData{
			{
				Metric: map[string]string{
					"instance": "test-instance",
				},
				Values: []types.MonitorValue{
					{
						Timestamp: 1678617600.0,
						Value:     100,
					},
				},
			},
		},
	})

}
