package types

type SpaceResource struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Resources   string  `json:"resources"`
	CostPerHour float64 `json:"cost_per_hour"`
	IsAvailable bool    `json:"is_available"`
}

type CreateSpaceResourceReq struct {
	Name        string  `json:"name" binding:"required"`
	Resources   string  `json:"resources" binding:"required"`
	CostPerHour float64 `json:"cost_per_hour" binding:"required"`
	ClusterID   string  `json:"cluster_id" binding:"required"`
}

type UpdateSpaceResourceReq struct {
	ID          int64   `json:"-"`
	Name        string  `json:"name"`
	Resources   string  `json:"resources"`
	CostPerHour float64 `json:"cost_per_hour"`
}
