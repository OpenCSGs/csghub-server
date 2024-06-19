package types

type CreateMirrorTokenReq struct {
	Token           string `json:"token" binding:"required"`
	ConcurrentCount int    `json:"concurrent_count"`
	MaxBandwidth    int    `json:"max_bandwidth"`
}
