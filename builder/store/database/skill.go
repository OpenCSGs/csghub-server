package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type skillStoreImpl struct {
	db *DB
}

type SkillStore interface {
	ByRepoIDs(ctx context.Context, repoIDs []int64) (skills []Skill, err error)
	ByRepoID(ctx context.Context, repoID int64) (*Skill, error)
	ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (skills []Skill, total int, err error)
	UserLikesSkills(ctx context.Context, userID int64, per, page int) (skills []Skill, total int, err error)
	ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (skills []Skill, total int, err error)
	Create(ctx context.Context, input Skill) (*Skill, error)
	Update(ctx context.Context, input Skill) (err error)
	FindByPath(ctx context.Context, namespace string, repoPath string) (skill *Skill, err error)
	Delete(ctx context.Context, input Skill) error
	ListByPath(ctx context.Context, paths []string) ([]Skill, error)
	CreateIfNotExist(ctx context.Context, input Skill) (*Skill, error)
	CreateAndUpdateRepoPath(ctx context.Context, input Skill, path string) (*Skill, error)
}

func NewSkillStore() SkillStore {
	return &skillStoreImpl{db: defaultDB}
}

func NewSkillStoreWithDB(db *DB) SkillStore {
	return &skillStoreImpl{db: db}
}

type Skill struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last_updated_at"`
	times
}

func (s *skillStoreImpl) ByRepoIDs(ctx context.Context, repoIDs []int64) (skills []Skill, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&skills).
		Relation("Repository").
		Relation("Repository.Mirror").
		Relation("Repository.Mirror.CurrentTask").
		Where("skill.repository_id in (?)", bun.In(repoIDs)).
		Scan(ctx)

	return
}

func (s *skillStoreImpl) ByRepoID(ctx context.Context, repoID int64) (*Skill, error) {
	var skill Skill
	err := s.db.Operator.Core.NewSelect().
		Model(&skill).
		Where("repository_id = ?", repoID).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to select skill, error: %w", err)
	}

	return &skill, nil
}

func (s *skillStoreImpl) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (skills []Skill, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&skills).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", username))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("skill.created_at DESC").
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

func (s *skillStoreImpl) UserLikesSkills(ctx context.Context, userID int64, per, page int) (skills []Skill, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&skills).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.id in (select repo_id from user_likes where user_id=? and deleted_at is NULL)", userID)

	query = query.Order("skill.created_at DESC").
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

func (s *skillStoreImpl) ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (skills []Skill, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&skills).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", namespace))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("skill.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx, &skills)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *skillStoreImpl) Create(ctx context.Context, input Skill) (*Skill, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create skill in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create skill in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *skillStoreImpl) Update(ctx context.Context, input Skill) (err error) {
	_, err = s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return
}

func (s *skillStoreImpl) FindByPath(ctx context.Context, namespace string, repoPath string) (skill *Skill, err error) {
	resSkill := new(Skill)
	err = s.db.Operator.Core.
		NewSelect().
		Model(resSkill).
		Relation("Repository.User").
		Relation("Repository.Mirror").
		Relation("Repository.Mirror.CurrentTask").
		Where("repository.path =?", fmt.Sprintf("%s/%s", namespace, repoPath)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find skill: %w", err)
	}
	err = s.db.Operator.Core.NewSelect().
		Model(resSkill.Repository).
		WherePK().
		Relation("Tags", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("repository_tag.count > 0")
		}).
		Scan(ctx)
	return resSkill, err
}

func (s *skillStoreImpl) Delete(ctx context.Context, input Skill) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete skill in tx failed,error:%w", err)
	}
	return nil
}

func (s *skillStoreImpl) ListByPath(ctx context.Context, paths []string) ([]Skill, error) {
	var skills []Skill
	err := s.db.Operator.Core.
		NewSelect().
		Model(&Skill{}).
		Relation("Repository").
		Where("path IN (?)", bun.In(paths)).
		Scan(ctx, &skills)
	if err != nil {
		return nil, fmt.Errorf("failed to find skills by path,error: %w", err)
	}
	return skills, nil
}

func (s *skillStoreImpl) CreateIfNotExist(ctx context.Context, input Skill) (*Skill, error) {
	err := s.db.Core.NewSelect().
		Model(&input).
		Where("repository_id = ?", input.RepositoryID).
		Relation("Repository").
		Scan(ctx)
	if err == nil {
		return &input, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return &input, err
	}

	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create skill in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create skill in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *skillStoreImpl) CreateAndUpdateRepoPath(ctx context.Context, input Skill, path string) (*Skill, error) {
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var repo Repository
		_, err := tx.NewInsert().Model(&input).Exec(ctx, &input)
		if err != nil {
			return fmt.Errorf("failed to create skill: %w", err)
		}
		repo, err = updateRepoPath(ctx, tx, types.SkillRepo, path, input.RepositoryID)
		if err != nil {
			return fmt.Errorf("failed to update repository path: %w", err)
		}
		input.Repository = &repo
		return nil
	})
	return &input, err
}
