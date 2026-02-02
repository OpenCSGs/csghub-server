package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	argo "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	units "github.com/dustin/go-humanize"
	authorizationv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	knative "knative.dev/serving/pkg/client/clientset/versioned"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	rtypes "opencsg.com/csghub-server/runner/types"
	lwscli "sigs.k8s.io/lws/client-go/clientset/versioned"
)

// Cluster holds basic information about a Kubernetes cluster

type Cluster struct {
	CID              string               // config id
	ID               string               // unique id
	ConfigPath       string               // Path to the kubeconfig file
	Client           kubernetes.Interface // Kubernetes client
	KnativeClient    knative.Interface    // Knative client
	LWSClient        lwscli.Interface     // LWS client
	ArgoClient       argo.Interface       // Argo client
	StorageClass     string
	NetworkInterface string            // Main network interface, used to rdma, ex: eth0
	ConnectMode      types.ClusterMode // InCluster | kubeconfig
	Region           string
	Nodes            []types.Node
}

// Pool is a resource pool of cluster information
type pool struct {
	Clusters     []*Cluster
	ClusterStore database.ClusterInfoStore
	Config       *config.Config // Configuration for the cluster pool
}

type Pool interface {
	GetCluster() (*Cluster, error)
	GetClusterByID(ctx context.Context, id string) (*Cluster, error)
	GetAllCluster() []*Cluster
	Update(ctx context.Context, cluster *database.ClusterInfo) error
}

// NewClusterPool initializes and returns a ClusterPool by auto-detecting whether it's running in a cluster or using local kubeconfig files.
func NewClusterPool(config *config.Config) (Pool, error) {
	pool := &pool{}
	pool.Config = config

	// Try in-cluster config first
	err := tryInClusterConfig(pool, config)
	if err == nil {
		pool.ClusterStore = database.NewClusterInfoStore()
		slog.Info("Successfully initialized cluster pool", slog.Any("mode", pool.Clusters[0].ConnectMode))
		return pool, nil
	}
	slog.Warn("In-cluster config failed, falling back to kubeconfig files", slog.Any("reason", err))
	// Fallback to kubeconfig files
	if err := tryKubeconfigFiles(pool, config); err != nil {
		return nil, fmt.Errorf("failed to initialize cluster pool from $HOME/.kube/ files: %w", err)
	}
	if len(pool.Clusters) == 0 {
		return nil, fmt.Errorf("no clusters found in $HOME/.kube/")
	}
	slog.Info("Successfully initialized cluster pool using $HOME/.kube/", slog.Any("mode", pool.Clusters[0].ConnectMode))
	pool.ClusterStore = database.NewClusterInfoStore()
	return pool, nil
}

func tryInClusterConfig(pool *pool, config *config.Config) error {
	slog.Info("Attempting to connect to Kubernetes using in-cluster config")
	kubeconfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to load in-cluster config: %w", err)
	}

	slog.Info("Successfully loaded in-cluster config, verifying permissions...")
	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}
	if err := verifyPermissions(clientset, config); err != nil {
		return fmt.Errorf("in-cluster permission check failed: %w", err)
	}

	slog.Info("In-cluster permission check successful")
	c, err := buildCluster(kubeconfig, types.DefaultClusterCongfig, 0, types.ConnectModeInCluster, config)
	if err == nil {
		slog.Info("Successfully built cluster from in-cluster config")
		pool.Clusters = append(pool.Clusters, c)
		return nil
	}

	return fmt.Errorf("failed to build cluster from in-cluster config: %w", err)
}

