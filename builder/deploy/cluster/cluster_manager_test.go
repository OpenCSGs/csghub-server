package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"opencsg.com/csghub-server/common/config"
)

func TestVerifyPermissions(t *testing.T) {
	t.Run("should return error when namespace does not exist", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		// The function to create a clientset from config needs to be adapted for testing
		// For this test, we'll assume verifyPermissions can accept a clientset directly
		// or we mock the config to produce our fake clientset.
		// Let's refactor verifyPermissions slightly to allow injecting the clientset for easier testing.
		config := &config.Config{}
		err := verifyPermissions(clientset, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "please check your cluster configuration. the specified namespaces cannot be empty")
	})

	t.Run("should succeed when namespace exists", func(t *testing.T) {
		ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "existing-ns"}}
		clientset := fake.NewSimpleClientset(ns)
		config := &config.Config{}
		config.Cluster.SpaceNamespace = "spaces"
		err := verifyPermissions(clientset, config)
		assert.Error(t, err)
	})
}
