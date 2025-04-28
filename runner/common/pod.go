package common

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
)

func GetPodLog(ctx context.Context, cluster *cluster.Cluster, podName string, namespace string, container string) ([]byte, error) {
	logs, err := cluster.Client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: container,
	}).DoRaw(ctx)
	return logs, err
}

func GetPod(ctx context.Context, cluster *cluster.Cluster, podName string, namespace string) (*corev1.Pod, error) {
	pod, err := cluster.Client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		slog.Error("fail to get pod ", slog.Any("error", err), slog.String("pod name", podName))
		return nil, err
	}

	return pod, nil
}

func GetPodLogStream(ctx context.Context, cluster *cluster.Cluster, podName string, namespace string, container string) (chan []byte, string, error) {

	pod, err := GetPod(ctx, cluster, podName, namespace)
	if err != nil {
		return nil, "", err
	}

	cName := GetContainerName(pod, container)
	logs := cluster.Client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: cName,
		Follow:    true,
	})

	if pod.Status.Phase == "Pending" {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "PodScheduled" && condition.Status == "False" {
				message := fmt.Sprintf("Pod is pending due to reason: %s, message: %s", condition.Reason, condition.Message)
				return nil, message, nil
			}
		}
	}

	ch := make(chan []byte)
	buf := make([]byte, 32*1024)

	stream, err := logs.Stream(context.Background())
	if err != nil {
		return nil, "", err
	}

	go func() {
		defer close(ch)
		defer stream.Close()
		for {
			select {
			case <-ctx.Done():
				slog.Info("logs request context done", slog.Any("error", ctx.Err()))
				return
			default:
				n, err := stream.Read(buf)
				if err != nil {
					slog.Error("read pod logs failed", slog.Any("error", err))
					return
				}
				if n == 0 {
					time.Sleep(5 * time.Second)
				}

				if n > 0 {
					ch <- buf[:n]
				}
			}
		}
	}()

	return ch, "", nil
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
