package database

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

	HasAppFile bool `bun:"," json:"has_app_file"`

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
