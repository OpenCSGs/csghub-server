package database

import (
	"time"

	"opencsg.com/csghub-server/common/types"
)

type Space struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",notnull" json:"repository_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	// gradio, streamlit, docker etc
	Sdk        string `bun:",notnull" json:"sdk"`
	SdkVersion string `bun:",notnull" json:"sdk_version"`
	// PythonVersion string `bun:",notnull" json:"python_version"`
	Template      string `bun:",notnull" json:"template"`
	CoverImageUrl string `bun:"" json:"cover_image_url"`
	Env           string `bun:",notnull" json:"env"`
	Hardware      string `bun:",notnull" json:"hardware"`
	Secrets       string `bun:",notnull" json:"secrets"`
	HasAppFile    bool   `bun:"," json:"has_app_file"`
	SKU           string `bun:"," json:"sku"`
	times
}

/* tables for recommendations */

// RecomWeight are recommendation weight settings
type RecomWeight struct {
	Name string `bun:",pk" json:"name"`
	//the expression to calculate weight
	WeightExp string `bun:",notnull" json:"weight_exp" `
	times
}

// RecomOpWeight are the special weights of a repository manually set by operator
type RecomOpWeight struct {
	RepositoryID int64 `bun:",pk" json:"repository_id"`
	Weight       int   `bun:",notnull" json:"weight" `
	times
}

// RecomRepoScore is the recommendation score of a repository
type RecomRepoScore struct {
	RepositoryID int64 `bun:",pk" json:"repository_id"`
	//the total recommendation score calculated by all the recommendation weights
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

/* tables for on-premises repo synchronization */
type SyncVersion struct {
	Version        int64                `bun:",pk,autoincrement" json:"version"`
	SourceID       int64                `bun:",notnull" json:"source_id"`
	RepoPath       string               `bun:",notnull" json:"repo_path"`
	RepoType       types.RepositoryType `bun:",notnull" json:"repo_type"`
	LastModifiedAt time.Time            `bun:",notnull" json:"last_modified_at"`
	ChangeLog      string               `bun:"," json:"change_log"`
}
