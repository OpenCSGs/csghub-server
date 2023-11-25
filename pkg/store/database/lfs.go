package database

type LfsFile struct {
	ID           int        `bun:",pk,autoincrement" json:"id"`
	RepositoryID int        `bun:",notnull" json:"repository_id"`
	Repository   Repository `bun:"rel:belongs-to,join:repository_id=id"`
	OriginPath   string     `bun:",notnull" json:"orgin_path"`
	times
}
