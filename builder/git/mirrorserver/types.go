package mirrorserver

type CreateMirrorRepoReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	CloneUrl    string `json:"clone_url"`
	Username    string `json:"username"`
	AccessToken string `json:"access_token"`
	Private     bool   `json:"private"`
	Description string `json:"description"`
	Interval    int    `json:"interval"`
}
