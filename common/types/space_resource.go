package types

type ResourceType string
type PayMode string

const (
	ResourceTypeCPU   ResourceType = "cpu"
	ResourceTypeGPU   ResourceType = "gpu"
	ResourceTypeNPU   ResourceType = "npu"
	ResourceTypeGCU   ResourceType = "gcu"
	ResourceTypeGPGPU ResourceType = "gpgpu"
	ResourceTypeMLU   ResourceType = "mlu"
	ResourceTypeDCU   ResourceType = "dcu"
	PayModeFree       PayMode      = "free"
	PayModeMinute     PayMode      = "minute"
	PayModeMonth      PayMode      = "month"
	PayModeYear       PayMode      = "year"
)

func ResourceTypeValid(resourceType ResourceType) bool {
	return resourceType == ResourceTypeCPU ||
		resourceType == ResourceTypeGPU ||
		resourceType == ResourceTypeNPU ||
		resourceType == ResourceTypeGCU ||
		resourceType == ResourceTypeGPGPU ||
		resourceType == ResourceTypeMLU ||
		resourceType == ResourceTypeDCU
}

type SpaceResource struct {
	ID            int64        `json:"id"`
	Name          string       `json:"name"`
	ClusterID     string       `json:"cluster_id"`
	Resources     string       `json:"resources"`
	Price         float64      `json:"price"`
	IsAvailable   bool         `json:"is_available"`
	Type          ResourceType `json:"type"`
	PayMode       PayMode      `json:"pay_mode"`
	IsReserved    bool         `json:"is_reserved"`
	OrderDetailId int64        `json:"order_detail_id"`
}

type CreateSpaceResourceReq struct {
	Name      string `json:"name" binding:"required"`
	Resources string `json:"resources" binding:"required"`
	ClusterID string `json:"cluster_id" binding:"required"`
}

type UpdateSpaceResourceReq struct {
	ID        int64  `json:"-"`
	Name      string `json:"name"`
	Resources string `json:"resources"`
}

type SpaceResourceIndexReq struct {
	ClusterID    string       `json:"cluster_id"`
	DeployType   int          `json:"deploy_type"`
	CurrentUser  string       `json:"current_user"`
	ResourceType ResourceType `json:"resource_type"`
	HardwareType string       `json:"hardware_type"`
	IsAvailable  bool         `json:"is_available"`
	PageOpts
}

type SpaceResourceFilter struct {
	ClusterID    string       `json:"cluster_id"`
	ResourceType ResourceType `json:"resource_type"`
	HardwareType string       `json:"hardware_type"`
}
