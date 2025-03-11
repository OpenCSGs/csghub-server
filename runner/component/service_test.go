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
	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mq"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/event"
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
	pool.Clusters = append(pool.Clusters, cluster.Cluster{
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
	pool.Clusters = append(pool.Clusters, cluster)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	cis.EXPECT().ByClusterID(ctx, "test").Return(database.ClusterInfo{
		ClusterID:     "test",
		ClusterConfig: "config",
		StorageClass:  "test",
	}, nil)
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
	pool.Clusters = append(pool.Clusters, cluster)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
	pool.Clusters = append(pool.Clusters, cluster)
	sc := &serviceComponentImpl{
		k8sNameSpace:       "test",
		env:                &config.Config{},
		spaceDockerRegBase: "http://test.com",
		modelDockerRegBase: "http://test.com",
		imagePullSecret:    "test",
		serviceStore:       kss,
		clusterPool:        pool,
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
			Gpu: types.GPU{
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
	pool.Clusters = append(pool.Clusters, cluster.Cluster{
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	_, err = sc.GetServicePodsWithStatus(ctx, pool.Clusters[0], "test", "test")
	require.Nil(t, err)
}

func TestServiceComponent_GetAllStatus(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := &cluster.ClusterPool{}
	pool.ClusterStore = mockdb.NewMockClusterInfoStore(t)
	kubeClient := fake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, cluster.Cluster{
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	kss.EXPECT().GetByCluster(ctx, "test").Return([]database.KnativeService{
		{
			Name: "test",
			ID:   1,
			Code: common.Running,
		},
	}, nil)
	status, err := sc.GetAllServiceStatus(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(status))
	require.Equal(t, common.Running, status["test"].Code)
}

func TestServiceComponent_GetServiceByName(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := &cluster.ClusterPool{}
	pool.ClusterStore = mockdb.NewMockClusterInfoStore(t)
	kubeClient := fake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, cluster.Cluster{
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
	pool.Clusters = append(pool.Clusters, cluster.Cluster{
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
	pool.Clusters = append(pool.Clusters, cluster.Cluster{
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
	err = sc.AddServiceInDB(*ksvc, "test")
	require.Nil(t, err)
}

func TestServiceComponent_updateServiceInDB(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, cluster.Cluster{
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
		Code:      common.Running,
	}, nil)
	ksvc, err := knativeClient.ServingV1().Services("test").
		Get(ctx, "test", metav1.GetOptions{})
	require.Nil(t, err)
	err = sc.UpdateServiceInDB(*ksvc, "test")
	require.Nil(t, err)
}

func TestServiceComponent_deleteServiceInDB2(t *testing.T) {
	cfg, err := config.LoadConfig()
	cfg.Accounting.ChargingEnable = true
	require.Nil(t, err)

	mq := mockmq.NewMockMessageQueue(t)
	mq.EXPECT().VerifyDeployServiceStream().Return(nil)
	mq.EXPECT().PublishDeployServiceData(mock.Anything).Return(nil)
	eventPub := event.EventPublisher{
		Connector:    mq,
		SyncInterval: cfg.Event.SyncInterval,
	}
	kss := mockdb.NewMockKnativeServiceStore(t)
	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	knativeClient := knativefake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, cluster.Cluster{
		CID:           "config",
		ID:            "test",
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
		eventPub:           &eventPub,
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
		SrvName:    "test",
		Hardware: types.HardWare{
			Gpu: types.GPU{
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
	err = sc.RunService(ctx, req)
	require.Nil(t, err)
	ksvc, err := knativeClient.ServingV1().Services("test").
		Get(ctx, "test", metav1.GetOptions{})
	require.Nil(t, err)
	kss.EXPECT().Delete(mock.Anything, "test", "test").Return(nil)
	err = sc.DeleteServiceInDB(*ksvc, "test")
	require.Nil(t, err)
}
