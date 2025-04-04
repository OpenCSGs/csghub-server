package component

import (
	"context"
	"testing"

	argofake "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/runner/types"
)

func TestImagebuilderComponent_Build(t *testing.T) {
	conf := &config.Config{}
	testCluster := &cluster.Cluster{
		CID:        "config",
		ID:         "config",
		Client:     fake.NewSimpleClientset(),
		ArgoClient: argofake.NewSimpleClientset(),
	}

	db := tests.InitTestDB()
	defer db.Close()
	ibc := &imagebuilderComponentImpl{
		clusterPool: &cluster.ClusterPool{
			Clusters: []cluster.Cluster{*testCluster},
		},
		config: conf,
		db:     database.NewImageBuilderStoreWithDB(db),
	}

	_, err := ibc.Build(context.Background(), types.SpaceBuilderConfig{
		OrgName:   "test-org",
		SpaceName: "test-space",
		BuildId:   "test-build-id",
		SpaceURL:  "https://github.com/test-org/test-space",
	})

	require.Nil(t, err)
}
