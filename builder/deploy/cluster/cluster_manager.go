package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"path/filepath"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	knative "knative.dev/serving/pkg/client/clientset/versioned"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// Cluster holds basic information about a Kubernetes cluster
type Cluster struct {
	ID            string                // Unique identifier for the cluster
	ConfigPath    string                // Path to the kubeconfig file
	Client        *kubernetes.Clientset // Kubernetes client
	KnativeClient *knative.Clientset    // Knative client
	StorageClass  string
}

// ClusterPool is a resource pool of cluster information
type ClusterPool struct {
	Clusters     []Cluster
	ClusterStore *database.ClusterInfoStore
}

// NewClusterPool initializes and returns a ClusterPool by reading kubeconfig files from $HOME/.kube directory
func NewClusterPool() (*ClusterPool, error) {
	pool := &ClusterPool{}
	pool.ClusterStore = database.NewClusterInfoStore()

	home := homedir.HomeDir()
	kubeconfigFolderPath := filepath.Join(home, ".kube")
	kubeconfigFiles, err := filepath.Glob(filepath.Join(kubeconfigFolderPath, "config*"))
	if err != nil {
		return nil, err
	}

	if len(kubeconfigFiles) == 0 {
		slog.Error("No kubeconfig files", slog.Any("path", kubeconfigFolderPath))
	}

	for i, kubeconfig := range kubeconfigFiles {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
		knativeClient, err := knative.NewForConfig(config)
		if err != nil {
			slog.Error("falied to create knative client", "error", err)
			return nil, fmt.Errorf("falied to create knative client,%w", err)
		}
		id := filepath.Base(kubeconfig)
		pool.Clusters = append(pool.Clusters, Cluster{
			ID:            id,
			ConfigPath:    kubeconfig,
			Client:        client,
			KnativeClient: knativeClient,
		})
		err = pool.ClusterStore.Add(context.TODO(), id, fmt.Sprintf("region-%d", i))
		if err != nil {
			slog.Error("falied to add cluster info to db", "error", err)
			return nil, fmt.Errorf("falied to add cluster info to db,%w", err)
		}
	}

	return pool, nil
}

// SelectCluster selects the most appropriate cluster to deploy the service to
func (p *ClusterPool) GetCluster() (*Cluster, error) {
	if len(p.Clusters) == 0 {
		return nil, fmt.Errorf("no available clusters")
	}
	// Randomly choose a cluster to deploy the service to
	// to do: The cluster should be selected based on criteria such as availability, performance, load, etc.
	randomIndex := rand.Intn(len(p.Clusters))

	// Select a cluster using the random index
	selectedCluster := p.Clusters[randomIndex]
	return &selectedCluster, nil
}

// GetClusterByID retrieves a cluster from the pool given its unique ID
func (p *ClusterPool) GetClusterByID(ctx context.Context, id string) (*Cluster, error) {
	cfId := "config"
	storageClass := ""
	if len(id) != 0 {
		cInfo, _ := p.ClusterStore.ByClusterID(ctx, id)
		cfId = cInfo.ClusterConfig
		storageClass = cInfo.StorageClass
	}
	for _, Cluster := range p.Clusters {
		if Cluster.ID == cfId {
			Cluster.StorageClass = storageClass
			return &Cluster, nil
		}
	}
	return nil, fmt.Errorf("cluster with the given ID does not exist")
}

// getNodeResources retrieves all node cpu and gpu info
func GetNodeResources(clientset *kubernetes.Clientset, config *config.Config) (map[string]types.NodeResourceInfo, error) {
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodeResourcesMap := make(map[string]types.NodeResourceInfo)

	for _, node := range nodes.Items {
		memCapacity := node.Status.Capacity["memory"]
		memQuantity, ok := memCapacity.AsInt64()
		if !ok {
			slog.Error("falied to get node memory", "node", node.Name, "error", err)
			continue
		}
		totalMem := getMem(memQuantity)
		totalCPU := node.Status.Capacity.Cpu().MilliValue()
		totalXPU := resource.Quantity{}
		xpuCapacityLabel, xpuTypeLabel := getXPULabel(node, config)
		if xpuCapacityLabel != "" {
			totalXPU = node.Status.Capacity[v1.ResourceName(xpuCapacityLabel)]
		}

		gpuModelVendor := strings.Split(node.Labels[xpuTypeLabel], "-")
		gpuModel := ""
		if len(gpuModelVendor) > 1 {
			gpuModel = gpuModelVendor[1]
		}
		nodeResourcesMap[node.Name] = types.NodeResourceInfo{
			NodeName:         node.Name,
			TotalCPU:         millicoresToCores(totalCPU),
			AvailableCPU:     millicoresToCores(totalCPU),
			XPUModel:         gpuModel,
			GPUVendor:        gpuModelVendor[0],
			TotalXPU:         parseQuantityToInt64(totalXPU),
			AvailableXPU:     parseQuantityToInt64(totalXPU),
			AvailableMem:     totalMem,
			TotalMem:         totalMem,
			XPUCapacityLabel: xpuCapacityLabel,
		}
	}
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == "" || pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			continue
		}

		nodeResource := nodeResourcesMap[pod.Spec.NodeName]
		for _, container := range pod.Spec.Containers {
			if requestedGPU, hasGPU := container.Resources.Requests[v1.ResourceName(nodeResource.XPUCapacityLabel)]; hasGPU {
				nodeResource.AvailableXPU -= parseQuantityToInt64(requestedGPU)
			}
			if memoryRequest, hasMemory := container.Resources.Requests[v1.ResourceMemory]; hasMemory {
				nodeResource.AvailableMem -= getMem(parseQuantityToInt64(memoryRequest))
			}
			if cpuRequest, hasCPU := container.Resources.Requests[v1.ResourceCPU]; hasCPU {
				nodeResource.AvailableCPU -= millicoresToCores(cpuRequest.MilliValue())
			}
		}

		nodeResourcesMap[pod.Spec.NodeName] = nodeResource
	}

	return nodeResourcesMap, nil
}

func getXPULabel(node v1.Node, config *config.Config) (string, string) {
	if _, found := node.Labels[config.Space.GPUModelLabel]; found {
		//for default clsuter
		return "nvidia.com/gpu", config.Space.GPUModelLabel
	}
	if _, found := node.Labels["kubemore_xpu_type"]; found {
		//for huawei gpu
		return "huawei.com/Ascend910", "kubemore_xpu_type"
	}
	if _, found := node.Labels["kubemore_xpu_type"]; found {
		//for huawei gpu
		return "huawei.com/Ascend910", "kubemore_xpu_type"
	}
	if _, found := node.Labels["accelerator/huawei-npu"]; found {
		//for huawei gpu
		return "huawei.com/Ascend910", "accelerator/huawei-npu"
	}
	return "", ""
}

func getMem(memByte int64) float32 {
	memGB := float32(memByte) / (1024 * 1024 * 1024)
	return memGB
}

func millicoresToCores(millicores int64) float64 {
	cores := float64(millicores) / 1000.0
	return math.Round(cores*10) / 10
}

func parseQuantityToInt64(q resource.Quantity) int64 {
	if q.IsZero() {
		return 0
	}
	value, _ := q.AsInt64()
	return value
}
