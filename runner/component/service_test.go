package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	knativefake "knative.dev/serving/pkg/client/clientset/versioned/fake"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockReporter "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestServiceComponent_RunService(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := &cluster.ClusterPool{}
	pool.ClusterStore = mockdb.NewMockClusterInfoStore(t)
	kubeClient := fake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	})
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
	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	kubeClient := fake.NewSimpleClientset()
	cluster := cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.Clusters = append(pool.Clusters, &cluster)

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
	cis.EXPECT().ByClusterID(ctx, "test").Return(database.ClusterInfo{
		ClusterID:     "test",
		ClusterConfig: "config",
		StorageClass:  "test",
	}, nil)
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
	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	kubeClient := fake.NewSimpleClientset()
	cluster := cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.Clusters = append(pool.Clusters, &cluster)
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
	cis.EXPECT().ByClusterID(ctx, "test").Return(database.ClusterInfo{
		ClusterID:     "test",
		ClusterConfig: "config",
		StorageClass:  "test",
	}, nil)
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
	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	kubeClient := fake.NewSimpleClientset()
	cluster := cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	}
	pool.Clusters = append(pool.Clusters, &cluster)
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
	cis.EXPECT().ByClusterID(ctx, "test").Return(database.ClusterInfo{
		ClusterID:     "test",
		ClusterConfig: "config",
		StorageClass:  "test",
	}, nil)
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
	pool := &cluster.ClusterPool{}
	pool.ClusterStore = mockdb.NewMockClusterInfoStore(t)
	kubeClient := fake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	})
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
	_, err = sc.getServicePodsWithStatus(ctx, pool.Clusters[0], "test", "test")
	require.Nil(t, err)
}

func TestServiceComponent_GetServiceByName(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := &cluster.ClusterPool{}
	pool.ClusterStore = mockdb.NewMockClusterInfoStore(t)
	kubeClient := fake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	})
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
	pool := &cluster.ClusterPool{}
	pool.ClusterStore = mockdb.NewMockClusterInfoStore(t)
	kubeClient := fake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	})
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
	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativeClient,
	})
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
	cis.EXPECT().ByClusterID(mock.Anything, "test").Return(database.ClusterInfo{
		ClusterID:     "test",
		ClusterConfig: "config",
		StorageClass:  "test",
	}, nil)
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
	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativeClient,
	})
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
	cis.EXPECT().ByClusterID(mock.Anything, "test").Return(database.ClusterInfo{
		ClusterID:     "test",
		ClusterConfig: "config",
		StorageClass:  "test",
	}, nil)
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
	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	knativeClient := knativefake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		KnativeClient: knativeClient,
	})
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

	pool := &cluster.ClusterPool{}
	pool.ClusterStore = mockdb.NewMockClusterInfoStore(t)
	kubeClient := fake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
	})

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

	res, err := sc.PodExist(ctx, pool.Clusters[0], "pod1")
	require.Nil(t, err)
	require.False(t, res)
}

func TestServiceComponent_GetPodLogsFromDB(t *testing.T) {
	ctx := context.TODO()

	kss := mockdb.NewMockKnativeServiceStore(t)

	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	knativeClient := knativefake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		KnativeClient: knativeClient,
	})

	logReq := database.DeployLog{
		ClusterID: pool.Clusters[0].ID,
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

	res, err := sc.GetPodLogsFromDB(ctx, pool.Clusters[0], "pod1", "svc")
	require.Nil(t, err)
	require.Equal(t, "", res)
}