func tryKubeconfigFiles(pool *pool, config *config.Config) error {
	slog.Info("Attempting to connect to Kubernetes using kubeconfig files from home directory")
	home := homedir.HomeDir()
	if home == "" {
		return fmt.Errorf("home directory not found")
	}
	kubeconfigFolderPath := filepath.Join(home, ".kube")
	kubeconfigFiles, err := filepath.Glob(filepath.Join(kubeconfigFolderPath, "config*"))
	if err != nil {
		return fmt.Errorf("error finding kubeconfig files: %w", err)
	}

	if len(kubeconfigFiles) == 0 {
		return fmt.Errorf("no kubeconfig files found in %s", kubeconfigFolderPath)
	}

	slog.Info("Found kubeconfig files", "files", kubeconfigFiles)
	for i, kubeconfigPath := range kubeconfigFiles {
		slog.Info("Loading kubeconfig", "path", kubeconfigPath)
		kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			slog.Warn("Failed to build config from kubeconfig file", "path", kubeconfigPath, "error", err)
			continue
		}

		id := filepath.Base(kubeconfigPath)
		if c, err := buildCluster(kubeconfig, id, i, types.ConnectModeKubeConfig, config); err != nil {
			slog.Warn("Failed to build cluster from kubeconfig", "path", kubeconfigPath, "error", err, slog.String("id", id))
		} else {
			if c != nil {
				pool.Clusters = append(pool.Clusters, c)
			}
		}
	}
	return nil
}

// verifyPermissions checks if the provided kubeconfig has enough permissions for runner operations.
func verifyPermissions(clientset kubernetes.Interface, config *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Check if the provided kubeconfig has enough permissions for runner operations.
	// Cross namespace access is required ClusterRole
	namespaces := []string{
		config.Cluster.SpaceNamespace,
	}
	for _, ns := range namespaces {
		if len(ns) == 0 {
			return fmt.Errorf("please check your cluster configuration. the specified namespaces cannot be empty")
		}
		// 1. First, check if the namespace exists.
		_, err := clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get namespace '%s', please check if it exists and you have permissions: %w", ns, err)
		}
	}

	// 2. Then, check for detailed permissions within the namespace using SelfSubjectAccessReview.
	// Define required permissions that will be checked in namespace
	permissions := []authorizationv1.ResourceAttributes{
		{Group: "", Resource: "pods", Verb: "list"},
		{Group: "", Resource: "pods", Verb: "watch"},
		{Group: "", Resource: "pods", Verb: "get"},
		{Group: "", Resource: "pods/log", Verb: "get"},
		{Group: "", Resource: "services", Verb: "list"},
		{Group: "", Resource: "services", Verb: "watch"},
		{Group: "", Resource: "configmaps", Verb: "list"},
		{Group: "", Resource: "configmaps", Verb: "watch"},
	}
	for _, p := range permissions {
		p.Namespace = config.Cluster.SpaceNamespace
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &p,
			},
		}
		response, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to perform permission check for %s on %s in namespace %s: %w", p.Verb, p.Resource, p.Namespace, err)
		}

		if !response.Status.Allowed {
			reason := response.Status.Reason
			if reason == "" {
				reason = "reason not provided by API server"
			}
			return fmt.Errorf("permission denied to %s %s in namespace %s: %s", p.Verb, p.Resource, p.Namespace, reason)
		}
	}

	slog.Info("All required permissions are verified for relevant namespaces.")
	return nil
}

