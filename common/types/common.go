package types

type MultiSource struct {
	HFPath  string `json:"hf_path"`
	MSPath  string `json:"ms_path"`
	CSGPath string `json:"csg_path"`
}

type CaptchaResponse struct {
	ID   string `json:"id"`
	B64s string `json:"bs64"`
}
