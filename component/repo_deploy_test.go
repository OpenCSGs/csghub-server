package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

var errTestDeployer = errors.New("deployer error")

func TestRepoComponent_DeployInstanceLastLogs_NormalUser(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	logReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "repo",
		CurrentUser: "test-user",
		DeployID:    1,
		DeployType:  types.InferenceType,
		Limit:       100,
	}

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:        1,
		UserID:    123,
		SvcName:   "svc-1",
		ClusterID: "cluster-1",
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "test-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)

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
				{"4000000000", "log4"},
			},
		},
	}

	repo.mocks.deployer.EXPECT().InstanceLastLogs(ctx, mock.AnythingOfType("types.DeployRequest")).Return(lokiResp, nil)

	result, err := repo.DeployInstanceLastLogs(ctx, logReq)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify merged and sorted by timestamp ascending
	require.Equal(t, 4, len(result.Values))
	require.Equal(t, "1000000000", result.Values[0][0])
	require.Equal(t, "2000000000", result.Values[1][0])
	require.Equal(t, "3000000000", result.Values[2][0])
	require.Equal(t, "4000000000", result.Values[3][0])

	require.Equal(t, "4", result.Stream["count"])
}

func TestRepoComponent_DeployInstanceLastLogs_SingleStream(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	logReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "repo",
		CurrentUser: "test-user",
		DeployID:    2,
		DeployType:  types.InferenceType,
		Limit:       50,
	}

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:        2,
		UserID:    123,
		SvcName:   "svc-2",
		ClusterID: "cluster-2",
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "test-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(2)).Return(dbDeploy, nil)

	lokiResp := &loki.LokiQueryResponse{}
	lokiResp.Status = "success"
	lokiResp.Data.ResultType = "streams"
	lokiResp.Data.Result = []loki.LokiStream{
		{
			Stream: map[string]string{"app": "a"},
			Values: [][]string{
				{"3000000000", "log3"},
				{"1000000000", "log1"},
				{"2000000000", "log2"},
			},
		},
	}

	repo.mocks.deployer.EXPECT().InstanceLastLogs(ctx, mock.AnythingOfType("types.DeployRequest")).Return(lokiResp, nil)

	result, err := repo.DeployInstanceLastLogs(ctx, logReq)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 3, len(result.Values))
	require.Equal(t, "1000000000", result.Values[0][0])
	require.Equal(t, "2000000000", result.Values[1][0])
	require.Equal(t, "3000000000", result.Values[2][0])
}

func TestRepoComponent_DeployInstanceLastLogs_EmptyResult(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	logReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "repo",
		CurrentUser: "test-user",
		DeployID:    3,
		DeployType:  types.InferenceType,
		Limit:       100,
	}

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:        3,
		UserID:    123,
		SvcName:   "svc-3",
		ClusterID: "cluster-3",
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "test-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(3)).Return(dbDeploy, nil)

	lokiResp := &loki.LokiQueryResponse{}
	lokiResp.Status = "success"
	lokiResp.Data.ResultType = "streams"
	lokiResp.Data.Result = []loki.LokiStream{}

	repo.mocks.deployer.EXPECT().InstanceLastLogs(ctx, mock.AnythingOfType("types.DeployRequest")).Return(lokiResp, nil)

	result, err := repo.DeployInstanceLastLogs(ctx, logReq)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 0, len(result.Values))
	require.Equal(t, "0", result.Stream["count"])
}

func TestRepoComponent_DeployInstanceLastLogs_DeployerError(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	logReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "repo",
		CurrentUser: "test-user",
		DeployID:    4,
		DeployType:  types.InferenceType,
		Limit:       100,
	}

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:        4,
		UserID:    123,
		SvcName:   "svc-4",
		ClusterID: "cluster-4",
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "test-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(4)).Return(dbDeploy, nil)

	repo.mocks.deployer.EXPECT().InstanceLastLogs(ctx, mock.AnythingOfType("types.DeployRequest")).Return(nil, errTestDeployer)

	result, err := repo.DeployInstanceLastLogs(ctx, logReq)
	require.Error(t, err)
	require.Nil(t, result)
}

func TestRepoComponent_DeployInstanceLastLogs_ServerlessType(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	logReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "repo",
		CurrentUser: "admin-user",
		DeployID:    6,
		DeployType:  types.ServerlessType,
		Limit:       100,
	}

	dbUser := database.User{
		ID:       1,
		RoleMask: "admin",
	}
	dbDeploy := &database.Deploy{
		ID:        6,
		UserID:    1,
		SvcName:   "svc-6",
		ClusterID: "cluster-6",
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(6)).Return(dbDeploy, nil)

	lokiResp := &loki.LokiQueryResponse{}
	lokiResp.Status = "success"
	lokiResp.Data.ResultType = "streams"
	lokiResp.Data.Result = []loki.LokiStream{
		{
			Values: [][]string{
				{"5000000000", "log5"},
			},
		},
	}

	repo.mocks.deployer.EXPECT().InstanceLastLogs(ctx, mock.AnythingOfType("types.DeployRequest")).Return(lokiResp, nil)

	result, err := repo.DeployInstanceLastLogs(ctx, logReq)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, len(result.Values))
	require.Equal(t, "5000000000", result.Values[0][0])
	require.Equal(t, "1", result.Stream["count"])
}
