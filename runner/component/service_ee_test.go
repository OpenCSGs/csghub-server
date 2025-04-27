//go:build ee || saas

package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	knativefake "knative.dev/serving/pkg/client/clientset/versioned/fake"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	lwsfake "sigs.k8s.io/lws/client-go/clientset/versioned/fake"
)

func TestServiceComponent_RunServiceWithMutiHost(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	kubeClient := fake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
		LWSClient:     lwsfake.NewSimpleClientset(),
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
		ClusterID:  "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory:   "16Gi",
			Replicas: 2,
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	cis.EXPECT().ByClusterID(ctx, "test").Return(database.ClusterInfo{
		ClusterID:     "test",
		ClusterConfig: "config",
		StorageClass:  "test",
	}, nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	//query the lws service
	lws, err := pool.Clusters[0].LWSClient.LeaderworkersetV1().LeaderWorkerSets("test").Get(ctx, "test-lws", metav1.GetOptions{})
	require.Nil(t, err)
	require.Equal(t, "test-lws", lws.Name)
	ks, err := pool.Clusters[0].KnativeClient.ServingV1().Services("test").Get(ctx, "test", metav1.GetOptions{})
	require.Nil(t, err)
	require.Equal(t, "test", ks.Name)
}

func TestServiceComponent_StopServiceWithMutiHost(t *testing.T) {
	kss := mockdb.NewMockKnativeServiceStore(t)
	ctx := context.TODO()
	pool := &cluster.ClusterPool{}
	cis := mockdb.NewMockClusterInfoStore(t)
	pool.ClusterStore = cis
	kubeClient := fake.NewSimpleClientset()
	pool.Clusters = append(pool.Clusters, cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
		LWSClient:     lwsfake.NewSimpleClientset(),
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
		ClusterID:  "test",
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:  "1",
				Type: "A10",
			},
			Memory:   "16Gi",
			Replicas: 2,
		},
		Env: map[string]string{
			"test": "test",
			"port": "8000",
		},
		Annotation: map[string]string{},
	}
	cis.EXPECT().ByClusterID(ctx, "test").Return(database.ClusterInfo{
		ClusterID:     "test",
		ClusterConfig: "config",
		StorageClass:  "test",
	}, nil)
	err := sc.RunService(ctx, req)
	require.Nil(t, err)
	//query the lws service
	lws, err := pool.Clusters[0].LWSClient.LeaderworkersetV1().LeaderWorkerSets("test").Get(ctx, "test-lws", metav1.GetOptions{})
	require.Nil(t, err)
	require.Equal(t, "test-lws", lws.Name)
	ks, err := pool.Clusters[0].KnativeClient.ServingV1().Services("test").Get(ctx, "test", metav1.GetOptions{})
	require.Nil(t, err)
	require.Equal(t, "test", ks.Name)

	kss.EXPECT().Delete(ctx, "test", "test").Return(nil)
	resp, err := sc.StopService(ctx, types.StopRequest{
		SvcName:   "test",
		ClusterID: "test",
	})
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, resp.Code, 0)
	//query the lws service
	_, err = pool.Clusters[0].LWSClient.LeaderworkersetV1().LeaderWorkerSets("test").Get(ctx, "test-lws", metav1.GetOptions{})
	require.NotNil(t, err)
}
