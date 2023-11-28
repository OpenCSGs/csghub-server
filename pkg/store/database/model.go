package database

import (
	"context"

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

func (s *ModelStore) CreateRepo(ctx context.Context, repo Repository) (err error) {
	err = s.db.Operator.Core.NewInsert().Model(&repo).Scan(ctx)
	return
}
