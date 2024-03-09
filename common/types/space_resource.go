package types

type SpaceResource struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Cpu    int    `json:"cpu"`
	Gpu    int    `json:"gpu"`
	Memory int    `json:"memory"`
	Disk   int    `json:"disk"`
}

type CreateSpaceResourceReq struct {
	Name   string `json:"name" binding:"required"`
	Cpu    int    `json:"cpu" binding:"required"`
	Gpu    int    `json:"gpu" binding:"required"`
	Memory int    `json:"memory" binding:"required"`
	Disk   int    `json:"disk" binding:"required"`
}

type UpdateSpaceResourceReq struct {
	ID     int64  `json:"-"`
	Name   string `json:"name"`
	Cpu    int    `json:"cpu"`
	Gpu    int    `json:"gpu"`
	Memory int    `json:"memory"`
	Disk   int    `json:"disk"`
}