func buildCluster(kubeconfig *rest.Config, id string, index int, connectMode types.ClusterMode, config *config.Config) (*Cluster, error) {
	var err error
	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	// Check client connection with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.Discovery().RESTClient().Get().AbsPath("/version").Do(ctx).Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to cluster, %w", err)
	}

	argoClient, err := argo.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	knativeClient, err := knative.NewForConfig(kubeconfig)
	if err != nil {
		slog.Error("failed to create knative client", "error", err)
		return nil, fmt.Errorf("failed to create knative client,%w", err)
	}
	lwsclient, err := lwscli.NewForConfig(kubeconfig)
	if err != nil {
		slog.Error("failed to create lws client", "error", err)
		return nil, fmt.Errorf("failed to create lws client,%w", err)
	}
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if connectMode == types.ConnectModeInCluster {
		if err := database.InitInMemoryDB(); err != nil {
			return nil, fmt.Errorf("failed to init in memory db, %w", err)
		}
	} else {
		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}
		if err := database.InitDB(dbConfig); err != nil {
			return nil, fmt.Errorf("failed to init db, %w", err)
		}
	}
	clusterStore := database.NewClusterInfoStore()
	region := fmt.Sprintf("region-%d", index)

	// Read StorageClass and NetworkInterface from ConfigMap before AddByClusterID
	storageClass := ""
	networkInterface := ""
	if connectMode == types.ConnectModeInCluster && len(config.Runner.RunnerNamespace) > 0 && len(config.Runner.WatchConfigmapName) > 0 {
		ctxConfigMap, cancelConfigMap := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelConfigMap()
		cm, err := client.CoreV1().ConfigMaps(config.Runner.RunnerNamespace).Get(ctxConfigMap, config.Runner.WatchConfigmapName, metav1.GetOptions{})
		if err == nil && cm != nil {
			if val, ok := cm.Data[rtypes.KeyStorageClass]; ok && len(val) > 0 {
				storageClass = val
				slog.Debug("read storage class from ConfigMap", slog.String("storage_class", storageClass))
			}
			if val, ok := cm.Data[rtypes.KeyNetworkInterface]; ok && len(val) > 0 {
				networkInterface = val
				slog.Debug("read network interface from ConfigMap", slog.String("network_interface", networkInterface))
			}
		} else if err != nil {
			slog.Debug("failed to read ConfigMap during initialization, will use default values", slog.String("namespace", config.Runner.RunnerNamespace), slog.String("configmap", config.Runner.WatchConfigmapName), slog.Any("error", err))
		}
	}

	var cluster *database.ClusterInfo
	if connectMode == types.ConnectModeKubeConfig {
		cluster, err = clusterStore.Add(ctxTimeout, id, region, types.ConnectModeKubeConfig)
	} else {
		region = config.Cluster.Region
		var clusterID string
		clusterID, err = GetClusterID(client, config)
		if err != nil {
			return nil, fmt.Errorf("failed to get cluster id,%w", err)
		}
		cluster, err = clusterStore.AddByClusterID(ctxTimeout, clusterID, region)
	}
	if err != nil {
		slog.Error("failed to add cluster info to db", slog.Any("error", err), slog.Any("config id", id))
		return nil, fmt.Errorf("failed to add cluster info to db error: %w", err)
	}
	if !cluster.Enable {
		return nil, nil
	}

	// Update StorageClass and NetworkInterface to database if read from ConfigMap
	if len(storageClass) > 0 || len(networkInterface) > 0 {
		updateEvent := types.ClusterEvent{
			ClusterID:        cluster.ClusterID,
			ClusterConfig:    cluster.ClusterConfig,
			Region:           cluster.Region,
			Zone:             cluster.Zone,
			Provider:         cluster.Provider,
			StorageClass:     cluster.StorageClass,     // Use existing value as default
			NetworkInterface: cluster.NetworkInterface, // Use existing value as default
			Mode:             cluster.Mode,
			Endpoint:         cluster.RunnerEndpoint,
			AppEndpoint:      cluster.AppEndpoint,
		}
		// Override with values from ConfigMap if available
		if len(storageClass) > 0 {
			updateEvent.StorageClass = storageClass
			cluster.StorageClass = storageClass
		}
		if len(networkInterface) > 0 {
			updateEvent.NetworkInterface = networkInterface
			cluster.NetworkInterface = networkInterface
		}
		err = clusterStore.UpdateByClusterID(ctxTimeout, updateEvent)
		if err != nil {
			slog.Warn("failed to update cluster info from ConfigMap", slog.Any("error", err))
		} else {
			slog.Info("updated cluster info from ConfigMap", slog.String("storage_class", updateEvent.StorageClass), slog.String("network_interface", updateEvent.NetworkInterface))
		}
	}

	c := &Cluster{
		CID:              id,
		ID:               cluster.ClusterID,
		Client:           client,
		KnativeClient:    knativeClient,
		ArgoClient:       argoClient,
		LWSClient:        lwsclient,
		ConnectMode:      connectMode,
		Region:           region,
		StorageClass:     cluster.StorageClass,
		NetworkInterface: cluster.NetworkInterface,
	}
	return c, nil
}

