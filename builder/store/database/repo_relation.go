package database

import (
	"context"
	"fmt"
)

type repoRelationsStoreImpl struct {
	db *DB
}

type RepoRelationsStore interface {
	// From gets the relationships from a repository
	From(ctx context.Context, repoID int64) ([]*RepoRelation, error)
	// To gets the relationships to a repository
	To(ctx context.Context, repoID int64) ([]*RepoRelation, error)
	// Override replaces all existing relationships from a repository to others
	//
	// `to` can be empty, in which case all existing relationships will be deleted
	Override(ctx context.Context, from int64, to ...int64) error
	// Delete removes a relationship from a repository to another
	Delete(ctx context.Context, from, to int64) error
}

func NewRepoRelationsStore() RepoRelationsStore {
	return &repoRelationsStoreImpl{
		db: defaultDB,
	}
}

type RepoRelation struct {
	ID         int64 `bun:",pk,autoincrement" json:"id"`
	FromRepoID int64 `bun:",notnull" json:"from_repo_id"`
	ToRepoID   int64 `bun:",notnull" json:"to_repo_id"`
}

// From gets the relationships from a repository
func (r *repoRelationsStoreImpl) From(ctx context.Context, repoID int64) ([]*RepoRelation, error) {
	var rrs []*RepoRelation
	err := r.db.Core.NewSelect().Model(&rrs).Where("from_repo_id = ?", repoID).Scan(ctx)
	return rrs, err
}

// To gets the relationships to a repository
func (r *repoRelationsStoreImpl) To(ctx context.Context, repoID int64) ([]*RepoRelation, error) {
	var rrs []*RepoRelation
	err := r.db.Core.NewSelect().Model(&rrs).Where("to_repo_id = ?", repoID).Scan(ctx)
	return rrs, err
}

// Override replaces all existing relationships from a repository to others
//
// `to` can be empty, in which case all existing relationships will be deleted
func (r *repoRelationsStoreImpl) Override(ctx context.Context, from int64, to ...int64) error {
	var relations []*RepoRelation
	for _, toRepoID := range to {
		relations = append(relations, &RepoRelation{
			FromRepoID: from,
			ToRepoID:   toRepoID,
		})
	}

	tx, err := r.db.Core.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	_, err = tx.NewDelete().Model((*RepoRelation)(nil)).Where("from_repo_id = ?", from).Exec(ctx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete existing relations: %w", err)
	}

	if len(relations) > 0 {
		_, err = tx.NewInsert().Model(&relations).
			Exec(ctx)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert relations: %w", err)
		}
	}

	return tx.Commit()
}

// Delete removes a relationship from a repository to another
func (r *repoRelationsStoreImpl) Delete(ctx context.Context, from, to int64) error {
	result, err := r.db.Core.NewDelete().
		Model((*RepoRelation)(nil)).
		Where("from_repo_id = ? and to_repo_id = ?", from, to).
		Exec(ctx)
	return assertAffectedOneRow(result, err)
}
