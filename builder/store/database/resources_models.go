package database

import (
	"context"
)

type ResourceModelStore struct {
	db *DB
}

func NewResourceModelStore() *ResourceModelStore {
	return &ResourceModelStore{db: defaultDB}
}

type ResourceModel struct {
	ID           int64  `bun:",pk,autoincrement" json:"id"`
	ResourceName string `bun:",notnull" json:"resource_name"`
	EngineName   string `bun:",notnull" json:"engine_name"`
	ModelName    string `bun:",notnull" json:"model_name"`
	Type         string `bun:",notnull" json:"type"`
	times
}

// find multi Resource model by model name with fuzzy matching, parameter modelName like model_name in db
func (s *ResourceModelStore) FindByModelName(ctx context.Context, modelName string) ([]*ResourceModel, error) {
	var models []*ResourceModel
	err := s.db.Core.NewSelect().Model(&models).Where("model_name LIKE ?", "%"+modelName+"%").Scan(ctx)
	return models, err
}

// find model by name which is in resource model table but not in runtime framework repo
func (s *ResourceModelStore) CheckModelNameNotInRFRepo(ctx context.Context, modelName string, repoId int64) (*ResourceModel, error) {
	var rm ResourceModel
	_, err := s.db.Core.NewSelect().Model(&rm).
		Where("LOWER(model_name) LIKE ?", "%"+modelName+"%").
		Exec(ctx, &rm)
	if err != nil {
		return nil, err
	}

	var rrfs []*RepositoriesRuntimeFramework
	err = s.db.Core.NewSelect().Model(&rrfs).Where("repo_id = ?", repoId).
		Scan(ctx)
	if err != nil || len(rrfs) > 0 {
		return nil, err
	}

	return &rm, nil
}