// GetCluster selects the most appropriate cluster to deploy the service to
func (p *pool) GetCluster() (*Cluster, error) {
	if len(p.Clusters) == 0 {
		return nil, fmt.Errorf("no available clusters")
	}
	// Randomly choose a cluster to deploy the service to
	// to do: The cluster should be selected based on criteria such as availability, performance, load, etc.
	randomIndex := rand.Intn(len(p.Clusters))

	// Select a cluster using the random index
	selectedCluster := p.Clusters[randomIndex]
	return selectedCluster, nil
}

// GetClusterByID retrieves a cluster from the pool given its unique ID
func (p *pool) GetClusterByID(ctx context.Context, id string) (*Cluster, error) {
	cfId := "config"
	storageClass := ""
	networkInterface := "eth0"
	if len(id) != 0 {
		cInfo, _ := p.ClusterStore.ByClusterID(ctx, id)
		cfId = cInfo.ClusterConfig
		storageClass = cInfo.StorageClass
		networkInterface = cInfo.NetworkInterface
	}
	for _, Cluster := range p.Clusters {
		if Cluster.CID == cfId {
			if len(Cluster.StorageClass) < 1 && len(storageClass) > 0 {
				Cluster.StorageClass = storageClass
			}
			Cluster.NetworkInterface = networkInterface
			return Cluster, nil
		}
	}
	return nil, fmt.Errorf("cluster with the given ID does not exist")
}

func (p *pool) GetAllCluster() []*Cluster {
	return p.Clusters
}

func (p *pool) Update(ctx context.Context, clusterInfo *database.ClusterInfo) error {
	return p.ClusterStore.Update(ctx, *clusterInfo)
}

// GetResourcesInCluster retrieves all node cpu and gpu info
func (cluster *Cluster) GetResourcesInCluster(config *config.Config) (map[string]types.NodeResourceInfo, error) {
	clientset := cluster.Client
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodeResourcesMap := make(map[string]types.NodeResourceInfo)

	cluster.Nodes = []types.Node{}
	for _, node := range nodes.Items {
		nodeName := node.Name
		nodeInfo := collectNodeResource(node, config)
		nodeResourcesMap[nodeName] = nodeInfo
		cluster.Nodes = append(cluster.Nodes, types.Node{
			Name:       nodeName,
			EnableVXPU: nodeInfo.EnableVXPU,
		})
	}

	for _, pod := range pods.Items {
		if pod.Spec.NodeName == "" || pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			continue
		}

		nodeResource, ok := nodeResourcesMap[pod.Spec.NodeName]
		if !ok {
			continue
		}

		collectNodePodsResource(pod, config, &nodeResource)

		nodeResourcesMap[pod.Spec.NodeName] = nodeResource
	}

	for k, v := range nodeResourcesMap {
		nodeResourcesMap[k] = *calcSingleNodeXPUMem(&v)
	}

	return nodeResourcesMap, nil
}

