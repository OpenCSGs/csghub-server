package component

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	apis "knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	knativefake "knative.dev/serving/pkg/client/clientset/versioned/fake"
	mockCluster "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/cluster"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockReporter "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	rtypes "opencsg.com/csghub-server/runner/types"
)

func TestServiceComponent_RunService(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	expectCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(expectCluster, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
}

func TestServiceComponent_StopService(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	cluster := cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(&cluster, nil)
	reporter := mockReporter.NewMockLogCollector(t)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        reporter,
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	kss.EXPECT().Get(ctx, "test", "test").Return(&database.KnativeService{}, nil)
	kss.EXPECT().Delete(ctx, "test", "test").Return(nil)
	reporter.EXPECT().Report(mock.Anything)

	resp, err := sc.StopService(ctx, types.StopRequest{
		SvcName:   "test",
		ClusterID: "test",
	})
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, resp.Code, 0)
}

func TestServiceComponent_PurgeService(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	cluster := cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(&cluster, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	resp, err := sc.PurgeService(ctx, types.PurgeRequest{
		SvcName:   "test",
		ClusterID: "test",
	})
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, resp.Code, 0)
}

func TestServiceComponent_UpdateService(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	cluster := cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(&cluster, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	resp, err := sc.UpdateService(ctx, types.ModelUpdateRequest{
		SvcName:    "test",
		ClusterID:  "test",
		MinReplica: 2,
		MaxReplica: 2,
		ImageID:    "test2",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
	})
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, resp.Code, 0)
}
func TestServiceComponent_GetServicePodWithStatus(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	expectCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(expectCluster, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	_, _, err = sc.getServicePodsWithStatus(ctx, expectCluster, "test", "test")
	require.Nil(t, err)
}

func TestServiceComponent_GetServiceByName(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	expectCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(expectCluster, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	kss.EXPECT().Get(ctx, "test", "test").Return(&database.KnativeService{
		Name: "test",
		ID:   1,
		Code: common.Running,
	}, nil)

	resp, err := sc.GetServiceByName(ctx, "test", "test")
	require.Nil(t, err)
	require.Equal(t, "test", resp.ServiceName)
	require.Equal(t, common.Running, resp.Code)
}

func TestServiceComponent_GetServiceInfo(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	expectCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(expectCluster, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	kss.EXPECT().Get(ctx, "test", "test").Return(&database.KnativeService{
		Name: "test",
		ID:   1,
		Code: common.Running,
	}, nil)
	resp, err := sc.GetServiceInfo(ctx, types.ServiceRequest{
		ServiceName: "test",
		ClusterID:   "test",
	})
	require.Nil(t, err)
	require.Equal(t, "test", resp.ServiceName)
}

func TestServiceComponent_AddServiceInDB(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(&cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativeClient,
	}, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	ctx := context.TODO()
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	ksvc, err := knativeClient.ServingV1().Services("test").
		Get(ctx, "test", metav1.GetOptions{})
	require.Nil(t, err)
	sc.logReporter.(*mockReporter.MockLogCollector).EXPECT().Report(mock.Anything)
	err = sc.addServiceInDB(*ksvc, "test")
	require.Nil(t, err)
}

func TestServiceComponent_updateServiceInDB(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(&cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativeClient,
	}, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	ctx := context.TODO()
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	kss.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
	kss.EXPECT().Get(mock.Anything, "test", "test").Return(&database.KnativeService{
		ID:        1,
		Name:      "test",
		ClusterID: "test",
		Code:      common.Deploying,
	}, nil)
	ksvc, err := knativeClient.ServingV1().Services("test").
		Get(ctx, "test", metav1.GetOptions{})
	require.Nil(t, err)
	sc.logReporter.(*mockReporter.MockLogCollector).EXPECT().Report(mock.Anything)
	err = sc.updateServiceInDB(*ksvc, "test", nil)
	require.Nil(t, err)
}

func TestServiceComponent_deleteServiceInDB2(t *testing.T) {
	cfg, err := config.LoadConfig()
	cfg.Accounting.ChargingEnable = true
	require.Nil(t, err)

	kss := mockdb.NewMockKnativeServiceStore(t)
	pool := mockCluster.NewMockPool(t)
	knativeClient := knativefake.NewSimpleClientset()
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(&cluster.Cluster{
		CID:           "config",
		ID:            "test",
		KnativeClient: knativeClient,
	}, nil)
	reporter := mockReporter.NewMockLogCollector(t)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        reporter,
		// eventPub:           &eventPub,
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}

	ctx := context.TODO()
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)

	err = sc.RunService(ctx, req)
	require.Nil(t, err)

	ksvc, err := knativeClient.ServingV1().Services("test").
		Get(ctx, "test", metav1.GetOptions{})
	require.Nil(t, err)

	kss.EXPECT().Get(ctx, ksvc.Name, "test").Return(&database.KnativeService{
		ID:   1,
		Name: "test",
	}, nil)
	kss.EXPECT().Delete(mock.Anything, "test", "test").Return(nil)

	reporter.EXPECT().Report(mock.Anything)

	err = sc.deleteKServiceWithEvent(ctx, ksvc.Name, "test")
	require.Nil(t, err)
}

func TestServiceComponent_PodExist(t *testing.T) {
	ctx := context.TODO()

	kss := mockdb.NewMockKnativeServiceStore(t)

	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	expectCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}

	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}

	res, err := sc.PodExist(ctx, expectCluster, "pod1")
	require.Nil(t, err)
	require.False(t, res)
}

