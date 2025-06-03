package types

type ResourceType string

const (
	ResourceTypeCPU   ResourceType = "cpu"
	ResourceTypeGPU   ResourceType = "gpu"
	ResourceTypeNPU   ResourceType = "npu"
	ResourceTypeGCU   ResourceType = "gcu"
	ResourceTypeGPGPU ResourceType = "gpgpu"
	ResourceTypeMLU   ResourceType = "mlu"
	ResourceTypeDCU   ResourceType = "dcu"
)

type SpaceResource struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	Resources   string       `json:"resources"`
	IsAvailable bool         `json:"is_available"`
	Type        ResourceType `json:"type"`
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
