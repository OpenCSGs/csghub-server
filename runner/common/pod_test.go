package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	"opencsg.com/csghub-server/builder/deploy/cluster"
)

func TestGetPodLogValidation(t *testing.T) {
	testCluster := &cluster.Cluster{
		Client: fake.NewSimpleClientset(),
	}

	tests := []struct {
		name      string
		cluster   *cluster.Cluster
		podName   string
		namespace string
		container string
		wantErr   bool
	}{
		{
			name:      "test-pod",
			cluster:   testCluster,
			podName:   "test-pod",
			namespace: "default",
			container: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetPodLog(context.Background(), tt.cluster, tt.podName, tt.namespace, tt.container)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
