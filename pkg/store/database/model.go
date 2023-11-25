package database

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
)

type ModelStore struct {
	db *model.DB
}

func NewModelStore(db *model.DB) *ModelStore {
	return &ModelStore{
		db: db,
	}
}
