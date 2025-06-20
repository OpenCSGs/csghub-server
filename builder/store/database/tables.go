package database

import (
	"time"

	"opencsg.com/csghub-server/common/types"
)

type RecomWeightName string // like freshess, downloads, quality, op, etc

const (
	RecomWeightFreshness RecomWeightName = "freshness"
	RecomWeightDownloads RecomWeightName = "downloads"
	RecomWeightQuality   RecomWeightName = "quality"
	RecomWeightOp        RecomWeightName = "op"
	// sum of all other weight scores
	RecomWeightTotal RecomWeightName = "total"
)

type Space struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",notnull" json:"repository_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	// gradio, streamlit, docker etc
	Sdk           string `bun:",notnull" json:"sdk"`
	SdkVersion    string `bun:",notnull" json:"sdk_version"`
	DriverVersion string `bun:",notnull" json:"driver_version"`
	// PythonVersion string `bun:",notnull" json:"python_version"`
	Template      string `bun:",notnull" json:"template"`
	CoverImageUrl string `bun:"" json:"cover_image_url"`
	Env           string `bun:",notnull" json:"env"`
	Hardware      string `bun:",notnull" json:"hardware"`
	Secrets       string `bun:",notnull" json:"secrets"`
	HasAppFile    bool   `bun:"," json:"has_app_file"`
	SKU           string `bun:"," json:"sku"`
	OrderDetailID int64  `bun:"," json:"order_detail_id"`
	Variables     string `bun:",nullzero" json:"variables"`
	ClusterID     string `bun:",nullzero" json:"cluster_id"`
	times
}

/* tables for recommendations */

// RecomWeight are recommendation weight settings
type RecomWeight struct {
	Name RecomWeightName `bun:",pk" json:"name"`
	//the expression to calculate weight
	WeightExp string `bun:",notnull" json:"weight_exp" `
	times
}

// Deprecated: use RecomRepoScore's 'op' score instead
// RecomOpWeight are the special weights of a repository manually set by operator
type RecomOpWeight struct {
	RepositoryID int64 `bun:",pk" json:"repository_id"`
	Weight       int   `bun:",notnull" json:"weight" `
	times
}

// RecomRepoScore is the recommendation score of a repository
type RecomRepoScore struct {
	ID           int64 `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64 `bun:",notnull" json:"repository_id"`
	// like freshess, downloads, quality, op, etc
	WeightName RecomWeightName `bun:",notnull" json:"weight_name"`
	//the recommendation score calculated for corresponding weights
	Score float64 `bun:",notnull" json:"score"`
	times
}

/* tables for client events */
type Event struct {
	ID        int64     `bun:",pk,autoincrement" json:"id"`
	Module    string    `bun:",notnull" json:"module"`
	EventID   string    `bun:",notnull" json:"event_id"`
	Value     string    `bun:",notnull" json:"value"`
	ClientID  string    `bun:"," json:"client_id"`
	ClientIP  string    `bun:"," json:"client_ip"`
	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	Extension string    `bun:"," json:"extension"`
}

/* tables for broadcast messages */
type Broadcast struct {
	ID      int64  `bun:",pk,autoincrement" json:"id"`
	Content string `bun:"type:text,notnull" json:"content"`
	BcType  string `bun:",notnull" json:"bc_type"`
	Theme   string `bun:",notnull" json:"theme"`
	Status  string `bun:",notnull" json:"status"`
}

/* tables for on-premises repo synchronization */
type SyncVersion struct {
	Version        int64                `bun:",pk,autoincrement" json:"version"`
	SourceID       int64                `bun:",notnull" json:"source_id"`
	RepoPath       string               `bun:",notnull" json:"repo_path"`
	RepoType       types.RepositoryType `bun:",notnull" json:"repo_type"`
	LastModifiedAt time.Time            `bun:",notnull" json:"last_modified_at"`
	ChangeLog      string               `bun:"," json:"change_log"`
	// true if CE,EE complete the sync process successfully, e.g the repo created.
	Completed bool `bun:",notnull" json:"completed"`
}
