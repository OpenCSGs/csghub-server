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

	NameSpace string `json:"-"`
	Name      string `json:"-"`
	FilePath  string `json:"-"`
}

type CreateFileResp struct {
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

	NameSpace string `json:"-"`
	Name      string `json:"-"`
	//new file path, it will be different from OriginPath if file renamed
	FilePath string `json:"-"`
}

type GetCommitsReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Per       int    `json:"per"`
	Page      int    `json:"page"`
	Ref       string `json:"ref"`
}

type GetFileReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Ref       string `json:"ref"`
}

type GetBranchesReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Page      int    `json:"page"`
	Per       int    `json:"per"`
}

type GetTagsReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// currently update and create fiel share the same response
type UpdateFileResp CreateFileResp
