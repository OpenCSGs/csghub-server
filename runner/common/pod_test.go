package common

import (
	"context"
	"k8s.io/client-go/kubernetes"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetPodLogValidation(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	tests := []struct {
		name      string
		client    kubernetes.Interface
		podName   string
		namespace string
		container string
		wantErr   bool
	}{
		{
			name:      "test-pod",
			client:    clientset,
			podName:   "test-pod",
			namespace: "default",
			container: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetPodLog(context.Background(), tt.client, tt.podName, tt.namespace, tt.container)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
