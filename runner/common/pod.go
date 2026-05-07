package common

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"opencsg.com/csghub-server/builder/deploy/cluster"
	"k8s.io/client-go/kubernetes"
)

func GetPodLog(ctx context.Context, client kubernetes.Interface, podName string, namespace string, container string) ([]byte, error) {
	logs, err := client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: container,
	}).DoRaw(ctx)
	return logs, err
}

func GetPod(ctx context.Context, client kubernetes.Interface, podName string, namespace string) (*corev1.Pod, error) {
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		slog.Error("fail to get pod ", slog.Any("error", err), slog.String("pod name", podName))
		return nil, err
	}

	return pod, nil
}

func GetPodLogStream(ctx context.Context, client kubernetes.Interface, podName string, namespace string, container string) (stream io.ReadCloser, message string, err error) {
	pod, err := GetPod(ctx, client, podName, namespace)
	if err != nil {
		return nil, "", err
	}

	cName := GetContainerName(pod, container)
	logs := client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: cName,
		Follow:    true,
	})

	if pod.Status.Phase == "Pending" {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "PodScheduled" && condition.Status == "False" {
				message = fmt.Sprintf("Pod is pending due to reason: %s, message: %s", condition.Reason, condition.Message)
				return nil, message, nil
			}
		}
	}
	stream, err = logs.Stream(ctx)
	return stream, message, err
}

// get container name
func GetContainerName(pod *corev1.Pod, container string) string {
	name := pod.Spec.Containers[0].Name
	for _, c := range pod.Spec.Containers {
		if c.Name == container {
			name = c.Name
		}
	}
	return name
}
