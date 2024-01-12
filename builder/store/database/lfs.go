package database

type LfsFile struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",notnull" json:"repository_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id"`
	OriginPath   string      `bun:",notnull" json:"orgin_path"`
	times
}
