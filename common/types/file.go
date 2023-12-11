package types

type File struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Lfs         bool   `json:"lfs"`
	Size        int    `json:"size"`
	Commit      Commit `json:"commit"`
	Path        string `json:"path"`
	Mode        string `json:"mode"`
	SHA         string `json:"sha"`
	DownloadURL string `json:"download_url"`
	Content     string `json:"content"`
}

type CreateFileReq struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	Branch    string `json:"branch"`
	Content   string `json:"content"`
	NewBranch string `json:"new_branch"`
}

type UpdateFileReq struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Message    string `json:"message"`
	Branch     string `json:"branch"`
	Content    string `json:"content"`
	NewBranch  string `json:"new_branch"`
	OriginPath string `json:"origin_path"`
	SHA        string `json:"sha"`
}
