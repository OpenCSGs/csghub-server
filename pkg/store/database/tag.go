package database

import "opencsg.com/starhub-server/pkg/model"

type TagStore struct {
	db *model.DB
}

func NewTagStore(db *model.DB) *TagStore {
	return &TagStore{
		db: db,
	}
}

type Tag struct {
	ID       int64  `bun:",pk,autoincrement" json:"id"`
	ParentID int64  `bun:",pk" json:"parent_id"`
	Name     string `bun:",notnull" json:"name"`
	times
}
