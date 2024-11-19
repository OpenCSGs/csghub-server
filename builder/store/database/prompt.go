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

type promptStoreImpl struct {
	db *DB
}

type PromptStore interface {
	Create(ctx context.Context, input Prompt) (*Prompt, error)
	ByRepoIDs(ctx context.Context, repoIDs []int64) (prompts []Prompt, err error)
	ByRepoID(ctx context.Context, repoID int64) (*Prompt, error)
	Update(ctx context.Context, input Prompt) (err error)
	FindByPath(ctx context.Context, namespace string, repoPath string) (*Prompt, error)
	Delete(ctx context.Context, input Prompt) error
	ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (prompts []Prompt, total int, err error)
	ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (prompts []Prompt, total int, err error)
}

func NewPromptStore() PromptStore {
	return &promptStoreImpl{db: defaultDB}
}

func NewPromptStoreWithDB(db *DB) PromptStore {
	return &promptStoreImpl{db: db}
}

func (s *promptStoreImpl) Create(ctx context.Context, input Prompt) (*Prompt, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create prompt in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *promptStoreImpl) ByRepoIDs(ctx context.Context, repoIDs []int64) (prompts []Prompt, err error) {
	q := s.db.Operator.Core.NewSelect().
		Model(&prompts).
		Relation("Repository").
		Relation("Repository.User").
		Where("repository_id in (?)", bun.In(repoIDs))
	err = q.Scan(ctx)
	return
}

func (s *promptStoreImpl) ByRepoID(ctx context.Context, repoID int64) (*Prompt, error) {
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

func (s *promptStoreImpl) Update(ctx context.Context, input Prompt) (err error) {
	_, err = s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return
}

func (s *promptStoreImpl) FindByPath(ctx context.Context, namespace string, repoPath string) (*Prompt, error) {
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

func (s *promptStoreImpl) Delete(ctx context.Context, input Prompt) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete prompt failed,error:%w", err)
	}
	return nil
}

func (s *promptStoreImpl) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (prompts []Prompt, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&prompts).
		Relation("Repository.User").
		Where("username = ?", username)

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("prompt.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *promptStoreImpl) ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (prompts []Prompt, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&prompts).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", namespace))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("prompt.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx, &prompts)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}
