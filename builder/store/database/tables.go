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

	times
}
