package types

type SpaceSdk struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type CreateSpaceSdkReq struct {
	Name    string `json:"name" binding:"required"`
	Version string `json:"version" binding:"required"`
}

type UpdateSpaceSdkReq struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}
