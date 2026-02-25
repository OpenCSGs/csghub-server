package types

import "time"

type ClusterRequest struct {
	ClusterID     string `json:"cluster_id"`
	ClusterConfig string `json:"cluster_config"`
	Region        string `json:"region"`
	Zone          string `json:"zone"`     //cn-beijing
	Provider      string `json:"provider"` //ali
	Enable        bool   `json:"enable"`
	StorageClass  string `json:"storage_class"`
	Status        string `json:"status"`
	Endpoint      string `json:"endpoint"`
}
type ClusterEvent struct {
	ClusterID        string        `json:"cluster_id"`
	ClusterConfig    string        `json:"cluster_config"`
	Region           string        `json:"region"`
	Zone             string        `json:"zone"`     //cn-beijing
	Provider         string        `json:"provider"` //ali
	Enable           bool          `json:"enable"`
	StorageClass     string        `json:"storage_class"`
	Status           ClusterStatus `json:"status"`
	Endpoint         string        `json:"endpoint"`          // address of remote runner
	NetworkInterface string        `json:"network_interface"` //used for multi-host, e.g., eth0
	Mode             ClusterMode   `json:"mode"`              // InCluster | kubeconfig
	AppEndpoint      string        `json:"app_endpoint"`      // address of space/inference application
}
type HardwareInfo struct {
	Region    string `json:"region"`
	GPUVendor string `json:"gpu_vendor"`
	XPUModel  string `json:"xpu_model"`
	XPUMem    int64  `json:"xpu_mem"`
}

type PublicClusterRes struct {
	Regions    []string       `json:"region"`
	GPUVendors []string       `json:"gpu_vendor"`
	Hardware   []HardwareInfo `json:"hardware"`
}

type ClusterRes struct {
	ClusterID        string             `json:"cluster_id"`
	Status           ClusterStatus      `json:"status" i18n:"cluster.status"` //active, inactive
	TotalCPU         float64            `json:"total_cpu"`
	AvailableCPU     float64            `json:"available_cpu"`
	CPUUsage         float64            `json:"cpu_usage"`
	TotalGPU         int64              `json:"total_gpu"`
	AvailableGPU     int64              `json:"available_gpu"`
	GPUUsage         float64            `json:"gpu_usage"`
	TotalMem         float64            `json:"total_mem"`     //in GB
	AvailableMem     float64            `json:"available_mem"` //in GB
	MemUsage         float64            `json:"mem_usage"`
	TotalVXPU        int64              `json:"total_vxpu"`         // total vxpu number
	UsedVXPUNum      int64              `json:"used_vxpu_num"`      // use vxpu num
	TotalVXPUMem     int64              `json:"total_vxpu_mem"`     //in MB
	AvailableVXPUMem int64              `json:"available_vxpu_mem"` //in MB
	VXPUUsage        float64            `json:"vxpu_usage"`         // vxpu num usage
	VXPUMemUsage     float64            `json:"vxpu_mem_usage"`     // vxpu mem usage
	NodeNumber       int                `json:"node_number"`
	Region           string             `json:"region"`
	Zone             string             `json:"zone"`     //cn-beijing
	Provider         string             `json:"provider"` //ali
	Resources        []NodeResourceInfo `json:"resources,omitempty"`
	StorageClass     string             `json:"storage_class"`
	Endpoint         string             `json:"endpoint"`

	ResourceStatus ResourceStatus `json:"resource_status"`

	LastUpdateTime int64  `json:"last_update_time"`
	XPUVendors     string `json:"xpu_vendors"` // NVIDIA, AMD
	XPUModels      string `json:"xpu_models"`  // A10(32 GB),H100(80 GB)

	Enable bool `json:"enable"`
}
type DeployRes struct {
	ClusterID       string    `json:"cluster_id"`
	ClusterRegion   string    `json:"cluster_region"`
	DeployName      string    `json:"deploy_name"`
	User            User      `json:"user"`
	Resource        string    `json:"resource"`
	CreateTime      time.Time `json:"create_time"`
	Status          string    `json:"status"`
	TotalTimeInMin  int       `json:"total_time_in_min"`
	TotalFeeInCents int       `json:"total_fee_in_cents"`
	SvcName         string    `json:"svc_name"` // service name of the deployment, used for inference endpoint
}

