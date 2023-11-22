package database

import "git-devops.opencsg.com/product/community/starhub-server/pkg/model"

type DatasetStore struct {
	db *model.DB
}

func NewDatasetStore(db *model.DB) DatasetStore {
	return DatasetStore{
		db: db,
	}
}
