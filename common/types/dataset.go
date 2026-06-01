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
	// The type of the dataset
	Type int `json:"type"`
}

// DatasetType represents the type of a dataset
type DatasetType string

const (
	DatasetTypeNormal     DatasetType = "normal"
	DatasetTypeCommercial DatasetType = "commercial"
)

type UpdateDatasetReq struct {
	UpdateRepoReq
	DatasetType      DatasetType `json:"dataset_type"`
	RelatedDatasetID int64       `json:"related_dataset_id"`
	Price            float64     `json:"price"`
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
	Scores               []WeightScore        `json:"scores"`
	SensitiveCheckStatus string               `json:"sensitive_check_status"`
	MirrorLastUpdatedAt  time.Time            `json:"mirror_last_updated_at"`
	URL                  string               `json:"url"`
	MultiSource
	RecomOpWeight         int                       `json:"recom_op_weight,omitempty"`
	MirrorTaskStatus      MirrorTaskStatus          `json:"mirror_task_status"`
	XnetEnabled           bool                      `json:"xnet_enabled"`
	XnetMigrationStatus   XnetMigrationTaskStatus   `json:"xnet_migration_status"`
	XnetMigrationProgress int                       `json:"xnet_migration_progress"`
	DatasetType           DatasetType               `json:"dataset_type"`
	RelatedDatasetID      int64                     `json:"related_dataset_id"`
	RelatedDataset        *Dataset                  `json:"related_dataset,omitempty"`
	Price                 float64                   `json:"price"`
	Forked                bool                      `json:"forked"`
	IsForSale             bool                      `json:"is_for_sale"`
	UserPurchased         bool                      `json:"user_purchased"`
	PurchaseTaskStatus    DatasetPurchaseTaskStatus `json:"purchase_task_status"`
	Status                DatasetStatus             `json:"status"`
	CurrentApplication    *DatasetApplication       `json:"current_application,omitempty"`
}

// DatasetStatus represents the status of a dataset
type DatasetStatus string

const (
	DatasetStatusNormal   DatasetStatus = "normal"
	DatasetStatusListed   DatasetStatus = "listed"
	DatasetStatusDelisted DatasetStatus = "delisted"
)

// DatasetApplicationAction represents the action of a dataset application
type DatasetApplicationAction string

const (
	DatasetApplicationActionInitial DatasetApplicationAction = "initial"
	DatasetApplicationActionEdit    DatasetApplicationAction = "edit"
	DatasetApplicationActionRelist  DatasetApplicationAction = "relist"
	DatasetApplicationActionDelist  DatasetApplicationAction = "delist"
)

// DatasetApplicationStatus represents the status of a dataset application
type DatasetApplicationStatus string

const (
	DatasetApplicationStatusPending  DatasetApplicationStatus = "pending"
	DatasetApplicationStatusApproved DatasetApplicationStatus = "approved"
	DatasetApplicationStatusRejected DatasetApplicationStatus = "rejected"
)

type DatasetApplication struct {
	ID               int64                    `json:"id"`
	DatasetID        int64                    `json:"dataset_id"`
	Dataset          *Dataset                 `json:"dataset,omitempty"`
	ApplicantID      int64                    `json:"applicant_id"`
	Applicant        *User                    `json:"applicant,omitempty"`
	Action           DatasetApplicationAction `json:"action"`
	Price            float64                  `json:"price"`
	RelatedDatasetID int64                    `json:"related_dataset_id"`
	RelatedDataset   *Dataset                 `json:"related_dataset,omitempty"`
	Status           DatasetApplicationStatus `json:"status"`
	ReviewerID       int64                    `json:"reviewer_id"`
	Reviewer         *User                    `json:"reviewer,omitempty"`
	ReviewMsg        string                   `json:"review_msg"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
}

type CreateDatasetApplicationReq struct {
	Namespace        string  `json:"-"`
	Name             string  `json:"-"`
	Action           string  `json:"action" binding:"required"` // "list" or "delist", resolved to specific type by backend
	Price            float64 `json:"price" binding:"required"`
	RelatedDatasetID int64   `json:"related_dataset_id" binding:"required"`
	CurrentUser      string  `json:"-"`
}

type ReviewDatasetApplicationReq struct {
	ID          int64  `json:"id" binding:"required"`
	Action      string `json:"action" binding:"required"` // "approve" or "reject"
	ReviewMsg   string `json:"review_msg"`
	CurrentUser string `json:"-"`
}

type ListDatasetApplicationsReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	CurrentUser string `json:"current_user"`
	Status      string `json:"status"`
	Per         int    `json:"per"`
	Page        int    `json:"page"`
	Search      string `json:"search"`
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

type BuyDatasetReq struct {
	Namespace   string `json:"namespace" binding:"required"`
	Name        string `json:"name" binding:"required"`
	CurrentUser string `json:"current_user" binding:"required"`
	TargetName  string `json:"target_name" binding:"omitempty"`
}

// DatasetPurchaseTaskStatus represents the status of a dataset purchase task
type DatasetPurchaseTaskStatus string

const (
	// DatasetPurchaseTaskStatusPending means the task is waiting for execution
	DatasetPurchaseTaskStatusPending DatasetPurchaseTaskStatus = "pending"
	// DatasetPurchaseTaskStatusInProgress means the task is currently executing
	DatasetPurchaseTaskStatusInProgress DatasetPurchaseTaskStatus = "in_progress"
	// DatasetPurchaseTaskStatusCompleted means the task has completed successfully
	DatasetPurchaseTaskStatusCompleted DatasetPurchaseTaskStatus = "completed"
	// DatasetPurchaseTaskStatusFailed means the task has failed
	DatasetPurchaseTaskStatusFailed DatasetPurchaseTaskStatus = "failed"
)

type BuyDatasetResp struct {
	Success          bool    `json:"success"`
	Message          string  `json:"message"`
	Price            float64 `json:"price"`
	RelatedDatasetID int64   `json:"related_dataset_id"`
	TaskID           int64   `json:"task_id"`
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
