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
	LfsSHA256      string `json:"lfs_sha256"`
	LfsPointerSize int    `json:"lfs_pointer_size"`
	// relative path in lfs storage
	LfsRelativePath string `json:"lfs_relative_path"`
	LastCommitSHA   string `json:"last_commit_sha"`
	// whether file is previewable
	PreviewCode FilePreviewCode `json:"preview_code,omitempty"`
	XnetEnabled bool            `json:"xnet_enabled"`
}

type CreateFileReq struct {
	//will use login username, ignore username from http request body
	Username  string `json:"-"`
	Email     string `json:"-"`
	Message   string `json:"message" form:"message"`
	Branch    string `json:"branch" form:"branch"`
	Content   string `json:"content"`
	NewBranch string `json:"new_branch"`
	// Use for lfs file
	OriginalContent []byte   `json:"original_content"`
	Pointer         *Pointer `json:"pointer"`

	Namespace   string         `json:"-"`
	Name        string         `json:"-"`
	FilePath    string         `json:"-"`
	RepoType    RepositoryType `json:"-"`
	CurrentUser string         `json:"current_user"`

	StartRepoType  RepositoryType `json:"start_repo_type"`
	StartNamespace string         `json:"start_namespace"`
	StartName      string         `json:"start_name"`
	StartBranch    string         `json:"start_branch"`
	StartSha       string         `json:"start_sha"`
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

	// Use for lfs file
	OriginalContent []byte   `json:"original_content"`
	Pointer         *Pointer `json:"pointer"`

	Namespace string `json:"-"`
	Name      string `json:"-"`
	// new file path, it will be different from OriginPath if file renamed
	FilePath    string `json:"-"`
	RepoType    RepositoryType
	CurrentUser string `json:"-"`

	StartRepoType  RepositoryType `json:"start_repo_type"`
	StartNamespace string         `json:"start_namespace"`
	StartName      string         `json:"start_name"`
	StartBranch    string         `json:"start_branch"`
	StartSha       string         `json:"start_sha"`
}

type DeleteFileReq struct {
	//will use login username, ignore username from http request body
	Username   string `json:"-"`
	Email      string `json:"-"`
	Message    string `json:"message"`
	Branch     string `json:"branch"`
	Content    string `json:"content"`
	NewBranch  string `json:"new_branch"`
	OriginPath string `json:"origin_path"`

	// Use for lfs file
	OriginalContent []byte   `json:"original_content"`
	Pointer         *Pointer `json:"pointer"`

	Namespace string `json:"-"`
	Name      string `json:"-"`
	// new file path, it will be different from OriginPath if file renamed
	FilePath    string `json:"-"`
	RepoType    RepositoryType
	CurrentUser string `json:"-"`
}

type GetCommitsReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Per         int    `json:"per"`
	Page        int    `json:"page"`
	Ref         string `json:"ref"`
	RepoType    RepositoryType
	CurrentUser string `json:"current_user"`
}

type GetFileReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Ref         string `json:"ref"`
	Lfs         bool   `json:"lfs"`
	SaveAs      string `json:"save_as"`
	RepoType    RepositoryType
	CurrentUser string `json:"current_user"`
	// limit file size, don't return file content if file size is greater than MaxFileSize
	MaxFileSize int64 `json:"max_file_size"`
}

type GetTreeRequest struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	Ref         string         `json:"ref"`
	Path        string         `json:"path"`
	RepoType    RepositoryType `json:"repo_type"`
	Limit       int            `json:"limit"`
	Cursor      string         `json:"cursor"`
	CurrentUser string         `json:"current_user"`
	Recursive   bool           `json:"-"`
}

type GetLogsTreeRequest struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	Ref         string         `json:"ref"`
	Path        string         `json:"path"`
	RepoType    RepositoryType `json:"repo_type"`
	Limit       int            `json:"limit"`
	Offset      int            `json:"offset"`
	CurrentUser string         `json:"current_user"`
}

type GetBranchesReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Page        int    `json:"page"`
	Per         int    `json:"per"`
	RepoType    RepositoryType
	CurrentUser string `json:"current_user"`
}

type GetTagsReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	RepoType    RepositoryType
	CurrentUser string `json:"current_user"`
}

// currently update and create fiel share the same response
type UpdateFileResp CreateFileResp
type DeleteFileResp CreateFileResp

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
	Filename string  `json:"rfilename"`
	BlobID   string  `json:"blobId,omitempty"`
	Size     int64   `json:"size,omitempty"`
	LFS      *SDKLFS `json:"lfs,omitempty"`
}

type SDKLFS struct {
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	PointerSize int    `json:"pointerSize"`
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
	Ref         string         `json:"ref"`
	Limit       int            `json:"limit"`
	Cursor      string         `json:"cursor"`
	Path        string         `json:"path"`
}

type LFSPointer struct {
	Oid      string `json:"oid"`
	Size     int64  `json:"size"`
	FileOid  string `json:"file_oid"`
	Data     string `json:"data"`
	FileSize int64  `json:"file_size"`
}

type GetRepoFileTreeResp struct {
	Files  []*File
	Cursor string
}

type FilePreviewCode int

const (
	// allow to preview, by default
	FilePreviewCodeNormal FilePreviewCode = iota
	// dont allow to preview because file size is too large
	FilePreviewCodeTooLarge
	// dont allow to preview because file content is not text
	FilePreviewCodeNotText
)

type GetDiffBetweenCommitsReq struct {
	Namespace     string         `json:"namespace"`
	Name          string         `json:"name"`
	RepoType      RepositoryType `json:"repo_type"`
	LeftCommitID  string         `json:"left_commit_id"`
	RightCommitID string         `json:"right_commit_id"`
	CurrentUser   string         `json:"current_user"`
}

type PreuploadReq struct {
	Namespace   string          `json:"-"`
	Name        string          `json:"-"`
	RepoType    RepositoryType  `json:"-"`
	Revision    string          `json:"-"`
	CurrentUser string          `json:"-"`
	Files       []PreuploadFile `json:"files"`
}

type PreuploadFile struct {
	Path   string `json:"path"`
	Sample string `json:"sample"`
	Size   int64  `json:"size"`
}

type PreuploadResp struct {
	Files []PreuploadRespFile `json:"files"`
}

type PreuploadRespFile struct {
	OID          string     `json:"oid"`
	Path         string     `json:"path"`
	UploadMode   UploadMode `json:"uploadMode"`
	ShouldIgnore bool       `json:"shouldIgnore"`
	IsDir        bool       `json:"isDir"`
}

type UploadMode string

const (
	UploadModeRegular UploadMode = "regular"
	UploadModeLFS     UploadMode = "lfs"
)

type CommitFilesReq struct {
	Namespace   string          `json:"-"`
	Name        string          `json:"-"`
	RepoType    RepositoryType  `json:"-"`
	Revision    string          `json:"-"`
	CurrentUser string          `json:"-"`
	Message     string          `json:"message"`
	Files       []CommitFileReq `json:"files"`
}

type CommitFileReq struct {
	Path    string       `json:"path"`
	Action  CommitAction `json:"action"`
	Content string       `json:"content"`
}

type CommitAction string

const (
	CommitActionCreate CommitAction = "create"
	CommitActionUpdate CommitAction = "update"
	CommitActionDelete CommitAction = "delete"
)

type CommitFilesResp struct {
	Files []string `json:"files"`
}

type CommitHeader struct {
	Summary     string `json:"summary"`
	Description string `json:"description"`
}

// LFSFile represents an LFS file in the commit
type CommitLFSFile struct {
	Path string `json:"path"`
	Algo string `json:"algo"`
	OID  string `json:"oid"`
	Size int64  `json:"size"`
}

// File represents a regular file in the commit
type CommitFile struct {
	Content  string `json:"content"`
	Path     string `json:"path"`
	Encoding string `json:"encoding"`
}

// FormField represents a form field with key-value structure
type FormField struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// CommitRequest represents the complete commit request
type CommitRequest struct {
	Header   *CommitHeader   `json:"header,omitempty"`
	LFSFiles []CommitLFSFile `json:"lfsFiles,omitempty"`
	Files    []CommitFile    `json:"files,omitempty"`
}
