//go:build saas || ee

package database

import (
	"context"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type modelTreeImpl struct {
	db *database.DB
}

type ModelTreeStore interface {
	GetParent(ctx context.Context, targetRepoID int64) ([]ModelTree, error)
	GetSourceRelationCount(ctx context.Context, sourceRepoID int64, relation types.ModelRelation) (int, error)
	CheckRelationExist(ctx context.Context, repoID int64) (bool, error)
	Add(ctx context.Context, relation types.ModelTreeReq) error
	Delete(ctx context.Context, targetRepoID int64) error
}

func NewModelTreeStore() ModelTreeStore {
	return &modelTreeImpl{
		db: database.GetDB(),
	}
}

func NewModelTreeWithDB(db *database.DB) ModelTreeStore {
	return &modelTreeImpl{
		db: db,
	}
}

type times struct {
	CreatedAt time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

type ModelTree struct {
	ID           int64               `bun:",pk,autoincrement" json:"id"`
	SourceRepoID int64               `bun:",notnull" json:"source_repo_id"`
	SourcePath   string              `bun:",notnull" json:"source_path"`
	TargetRepoID int64               `bun:",notnull" json:"target_repo_id"`
	TargetPath   string              `bun:",notnull" json:"target_path"`
	Relation     types.ModelRelation `bun:",notnull" json:"relation"`
	times
}

func (s *modelTreeImpl) GetParent(ctx context.Context, targetRepoID int64) ([]ModelTree, error) {
	query := `
    WITH RECURSIVE ParentPath AS (
		SELECT 
			id,
			source_repo_id,
			source_path,
			target_repo_id,
			target_path,
			relation
		FROM model_trees
		WHERE target_repo_id = ?
		UNION ALL
		SELECT 
			mt.id,
			mt.source_repo_id,
			mt.source_path,
			mt.target_repo_id,
			mt.target_path,
			mt.relation
		FROM model_trees mt
		INNER JOIN ParentPath pp ON mt.target_repo_id = pp.source_repo_id
	)
	SELECT * FROM ParentPath;`

	rows, err := s.db.BunDB.NewSelect().DB().Query(query, targetRepoID)
	if err != nil {
		err = errorx.HandleDBError(err, errorx.Ctx().Set("repo_id", targetRepoID))
		return nil, err
	}
	defer rows.Close()

	var nodes []ModelTree
	for rows.Next() {
		var row ModelTree
		if err := rows.Scan(&row.ID, &row.SourceRepoID, &row.SourcePath, &row.TargetRepoID, &row.TargetPath, &row.Relation); err != nil {
			err = errorx.HandleDBError(err, errorx.Ctx().Set("repo_id", targetRepoID))
			return nil, err
		}
		nodes = append(nodes, row)
	}

	if err := rows.Err(); err != nil {
		err = errorx.HandleDBError(err, errorx.Ctx().Set("repo_id", targetRepoID))
		return nil, err
	}

	return nodes, nil
}

func (s *modelTreeImpl) GetSourceRelationCount(ctx context.Context, sourceRepoID int64, relation types.ModelRelation) (int, error) {
	count, err := s.db.Operator.Core.NewSelect().
		Model((*ModelTree)(nil)).
		Where("source_repo_id = ? AND relation = ?", sourceRepoID, relation).
		Count(ctx)
	if err != nil {
		err = errorx.HandleDBError(err, errorx.Ctx().Set("source_repo_id", sourceRepoID))
		return 0, err
	}
	return count, nil
}

// check source_repo_id or target_repo_id is exist
func (s *modelTreeImpl) CheckRelationExist(ctx context.Context, repoID int64) (bool, error) {
	exists, err := s.db.Operator.Core.NewSelect().
		Model((*ModelTree)(nil)).
		Where("source_repo_id = ? OR target_repo_id = ?", repoID, repoID).
		Limit(1).
		Exists(ctx)
	if err != nil {
		err = errorx.HandleDBError(err, errorx.Ctx().Set("source_repo_id or target_repo_id", repoID))
		return false, err
	}
	return exists, nil
}

// add model tree
func (s *modelTreeImpl) Add(ctx context.Context, relation types.ModelTreeReq) error {
	modelTree := &ModelTree{
		SourceRepoID: relation.SourceRepoID,
		SourcePath:   relation.SourcePath,
		TargetRepoID: relation.TargetRepoID,
		TargetPath:   relation.TargetPath,
		Relation:     relation.Relation,
	}
	_, err := s.db.Operator.Core.NewInsert().
		Model(modelTree).
		On("CONFLICT (source_repo_id,target_repo_id) DO UPDATE").
		Exec(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().Set("source_repo_id", relation.SourceRepoID).Set("target_repo_id", relation.TargetRepoID))
	return err
}

// delete model tree by target_repo_id
func (s *modelTreeImpl) Delete(ctx context.Context, targetRepoID int64) error {
	_, err := s.db.Operator.Core.NewDelete().
		Model((*ModelTree)(nil)).
		Where("target_repo_id = ?", targetRepoID).
		Exec(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().Set("target_repo_id", targetRepoID))
	return err
}