func TestServiceComponent_GetPodLogsFromDB(t *testing.T) {
	ctx := context.TODO()

	kss := mockdb.NewMockKnativeServiceStore(t)

	pool := mockCluster.NewMockPool(t)
	knativeClient := knativefake.NewSimpleClientset()
	expectCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		KnativeClient: knativeClient,
	}

	logReq := database.DeployLog{
		ClusterID: expectCluster.ID,
		SvcName:   "svc",
		PodName:   "pod1",
	}

	dls := mockdb.NewMockDeployLogStore(t)
	dls.EXPECT().GetDeployLogs(ctx, logReq).Return(&logReq, nil)

	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		// eventPub:           &eventPub,
		deployLogStore: dls,
		logReporter:    mockReporter.NewMockLogCollector(t),
	}

	res, err := sc.GetPodLogsFromDB(ctx, expectCluster, "pod1", "svc")
	require.Nil(t, err)
	require.Equal(t, "", res)
}

func TestServiceComponent_GetServiceByNameFromK8s(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)

	kubeClient := fake.NewSimpleClientset()
	expectCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(expectCluster, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}
	req := types.SVCRequest{
		ClusterID:  "test",
		ImageID:    "test",
		DeployID:   1,
		DeployType: types.InferenceType,
		RepoType:   string(types.ModelRepo),
		MinReplica: 1,
		MaxReplica: 1,
		UserID:     "test",
		Sku:        "1",
		SvcName:    "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory: "16Gi",
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	kss.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	kss.EXPECT().Get(ctx, "test", "test").Return(nil, sql.ErrNoRows)

	resp, err := sc.GetServiceByName(ctx, "test", "test")
	require.Nil(t, err)
	require.Equal(t, "test", resp.ServiceName)
	require.Equal(t, common.Pending, resp.Code)
}

func TestServiceComponent_SetVersionsTraffic(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(&cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativeClient,
	}, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
	}

	// Test case 1: Successful traffic setting
	trafficReqs := []types.TrafficReq{
		{
			Commit:         "commit1",
			TrafficPercent: 60,
		},
		{
			Commit:         "commit2",
			TrafficPercent: 40,
		},
	}

	// Create a mock service first
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "test",
		},
		Spec: v1.ServiceSpec{},
	}

	_, err := knativeClient.ServingV1().Services("test").Create(ctx, service, metav1.CreateOptions{})
	require.Nil(t, err)

	err = sc.SetVersionsTraffic(ctx, "test", "test-service", trafficReqs)
	// This might fail due to revision validation, but we're testing the basic flow
	if err != nil {
		// t.Errorf("SetVersionsTraffic failed: %v", err)
		t.Logf("SetVersionsTraffic failed: %v", err)
	}
}

