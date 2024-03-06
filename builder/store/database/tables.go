package database

type Space struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",notnull" json:"repository_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	SdkID        int64       `bun:",notnull" json:"sdk_id"`
	// gradio, streamlit, docker etc
	Sdk           *SpaceSdk      `bun:"rel:belongs-to,join:sdk_id=id" json:"sdk"`
	ResourceID    int64          `bun:",notnull" json:"resource_id"`
	Resource      *SpaceResource `bun:"rel:belongs-to,join:resource_id=id" json:"resource"`
	CoverImageUrl string         `bun:"" json:"cover_image_url"`

	times
}