func collectNodeResource(node v1.Node, config *config.Config) types.NodeResourceInfo {
	ready, status := checkNodeReadiness(node)

	if !ready {
		return types.NodeResourceInfo{
			NodeName:   node.Name,
			NodeStatus: status,
			Labels:     node.Labels,
			Processes:  []types.ProcessInfo{},
		}
	}

	totalMem := node.Status.Capacity.Memory().DeepCopy()
	totalCPU := node.Status.Capacity.Cpu().DeepCopy()
	allocatableMem := node.Status.Allocatable.Memory().DeepCopy()
	allocatableCPU := node.Status.Allocatable.Cpu().DeepCopy()
	totalXPU := resource.Quantity{}
	allocatableXPU := resource.Quantity{}
	xpuCapacityLabel, xpuTypeLabel, xpuMemLabel := getXPULabel(node.Labels, config)
	if xpuCapacityLabel != "" {
		totalXPU = node.Status.Capacity[v1.ResourceName(xpuCapacityLabel)]
		allocatableXPU = node.Status.Allocatable[v1.ResourceName(xpuCapacityLabel)]
	}

	bigXPUMem := ""
	if xpuMemLabel != "" {
		ulimit, err := units.ParseBigBytes(node.Labels[xpuMemLabel])
		if err == nil {
			bigXPUMem = units.BigIBytes(ulimit)
		}
	}

	gpuModelVendor, gpuModel := getGpuTypeAndVendor(node.Labels[xpuTypeLabel], xpuCapacityLabel)
	vXPUs := collectNodeVXPU(node)
	nodeResourceInfo := types.NodeResourceInfo{
		NodeName:   node.Name,
		NodeStatus: status,
		Labels:     node.Labels,
		NodeHardware: types.NodeHardware{
			TotalCPU:         millicoresToCores(totalCPU.MilliValue()),
			AvailableCPU:     millicoresToCores(allocatableCPU.MilliValue()),
			TotalMem:         getMem(totalMem.Value()),
			AvailableMem:     getMem(allocatableMem.Value()),
			XPUModel:         gpuModel,
			GPUVendor:        gpuModelVendor,
			TotalXPU:         parseQuantityToInt64(totalXPU),
			AvailableXPU:     parseQuantityToInt64(allocatableXPU),
			XPUCapacityLabel: xpuCapacityLabel,
			XPUMem:           bigXPUMem,
			VXPUs:            vXPUs,
		},
		Processes:  []types.ProcessInfo{},
		EnableVXPU: (len(vXPUs) > 0),
	}

	return nodeResourceInfo
}

func collectNodePodsResource(pod v1.Pod, config *config.Config, nodeResource *types.NodeResourceInfo) {
	for _, container := range pod.Spec.Containers {
		if requestedGPU, hasGPU := container.Resources.Requests[v1.ResourceName(nodeResource.XPUCapacityLabel)]; hasGPU {
			nodeResource.AvailableXPU -= parseQuantityToInt64(requestedGPU)
		}
		if memoryRequest, hasMemory := container.Resources.Requests[v1.ResourceMemory]; hasMemory {
			nodeResource.AvailableMem -= getMem(memoryRequest.Value())
		}
		if cpuRequest, hasCPU := container.Resources.Requests[v1.ResourceCPU]; hasCPU {
			nodeResource.AvailableCPU -= millicoresToCores(cpuRequest.MilliValue())
		}
	}

	if pod.Namespace == config.Cluster.SpaceNamespace {
		deployIDStr := pod.Labels[types.StreamKeyDeployID]
		deployID, err := strconv.ParseInt(deployIDStr, 10, 64)
		if err != nil {
			slog.Warn("failed to convert csghub_deploy_id in collecting node pod resource")
		}
		nodeResource.Processes = append(nodeResource.Processes, types.ProcessInfo{
			PodName:  pod.Name,
			DeployID: deployID,
			SvcName:  pod.Labels[rtypes.KnativeConfigLabelName],
			VXPUs:    collectPodVXPU(pod),
		})
	}
}

