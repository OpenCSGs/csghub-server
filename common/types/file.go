package types

type File struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Size   int64  `json:"size"`
	Commit Commit `json:"commit"`
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	SHA    string `json:"sha"`
	// URL to browse the file
	URL            string `json:"url"`
	Content        string `json:"content"`
	Lfs            bool   `json:"lfs"`
	LfsPointerSize int    `json:"lfs_pointer_size"`
	// relative path in lfs storage
	LfsRelativePath string `json:"lfs_relative_path"`
	LastCommitSHA   string `json:"last_commit_sha"`
}

type CreateFileReq struct {
	//will use login username, ignore username from http request body
	Username  string `json:"-"`
	Email     string `json:"-"`
	Message   string `json:"message" form:"message"`
	Branch    string `json:"branch" form:"branch"`
	Content   string `json:"content"`
	NewBranch string `json:"new_branch"`

	NameSpace string         `json:"-"`
	Name      string         `json:"-"`
	FilePath  string         `json:"-"`
	RepoType  RepositoryType `json:"-"`
}

type CreateFileResp struct{}

type UpdateFileReq struct {
	//will use login username, ignore username from http request body
	Username   string `json:"-"`
	Email      string `json:"-"`
	Message    string `json:"message"`
	Branch     string `json:"branch"`
	Content    string `json:"content"`
	NewBranch  string `json:"new_branch"`
	OriginPath string `json:"origin_path"`
	SHA        string `json:"sha"`

	NameSpace string `json:"-"`
	Name      string `json:"-"`
	// new file path, it will be different from OriginPath if file renamed
	FilePath string `json:"-"`
	RepoType RepositoryType
}

type GetCommitsReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Per       int    `json:"per"`
	Page      int    `json:"page"`
	Ref       string `json:"ref"`
	RepoType  RepositoryType
}

type GetFileReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Ref       string `json:"ref"`
	Lfs       bool   `json:"lfs"`
	SaveAs    string `json:"save_as"`
	RepoType  RepositoryType
}

type GetBranchesReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Page      int    `json:"page"`
	Per       int    `json:"per"`
	RepoType  RepositoryType
}

type GetTagsReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	RepoType  RepositoryType
}

// currently update and create fiel share the same response
type UpdateFileResp CreateFileResp

type SDKFiles struct {
	SHA       string    `json:"sha"`
	Tags      []string  `json:"tags"`
	Likes     int64     `json:"likes"`
	Downloads int64     `json:"downloads"`
	Private   bool      `json:"private"`
	ID        string    `json:"id"`
	Siblings  []SDKFile `json:"siblings"`
}

type SDKFile struct {
	Filename string `json:"rfilename"`
}

type CreateFileParams struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	Branch    string `json:"branch"`
	Content   string `json:"content"`
	NewBranch string `json:"new_branch"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	FilePath  string `json:"file_path"`
}

type GetAllFilesReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
	CurrentUser string         `json:"current_user"`
}
