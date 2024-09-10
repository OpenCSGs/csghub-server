package types

type ClusterResponse struct {
	ClusterID string                      `json:"cluster_id"`
	Region    string                      `json:"region"`
	Zone      string                      `json:"zone"`     //cn-beijing
	Provider  string                      `json:"provider"` //ali
	Enable    bool                        `json:"enable"`
	Nodes     map[string]NodeResourceInfo `json:"nodes"`
}
type ClusterRequest struct {
	ClusterID     string `json:"cluster_id"`
	ClusterConfig string `json:"cluster_config"`
	Region        string `json:"region"`
	Zone          string `json:"zone"`     //cn-beijing
	Provider      string `json:"provider"` //ali
	Enable        bool   `json:"enable"`
	StorageClass  string `json:"storage_class"`
}
type ClusterRes struct {
	ClusterID    string             `json:"cluster_id"`
	Region       string             `json:"region"`
	Zone         string             `json:"zone"`     //cn-beijing
	Provider     string             `json:"provider"` //ali
	Resources    []NodeResourceInfo `json:"resources"`
	StorageClass string             `json:"storage_class"`
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
}

type UpdateClusterResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
