package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type Prompt struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",notnull" json:"repository_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	times
}

type PromptStore struct {
	db *DB
}

func NewPromptStore() *PromptStore {
	return &PromptStore{db: defaultDB}
}

func (s *PromptStore) Create(ctx context.Context, input Prompt) (*Prompt, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create prompt in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *PromptStore) ByRepoIDs(ctx context.Context, repoIDs []int64) (prompts []Prompt, err error) {
	q := s.db.Operator.Core.NewSelect().
		Model(&prompts).
		Relation("Repository").
		Relation("Repository.User").
		Where("repository_id in (?)", bun.In(repoIDs))
	err = q.Scan(ctx)
	return
}

func (s *PromptStore) ByRepoID(ctx context.Context, repoID int64) (*Prompt, error) {
	var prompt Prompt
	err := s.db.Operator.Core.NewSelect().
		Model(&prompt).
		Where("repository_id = ?", repoID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select prompt by repository id: %d, error: %w", repoID, err)
	}

	return &prompt, nil
}

func (s *PromptStore) Update(ctx context.Context, input Prompt) (err error) {
	_, err = s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return
}

func (s *PromptStore) FindByPath(ctx context.Context, namespace string, repoPath string) (*Prompt, error) {
	resPrompt := new(Prompt)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resPrompt).
		Relation("Repository.User").
		Where("repository.path =?", fmt.Sprintf("%s/%s", namespace, repoPath)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find prompt: %w", err)
	}
	err = s.db.Operator.Core.NewSelect().
		Model(resPrompt.Repository).
		WherePK().
		Relation("Tags", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("repository_tag.count > 0")
		}).
		Scan(ctx)
	return resPrompt, err
}

func (s *PromptStore) Delete(ctx context.Context, input Prompt) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete prompt failed,error:%w", err)
	}
	return nil
}