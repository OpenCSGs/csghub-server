package types

type SpaceSdk struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CreateSpaceSdkReq struct {
	Name string `json:"name" binding:"required"`
}

type UpdateSpaceSdkReq struct {
	ID   int64  `json:"id"`
	Name string `json:"name" binding:"required"`
}
