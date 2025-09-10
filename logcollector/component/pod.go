package component

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Legacy cluster interface for backward compatibility
type Cluster struct {
	Client kubernetes.Interface
}

// GetPodLog gets pod logs (legacy function, kept for compatibility)
func GetPodLog(ctx context.Context, cluster *Cluster, podName string, namespace string, container string) ([]byte, error) {
	logs, err := cluster.Client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: container,
	}).DoRaw(ctx)
	return logs, err
}

// GetPod gets pod information (legacy function, kept for compatibility)
func GetPod(ctx context.Context, cluster *Cluster, podName string, namespace string) (*corev1.Pod, error) {
	pod, err := cluster.Client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		slog.Error("fail to get pod ", slog.Any("error", err), slog.String("pod name", podName))
		return nil, err
	}

	return pod, nil
}

// GetPodLogStream gets a streaming channel of pod logs
func GetPodLogStream(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod, container string, sinceTime *metav1.Time) (chan []byte, string, error) {
	logOptions := &corev1.PodLogOptions{
		Container:  container,
		Follow:     true,
		Timestamps: true, // Timestamps are useful for debugging and potential future logic
	}

	if sinceTime != nil {
		logOptions.SinceTime = sinceTime
	}

	if pod.Status.Phase == "Pending" {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "PodScheduled" && condition.Status == "False" {
				message := fmt.Sprintf("Pod is pending due to reason: %s, message: %s", condition.Reason, condition.Message)
				return nil, message, nil
			}
		}
	}

	ch := make(chan []byte, 1000)
	buf := make([]byte, 32*1024)

	logs := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOptions)

	// For streaming log requests, we must disable the client-side timeout.
	// The underlying http request will be kept alive until the context is cancelled.
	stream, err := logs.Timeout(0).Stream(ctx)
	if err != nil {
		return nil, "", err
	}

	go func() {
		defer close(ch)
		defer func() { _ = stream.Close() }()
		for {
			select {
			case <-ctx.Done():
				slog.Debug("logs request context done", slog.Any("error", ctx.Err()))
				return
			default:
				n, err := stream.Read(buf)
				if err != nil {
					slog.Debug("read pod logs finished", slog.Any("error", err), slog.String("pod", pod.Name))
					return
				}
				if n == 0 {
					time.Sleep(1000 * time.Millisecond)
					continue
				}

				if n > 0 {
					// Make a copy of the buffer to avoid data races
					data := make([]byte, n)
					copy(data, buf[:n])
					ch <- data
				}
			}
		}
	}()

	return ch, "", nil
}

// GetContainerName gets container name from pod (legacy function, kept for compatibility)
func GetContainerName(pod *corev1.Pod, container string) string {
	if container == "" && len(pod.Spec.Containers) > 0 {
		return pod.Spec.Containers[0].Name
	}

	for _, c := range pod.Spec.Containers {
		if c.Name == container {
			return c.Name
		}
	}

	// Fallback to first container if specified container not found
	if len(pod.Spec.Containers) > 0 {
		return pod.Spec.Containers[0].Name
	}

	return container
}