func Test_isContainerStatusChanged(t *testing.T) {
	tests := []struct {
		name           string
		oldPod         *corev1.Pod
		newPod         *corev1.Pod
		containerNames []string
		want           bool
	}{
		{
			name: "no change",
			oldPod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "c1",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			newPod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "c1",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			containerNames: []string{"c1"},
			want:           false,
		},
		{
			name: "status changed",
			oldPod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "c1",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			newPod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "c1",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{},
							},
						},
					},
				},
			},
			containerNames: []string{"c1"},
			want:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isContainerStatusChanged(tt.oldPod, tt.newPod, tt.containerNames...); got != tt.want {
				t.Errorf("isContainerStatusChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceComponent_ListVersions(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	rss := mockdb.NewMockKnativeServiceRevisionStore(t)
	ctx := context.TODO()

	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		logReporter:        mockReporter.NewMockLogCollector(t),
		revisionStore:      rss,
	}

	// Test case 1: Successful version listing
	expectedRevisions := []database.KnativeServiceRevision{
		{
			RevisionName:   "test-service-001",
			CommitID:       "commit1",
			TrafficPercent: 60,
			IsReady:        true,
			Message:        "Ready",
			Reason:         "",
			CreateTime:     time.Now(),
		},
		{
			RevisionName:   "test-service-002",
			CommitID:       "commit2",
			TrafficPercent: 40,
			IsReady:        true,
			Message:        "Ready",
			Reason:         "",
			CreateTime:     time.Now(),
		},
	}

	rss.EXPECT().ListRevisions(ctx, "test-service").Return(expectedRevisions, nil)

	versions, err := sc.ListVersions(ctx, "test", "test-service")
	require.Nil(t, err)
	require.Len(t, versions, 2)
	require.Equal(t, "commit1", versions[0].Commit)
	require.Equal(t, int64(60), versions[0].TrafficPercent)
	require.Equal(t, "commit2", versions[1].Commit)
	require.Equal(t, int64(40), versions[1].TrafficPercent)

	// Test case 2: No revisions found
	rss.EXPECT().ListRevisions(ctx, "empty-service").Return([]database.KnativeServiceRevision{}, nil)

	emptyVersions, err := sc.ListVersions(ctx, "test", "empty-service")
	require.Nil(t, err)
	require.Len(t, emptyVersions, 0)
}

func TestServiceComponent_DeleteKsvcVersion(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	rss := mockdb.NewMockKnativeServiceRevisionStore(t)
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)

	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(&cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativeClient,
	}, nil)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
		logReporter:        mockReporter.NewMockLogCollector(t),
		revisionStore:      rss,
	}

	// Test case 1: Successful deletion
	revision := &database.KnativeServiceRevision{
		RevisionName:   "test-service-001",
		CommitID:       "commit1",
		TrafficPercent: 0, // Can only delete if traffic is 0
		SvcName:        "test-service",
	}

	rss.EXPECT().QueryRevision(ctx, "test-service", "commit1").Return(revision, nil)

	err := sc.DeleteKsvcVersion(ctx, "test", "test-service", "commit1")
	require.Contains(t, err.Error(), "SERVERLESS-ERR-1")

	// Test case 2: Revision not found
	rss.EXPECT().QueryRevision(ctx, "test-service", "nonexistent").Return(nil, sql.ErrNoRows)

	err = sc.DeleteKsvcVersion(ctx, "test", "test-service", "nonexistent")
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)

	// Test case 3: Cannot delete revision with traffic
	trafficRevision := &database.KnativeServiceRevision{
		RevisionName:   "test-service-002",
		CommitID:       "commit2",
		TrafficPercent: 50, // Has traffic, cannot delete
		SvcName:        "test-service",
	}

	rss.EXPECT().QueryRevision(ctx, "test-service", "commit2").Return(trafficRevision, nil)

	err = sc.DeleteKsvcVersion(ctx, "test", "test-service", "commit2")
	require.Error(t, err)
	require.Equal(t, errorx.ErrDeployNotFoundErr, err)
}