// GetClusterID retrieves the unique ID of the cluster by fetching the UID of the specified namespace.
func GetClusterID(clientset kubernetes.Interface, config *config.Config) (string, error) {
	if len(config.Cluster.ClusterID) != 0 {
		return config.Cluster.ClusterID, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ns, err := clientset.CoreV1().Namespaces().Get(ctx, config.Cluster.SpaceNamespace, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get kube-system namespace: %w", err)
	}
	return string(ns.UID), nil
}

// return the gpu vendor and type
func getGpuTypeAndVendor(vendorType string, label string) (string, string) {
	if strings.Contains(vendorType, "-") {
		gpuModelVendor := strings.SplitN(vendorType, "-", 2)
		return gpuModelVendor[0], gpuModelVendor[1]
	}
	if strings.Contains(label, ".") {
		gpuModelVendor := strings.Split(label, ".")
		return gpuModelVendor[0], vendorType
	}
	return label, vendorType
}

// the first label is the xpu capacity label, the second is the gpu model label
func getXPULabel(labels map[string]string, config *config.Config) (string, string, string) {
	if _, found := labels["aliyun.accelerator/nvidia_name"]; found {
		//for default cluster
		return "nvidia.com/gpu", "aliyun.accelerator/nvidia_name", "aliyun.accelerator/nvidia_mem"
	}
	if _, found := labels["machine.cluster.vke.volcengine.com/gpu-name"]; found {
		//for volcano cluster
		return "nvidia.com/gpu", "machine.cluster.vke.volcengine.com/gpu-name", "machine.cluster.vke.volcengine.com/gpu-mem"
	}
	if _, found := labels["eks.tke.cloud.tencent.com/gpu-type"]; found {
		//for tencent cluster
		return "nvidia.com/gpu", "eks.tke.cloud.tencent.com/gpu-type", "eks.tke.cloud.tencent.com/gpu-mem"
	}
	if _, found := labels["nvidia.com/nvidia_name"]; found {
		//for k3s cluster
		return "nvidia.com/gpu", "nvidia.com/nvidia_name", "nvidia.com/nvidia_mem"
	}
	if _, found := labels["nvidia.com/gpu.product"]; found {
		//for nvidia gpu product label
		return "nvidia.com/gpu", "nvidia.com/gpu.product", "nvidia.com/gpu.mem"
	}
	if _, found := labels["kubemore_xpu_type"]; found {
		//for huawei gpu
		return "huawei.com/Ascend910", "kubemore_xpu_type", "kubemore_xpu_mem"
	}
	if _, found := labels["huawei.accelerator"]; found {
		//for huawei gpu
		return "huawei.com/Ascend910", "huawei.accelerator", "huawei.accelerator.mem"
	}
	if _, found := labels["accelerator/huawei-npu"]; found {
		//for huawei gpu
		return "huawei.com/Ascend910", "accelerator/huawei-npu", "accelerator/huawei-npu.mem"
	}
	if _, found := labels["hygon.com/dcu.name"]; found {
		//for hy dcu
		return "hygon.com/dcu", "hygon.com/dcu.name", "hygon.com/dcu.mem"
	}
	if _, found := labels["enflame.com/gcu"]; found {
		//for enflame gcu
		return "enflame.com/gcu", "enflame.com/gcu.model", "enflame.com/gcu.mem"
	}
	if _, found := labels["enflame.com/gcu.count"]; found {
		//for enflame gcu
		return "enflame.com/gcu.count", "enflame.com/gcu.model", "enflame.com/gcu.mem"
	}
	if _, found := labels["amd.com/gpu"]; found {
		//for enflame gcu
		return "amd.com/gpu", "amd.com/gpu.product-name", "amd.com/gpu.vram"
	}
	//check custom gpu model label
	if config.Space.GPUModelLabel != "" {
		var gpuLabels []types.GPUModel
		err := json.Unmarshal([]byte(config.Space.GPUModelLabel), &gpuLabels)
		if err != nil {
			slog.Warn("failed to parse custom GPUModelLabel from config", slog.Any("error", err))
			return "", "", ""
		}
		for _, gpuModel := range gpuLabels {
			if _, found := labels[gpuModel.TypeLabel]; found {
				return gpuModel.CapacityLabel, gpuModel.TypeLabel, gpuModel.MemLabel
			}
		}
	}
	return "", "", ""
}

// convert memory in bytes to GB
func getMem(memByte int64) float32 {
	memGB := float32(memByte) / (1024 * 1024 * 1024)
	return memGB
}

// convert millicores to cores, rounded to one decimal place
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

func checkNodeReadiness(node v1.Node) (bool, string) {
	isReady := false
	conditions := []string{}
	for _, cond := range node.Status.Conditions {
		if cond.Status == v1.ConditionTrue {
			if cond.Type == v1.NodeReady {
				isReady = true
			}
			conditions = append(conditions, string(cond.Type))
		}
	}
	return isReady, strings.Join(conditions, ",")
}
