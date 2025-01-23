package types

import (
	"time"
)

var OssFileExpire = 259200 * time.Second

type DatasetTagCommit struct {
	ID string `json:"id"`
}

type CreateDatasetReq struct {
	CreateRepoReq
	Type int `json:"type"`
}

type UpdateDatasetReq struct {
	UpdateRepoReq
}

type Dataset struct {
	ID                   int64                `json:"id,omitempty"`
	Name                 string               `json:"name"`
	Nickname             string               `json:"nickname"`
	Description          string               `json:"description"`
	Likes                int64                `json:"likes"`
	Downloads            int64                `json:"downloads"`
	Path                 string               `json:"path"`
	RepositoryID         int64                `json:"repository_id"`
	Repository           Repository           `json:"repository"`
	Private              bool                 `json:"private"`
	User                 User                 `json:"user"`
	Tags                 []RepoTag            `json:"tags"`
	Readme               string               `json:"readme"`
	DefaultBranch        string               `json:"default_branch"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
	UserLikes            bool                 `json:"user_likes"`
	Source               RepositorySource     `json:"source"`
	SyncStatus           RepositorySyncStatus `json:"sync_status"`
	License              string               `json:"license"`
	CanWrite             bool                 `json:"can_write"`
	CanManage            bool                 `json:"can_manage"`
	Namespace            *Namespace           `json:"namespace"`
	SensitiveCheckStatus string               `json:"sensitive_check_status"`
	MirrorLastUpdatedAt  time.Time            `json:"mirror_last_updated_at"`
}

type DataViewerReq struct {
	Config  string `json:"config"`
	Split   string `json:"split"`
	Search  string `json:"search"`
	Where   string `json:"where"`
	Orderby string `json:"orderby"`
}

type QueryReq struct {
	PageSize  int    `json:"page_size"`
	PageIndex int    `json:"page_index"`
	Search    string `json:"search"`
	Where     string `json:"where"`
	Orderby   string `json:"orderby"`
}

var GitattributesFileName = ".gitattributes"

const DatasetGitattributesContent = `*.7z filter=lfs diff=lfs merge=lfs -text
*.arrow filter=lfs diff=lfs merge=lfs -text
*.bin filter=lfs diff=lfs merge=lfs -text
*.bz2 filter=lfs diff=lfs merge=lfs -text
*.ckpt filter=lfs diff=lfs merge=lfs -text
*.ftz filter=lfs diff=lfs merge=lfs -text
*.gz filter=lfs diff=lfs merge=lfs -text
*.h5 filter=lfs diff=lfs merge=lfs -text
*.joblib filter=lfs diff=lfs merge=lfs -text
*.lfs.* filter=lfs diff=lfs merge=lfs -text
*.lz4 filter=lfs diff=lfs merge=lfs -text
*.mlmodel filter=lfs diff=lfs merge=lfs -text
*.model filter=lfs diff=lfs merge=lfs -text
*.msgpack filter=lfs diff=lfs merge=lfs -text
*.npy filter=lfs diff=lfs merge=lfs -text
*.npz filter=lfs diff=lfs merge=lfs -text
*.onnx filter=lfs diff=lfs merge=lfs -text
*.ot filter=lfs diff=lfs merge=lfs -text
*.parquet filter=lfs diff=lfs merge=lfs -text
*.pb filter=lfs diff=lfs merge=lfs -text
*.pickle filter=lfs diff=lfs merge=lfs -text
*.pkl filter=lfs diff=lfs merge=lfs -text
*.pt filter=lfs diff=lfs merge=lfs -text
*.pth filter=lfs diff=lfs merge=lfs -text
*.rar filter=lfs diff=lfs merge=lfs -text
*.safetensors filter=lfs diff=lfs merge=lfs -text
saved_model/**/* filter=lfs diff=lfs merge=lfs -text
*.tar.* filter=lfs diff=lfs merge=lfs -text
*.tar filter=lfs diff=lfs merge=lfs -text
*.tflite filter=lfs diff=lfs merge=lfs -text
*.tgz filter=lfs diff=lfs merge=lfs -text
*.wasm filter=lfs diff=lfs merge=lfs -text
*.xz filter=lfs diff=lfs merge=lfs -text
*.zip filter=lfs diff=lfs merge=lfs -text
*.zst filter=lfs diff=lfs merge=lfs -text
*tfevents* filter=lfs diff=lfs merge=lfs -text
# Audio files - uncompressed
*.pcm filter=lfs diff=lfs merge=lfs -text
*.sam filter=lfs diff=lfs merge=lfs -text
*.raw filter=lfs diff=lfs merge=lfs -text
# Audio files - compressed
*.aac filter=lfs diff=lfs merge=lfs -text
*.flac filter=lfs diff=lfs merge=lfs -text
*.mp3 filter=lfs diff=lfs merge=lfs -text
*.ogg filter=lfs diff=lfs merge=lfs -text
*.wav filter=lfs diff=lfs merge=lfs -text
# Image files - uncompressed
*.bmp filter=lfs diff=lfs merge=lfs -text
*.gif filter=lfs diff=lfs merge=lfs -text
*.png filter=lfs diff=lfs merge=lfs -text
*.tiff filter=lfs diff=lfs merge=lfs -text
# Image files - compressed
*.jpg filter=lfs diff=lfs merge=lfs -text
*.jpeg filter=lfs diff=lfs merge=lfs -text
*.webp filter=lfs diff=lfs merge=lfs -text

`
