package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
)

type ModelStore struct {
	db *DB
}

func NewModelStore() *ModelStore {
	return &ModelStore{
		db: defaultDB,
	}
}

type Model struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	Name          string      `bun:",notnull" json:"name"`
	UrlSlug       string      `bun:",notnull" json:"url_slug"`
	Description   string      `bun:",nullzero" json:"description"`
	Likes         int64       `bun:",notnull" json:"likes"`
	Downloads     int64       `bun:",notnull" json:"downloads"`
	Path          string      `bun:",notnull" json:"path"`
	GitPath       string      `bun:",notnull" json:"git_path"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last_updated_at"`
	Private       bool        `bun:",notnull" json:"private"`
	UserID        int64       `bun:",notnull" json:"user_id"`
	User          *User       `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

func (s *ModelStore) Index(ctx context.Context, per, page int) (models []Model, count int, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Order("created_at DESC").
		Limit(per).
		Offset((page - 1) * per).
		Scan(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) Public(ctx context.Context, search, sort, tag string, per, page int) (models []Model, count int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Where("model.private = ?", false)
	if search != "" {
		query = query.Where(
			"model.path like ? or model.description like ? or model.name like ?",
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
		)
	}
	if tag != "" {
		query = query.
			Join("JOIN repositories ON model.repository_id = repositories.id").
			Join("JOIN repository_tags ON repositories.id = repository_tags.repository_id").
			Join("JOIN tags ON repository_tags.tag_id = tags.id").
			Where("tags.name = ?", tag)
	}
	count, err = query.Count(ctx)
	if err != nil {
		return
	}

	query = query.Order(sortBy[sort])
	query = query.Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) PublicToUser(ctx context.Context, user *User, search, sort string, tags []TagReq, per, page int) (models []Model, count int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Relation("Repository.Tags")

	if user != nil {
		query = query.Where("model.private = ? or model.user_id = ?", false, user.ID)
	} else {
		query = query.Where("model.private = ?", false)
	}

	if search != "" {
		query = query.Where(
			"model.path like ? or model.description like ? or model.name like ?",
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
		)
	}
	// TODO：Optimize SQL
	if len(tags) > 0 {
		for _, tag := range tags {
			query = query.Where("model.repository_id IN (SELECT repository_id FROM repository_tags JOIN tags ON repository_tags.tag_id = tags.id WHERE tags.category = ? AND tags.name = ?)", tag.Category, tag.Name)
		}
	}

	count, err = query.Count(ctx)
	if err != nil {
		return
	}

	query = query.Order(sortBy[sort])
	query = query.Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (models []Model, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Join("JOIN users AS u ON u.id = model.user_id").
		Where("u.username = ?", username)

	if onlyPublic {
		query = query.Where("private = ?", false)
	}
	query = query.Order("created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx, &models)

	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) Count(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", ModelRepo).
		Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) PublicCount(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", DatasetRepo).
		Where("private = ?", false).
		Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) Create(ctx context.Context, model *Model, repo *Repository, userId int64) (newModel *Model, err error) {
	resModel := new(Model)
	model.UserID = userId
	repo.UserID = userId
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(tx.NewInsert().Model(repo).Exec(ctx)); err != nil {
			return err
		}
		model.RepositoryID = repo.ID
		if err = assertAffectedOneRow(tx.NewInsert().Model(model).Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	err = s.db.Operator.Core.NewSelect().
		Model(resModel).
		Where("model.id=?", model.ID).
		Relation("Repository").
		Scan(ctx)
	err = s.db.Operator.Core.NewSelect().
		Model(resModel.Repository).
		WherePK().
		Relation("Tags").
		Scan(ctx)

	return resModel, nil
}

func (s *ModelStore) Update(ctx context.Context, model *Model, repo *Repository) (err error) {
	repo.UpdatedAt = time.Now()
	model.UpdatedAt = time.Now()
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(tx.NewUpdate().Model(model).WherePK().Exec(ctx)); err != nil {
			return err
		}
		if err = assertAffectedOneRow(tx.NewUpdate().Model(repo).WherePK().Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	return
}

func (s *ModelStore) FindyByPath(ctx context.Context, namespace string, repoPath string) (*Model, error) {
	resModel := new(Model)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resModel).
		Relation("Repository").
		Relation("User").
		Where("model.path =?", fmt.Sprintf("%s/%s", namespace, repoPath)).
		Where("model.name =?", repoPath).
		Limit(1).
		Scan(ctx)
	err = s.db.Operator.Core.NewSelect().
		Model(resModel.Repository).
		WherePK().
		Relation("Tags").
		Scan(ctx)
	return resModel, err
}

func (s *ModelStore) Delete(ctx context.Context, namespace, name string) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(
			tx.NewDelete().
				Model(&Repository{}).
				Where("path = ?", fmt.Sprintf("%v/%v", namespace, name)).
				Where("repository_type = ?", ModelRepo).
				Exec(ctx)); err != nil {
			return err
		}
		if err = assertAffectedOneRow(
			tx.NewDelete().
				Model(&Model{}).
				Where("path = ?", fmt.Sprintf("%v/%v", namespace, name)).
				Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	return
}

func (s *ModelStore) Tags(ctx context.Context, namespace, name string) (tags []Tag, err error) {
	query := s.db.Operator.Core.NewSelect().
		ColumnExpr("tags.*").
		Model(&Model{}).
		Join("JOIN repositories ON model.repository_id = repositories.id").
		Join("JOIN repository_tags ON repositories.id = repository_tags.repository_id").
		Join("JOIN tags ON repository_tags.tag_id = tags.id").
		Where("repositories.repository_type = ?", ModelRepo).
		Where("model.path = ?", fmt.Sprintf("%v/%v", namespace, name))

	slog.Info(query.String())
	err = query.Scan(ctx, &tags)
	return
}