type VXPU struct {
	ID           string `json:"id"`
	Index        int    `json:"index"`
	Mem          int64  `json:"mem"` // total mem in MB
	Parallel     int64  `json:"parallel"`
	Core         int64  `json:"core"`
	Type         string `json:"type"`          // e.g., NVIDIA
	AllocatedMem int64  `json:"allocated_mem"` // allocated mem in MB
}

type ProcessInfo struct {
	PodName      string `json:"pod_name"`
	DeployID     string `json:"deploy_id"`
	SvcName      string `json:"svc_name"`
	VXPUs        []VXPU `json:"vxpus"`
	WorkflowName string `json:"workflow_name"`
	ClusterNode  string `json:"cluster_node"`
}

type MIGResource struct {
	Capacity    int64 `json:"capacity"`
	Allocatable int64 `json:"allocatable"`
	Requests    int64 `json:"requests"`
	Limits      int64 `json:"limits"`
}

type NodeHardware struct {
	TotalCPU         float64 `json:"total_cpu"`
	AvailableCPU     float64 `json:"available_cpu"`
	TotalMem         float32 `json:"total_mem"`     //in GB
	AvailableMem     float32 `json:"available_mem"` //in GB
	XPUModel         string  `json:"xpu_model"`
	TotalXPU         int64   `json:"total_xpu"`
	AvailableXPU     int64   `json:"available_xpu"`
	GPUVendor        string  `json:"gpu_vendor"`
	XPUCapacityLabel string  `json:"xpu_capacity_label"`
	ReservedXPU      int64   `json:"reserved_xpu"`
	XPUMem           string  `json:"xpu_mem"`

	VXPUs            []VXPU `json:"vxpus"`              // virtual XPU, e.g., vGPU, vTPU, etc.
	TotalVXPU        int64  `json:"total_vxpu"`         // total vxpu num
	UsedVXPUNum      int64  `json:"used_vxpu_num"`      // used vxpu num
	TotalVXPUMem     int64  `json:"total_vxpu_mem"`     // total mem in MB
	AvailableVXPUMem int64  `json:"available_vxpu_mem"` // available mem in MB

	MIGs map[string]*MIGResource `json:"migs"` // mig resources
}

type NodeResourceInfo struct {
	NodeName   string `json:"node_name"`
	NodeStatus string `json:"node_status"`
	NodeHardware
	Processes  []ProcessInfo     `json:"processes"` // pods running on the node
	Labels     map[string]string `json:"labels"`    // labels of the node
	EnableVXPU bool              `json:"enable_vxpu"`
}

type UpdateClusterResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type GPUModel struct {
	TypeLabel     string `json:"type_label"`
	CapacityLabel string `json:"capacity_label"`
	MemLabel      string `json:"mem_label"`
}

type ClusterStatus string

const (
	ClusterStatusRunning     ClusterStatus = "Running"
	ClusterStatusUnavailable ClusterStatus = "Unavailable"
)

type ClusterMode string

const (
	DefaultClusterCongfig             = "config"
	ConnectModeInCluster  ClusterMode = "incluster"  // InCluster RBAC Connect Mode
	ConnectModeKubeConfig ClusterMode = "kubeconfig" // KubeConfig Connect Mode
)

// ResourceStatus indicates how the resource availability result was calculated.
type ResourceStatus string

const (
	// StatusClusterWide signifies that the result was calculated by scanning the entire cluster (requires ClusterRole).
	StatusClusterWide ResourceStatus = "ClusterWide"
	// StatusNamespaceQuota signifies that the result was determined from a namespace-specific ResourceQuota.
	StatusNamespaceQuota ResourceStatus = "NamespaceQuota"
	// StatusUncertain signifies that resource availability could not be determined
	//
	// (e.g., insufficient permissions and no fallback Quota).
	StatusUncertain ResourceStatus = "Uncertain"
)

// ResourceAvailable
type ResourceAvailable struct {
	Status          ResourceStatus   `json:"status"`
	AvailableCPU    float64          `json:"available_cpu"`
	AvailableMemory float32          `json:"available_memory"`
	AvailableXPU    map[string]int64 `json:"available_xpu"`
}

type EndpointReq struct {
	ClusterID string
	Target    string
	Host      string
	Endpoint  string
	SvcName   string
}

type HAMIGPU struct {
	ID      string `json:"id"`
	Index   *int   `json:"index"`
	Count   int    `json:"count"`
	DevMem  int    `json:"devmem"`
	DevCore int    `json:"devcore"`
	Type    string `json:"type"`
	Numa    *int   `json:"numa"`
	Mode    string `json:"mode"`
	Health  bool   `json:"health"`
}
