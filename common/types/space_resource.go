package types

type SpaceResource struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CreateSpaceResourceReq struct {
	Name string `json:"name" binding:"required"`
}

type UpdateSpaceResourceReq struct {
	ID   int64  `json:"-"`
	Name string `json:"name" binding:"required"`
}
