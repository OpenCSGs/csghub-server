package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetPodLog(t *testing.T) {
	podName := "test-pod"
	namespace := "test-ns"
	containerName := "test-container"
	logMessage := "fake logs"

	clientset := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
	})

	cluster := &Cluster{Client: clientset}
	logs, err := GetPodLog(context.Background(), cluster, podName, namespace, containerName)

	assert.NoError(t, err)
	assert.Equal(t, logMessage, string(logs))
}

func TestGetPod(t *testing.T) {
	podName := "test-pod"
	namespace := "test-ns"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
	}
	clientset := fake.NewSimpleClientset(pod)
	cluster := &Cluster{Client: clientset}

	retrievedPod, err := GetPod(context.Background(), cluster, podName, namespace)
	assert.NoError(t, err)
	assert.Equal(t, podName, retrievedPod.Name)
}

func TestGetPodLogStream(t *testing.T) {
	podName := "test-pod"
	namespace := "test-ns"
	containerName := "test-container"
	logMessage := "fake logs"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: containerName},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	clientset := fake.NewSimpleClientset(pod)
	logChan, msg, err := GetPodLogStream(context.Background(), clientset, pod, containerName, nil)

	assert.NoError(t, err)
	assert.Empty(t, msg)
	assert.NotNil(t, logChan)

	select {
	case logData := <-logChan:
		assert.Equal(t, logMessage, string(logData))
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for log message")
	}
}

func TestGetContainerName(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "container1"},
				{Name: "container2"},
			},
		},
	}

	// Test case 1: container name is empty, should return the first container name
	assert.Equal(t, "container1", GetContainerName(pod, ""))

	// Test case 2: container name is specified and exists
	assert.Equal(t, "container2", GetContainerName(pod, "container2"))

	// Test case 3: container name is specified but does not exist, should fallback to the first container
	assert.Equal(t, "container1", GetContainerName(pod, "non-existent-container"))

	// Test case 4: no containers in pod
	pod.Spec.Containers = []corev1.Container{}
	assert.Equal(t, "some-container", GetContainerName(pod, "some-container"))
}