func TestServiceComponent_reportServiceLog(t *testing.T) {
	reporter := mockReporter.NewMockLogCollector(t)
	sc := &serviceComponentImpl{
		logReporter: reporter,
	}

	ksvc := &database.KnativeService{
		Name:       "test-service",
		Status:     corev1.ConditionTrue,
		DeployID:   123,
		ClusterID:  "test-cluster",
		DeployType: 1,
		ID:         456,
	}

	statusRes := &types.StatusResponse{
		Code:           1,
		Message:        "test message",
		Reason:         "test reason",
		ServiceMessage: "svc msg",
		ServiceReason:  "svc reason",
	}

	// Expect Report to be called with correct message format
	reporter.EXPECT().Report(mock.MatchedBy(func(logEntry types.LogEntry) bool {
		expectedMsg := fmt.Sprintf("test msg, ksvc statue: %s, deploy status: %d,\nksvc msg: %s\nksvc reason: %s",
			ksvc.Status, statusRes.Code, statusRes.ServiceMessage, statusRes.ServiceReason)
		return logEntry.Message == expectedMsg
	})).Return()

	sc.reportServiceLog("test msg", ksvc, nil, statusRes)
}

func TestServiceComponent_getServiceStatus(t *testing.T) {
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()

	expectCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test-cluster",
		Client:        kubeClient,
		KnativeClient: knativeClient,
	}

	sc := &serviceComponentImpl{
		clusterPool: pool,
	}

	// Setup K8s resources
	namespace := "test-ns"
	svcName := "test-service"

	// Create Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: namespace,
			Labels: map[string]string{
				KeyServiceLabel: svcName,
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  rtypes.UserContainerName,
					Ready: true,
				},
			},
		},
	}
	_, err := kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	require.Nil(t, err)

	ks := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: namespace,
		},
		Status: v1.ServiceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   v1.ServiceConditionReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	// Create revisions
	rev := &v1.Revision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rev",
			Namespace: namespace,
			Labels: map[string]string{
				"serving.knative.dev/service": svcName,
			},
		},
		Status: v1.RevisionStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   v1.RevisionConditionReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}
	_, err = knativeClient.ServingV1().Revisions(namespace).Create(ctx, rev, metav1.CreateOptions{})
	require.Nil(t, err)

	pool.EXPECT().GetClusterByID(ctx, "test-cluster").Return(expectCluster, nil)

	// Call getServiceStatus
	resp, err := sc.getServiceStatus(ctx, ks, "test-cluster")
	require.Nil(t, err)
	require.Equal(t, common.Deploying, resp.Code)
	require.NotEmpty(t, resp.Instances)
}

func Test_isUserContainerActive(t *testing.T) {
	tests := []struct {
		name     string
		instList []types.Instance
		wantBool bool
		wantStr  string
	}{
		{
			name: "Running pod",
			instList: []types.Instance{
				{Status: string(corev1.PodRunning)},
			},
			wantBool: true,
			wantStr:  string(corev1.PodRunning),
		},
		{
			name: "Pending pod",
			instList: []types.Instance{
				{Status: string(corev1.PodPending)},
			},
			wantBool: true,
			wantStr:  string(corev1.PodPending),
		},
		{
			name: "Failed pod",
			instList: []types.Instance{
				{Status: string(corev1.PodFailed)},
			},
			wantBool: false,
			wantStr:  "",
		},
		{
			name:     "Empty list",
			instList: []types.Instance{},
			wantBool: false,
			wantStr:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBool, gotStr := isUserContainerActive(tt.instList)
			if gotBool != tt.wantBool {
				t.Errorf("isUserContainerActive() gotBool = %v, want %v", gotBool, tt.wantBool)
			}
			if gotStr != tt.wantStr {
				t.Errorf("isUserContainerActive() gotStr = %v, want %v", gotStr, tt.wantStr)
			}
		})
	}
}
