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
}
type ClusterRes struct {
	ClusterID string             `json:"cluster_id"`
	Region    string             `json:"region"`
	Zone      string             `json:"zone"`     //cn-beijing
	Provider  string             `json:"provider"` //ali
	Resources []NodeResourceInfo `json:"resources"`
}

type NodeResourceInfo struct {
	NodeName     string  `json:"node_name"`
	GPUModel     string  `json:"gpu_model"`
	TotalCPU     float64 `json:"total_cpu"`
	AvailableCPU float64 `json:"available_cpu"`
	TotalGPU     int64   `json:"total_gpu"`
	AvailableGPU int64   `json:"available_gpu"`
	GPUVendor    string  `json:"gpu_vendor"`
	TotalMem     float32 `json:"total_mem"`     //in GB
	AvailableMem float32 `json:"available_mem"` //in GB
}

type UpdateClusterResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
