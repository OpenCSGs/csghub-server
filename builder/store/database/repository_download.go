package database

import "time"

type RepositoryDownload struct {
	ID                 int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID       int64       `bun:",notnull" json:"repository_id"`
	Date               time.Time   `bun:",notnull,type:date" json:"date"`
	CloneCount         int64       `bun:",notnull" json:"user_id"`
	ClickDownloadCount int64       `bun:",notnull" json:"click_download_count"`
	Repository         *Repository `bun:"rel:belongs-to,join:repository_id=id"`
	times
}
