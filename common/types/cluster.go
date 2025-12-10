package types

import "time"

type ClusterResponse struct {
	ClusterID string                      `json:"cluster_id"`
	Region    string                      `json:"region"`
	Zone      string                      `json:"zone"`     //cn-beijing
	Provider  string                      `json:"provider"` //ali
	Enable    bool                        `json:"enable"`
	Nodes     map[string]NodeResourceInfo `json:"nodes"`

	ClusterName    string         `json:"cluster_name"`
	StorageClass   string         `json:"storage_class"`
	ResourceStatus ResourceStatus `json:"resource_status"`

	UpdatedAt time.Time `json:"updated_at"`
}

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
type ClusterRes struct {
	ClusterID    string             `json:"cluster_id"`
	Status       ClusterStatus      `json:"status" i18n:"cluster.status"` //active, inactive
	TotalCPU     float64            `json:"total_cpu"`
	AvailableCPU float64            `json:"available_cpu"`
	CPUUsage     float64            `json:"cpu_usage"`
	TotalGPU     int64              `json:"total_gpu"`
	AvailableGPU int64              `json:"available_gpu"`
	GPUUsage     float64            `json:"gpu_usage"`
	TotalMem     float64            `json:"total_mem"`     //in GB
	AvailableMem float64            `json:"available_mem"` //in GB
	MemUsage     float64            `json:"mem_usage"`     //in GB
	NodeNumber   int                `json:"node_number"`
	Region       string             `json:"region"`
	Zone         string             `json:"zone"`     //cn-beijing
	Provider     string             `json:"provider"` //ali
	Resources    []NodeResourceInfo `json:"resources,omitempty"`
	StorageClass string             `json:"storage_class"`
	Endpoint     string             `json:"endpoint"`

	ResourceStatus ResourceStatus `json:"resource_status"`

	LastUpdateTime int64  `json:"last_update_time"`
	XPUVendors     string `json:"xpu_vendors"` // NVIDIA, AMD
	XPUModels      string `json:"xpu_models"`  // A10(32 GB),H100(80 GB)
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
}

type NodeResourceInfo struct {
	NodeName         string  `json:"node_name"`
	XPUModel         string  `json:"xpu_model"`
	TotalCPU         float64 `json:"total_cpu"`
	AvailableCPU     float64 `json:"available_cpu"`
	TotalXPU         int64   `json:"total_xpu"`
	AvailableXPU     int64   `json:"available_xpu"`
	GPUVendor        string  `json:"gpu_vendor"`
	TotalMem         float32 `json:"total_mem"`     //in GB
	AvailableMem     float32 `json:"available_mem"` //in GB
	XPUCapacityLabel string  `json:"xpu_capacity_label"`
	ReservedXPU      int64   `json:"reserved_xpu"`
	XPUMem           string  `json:"xpu_mem"`
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

type HearBeatEvent struct {
	Running     []string `json:"running"`
	Unavailable []string `json:"unavailable"`
}

type EndpointReq struct {
	ClusterID string
	Target    string
	Host      string
	Endpoint  string
	SvcName   string
}
