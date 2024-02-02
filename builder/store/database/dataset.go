package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

var sortBy = map[string]string{
	"trending":        "downloads DESC",
	"recently_update": "updated_at DESC",
	"most_download":   "downloads DESC",
	"most_favorite":   "likes DESC",
}

type DatasetStore struct {
	db *DB
}

func NewDatasetStore() *DatasetStore {
	return &DatasetStore{db: defaultDB}
}

type Dataset struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	Name          string      `bun:",notnull" json:"name"`
	UrlSlug       string      `bun:",notnull" json:"nickname"`
	Description   string      `bun:",nullzero" json:"description"`
	Likes         int64       `bun:",notnull" json:"likes"`
	Downloads     int64       `bun:",notnull" json:"downloads"`
	Path          string      `bun:",notnull" json:"path"`
	GitPath       string      `bun:",notnull" json:"git_path"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last"`
	Private       bool        `bun:",notnull" json:"private"`
	UserID        int64       `bun:",notnull" json:"user_id"`
	User          *User       `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

func (s *DatasetStore) Index(ctx context.Context, per, page int) (datasets []*Repository, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Where("repository_type = ?", types.DatasetRepo).
		Order("created_at DESC").
		Limit(per).
		Offset((page - 1) * per).
		Scan(ctx)
	if err != nil {
		return
	}
	return
}

func (s *DatasetStore) PublicToUser(ctx context.Context, user *User, search, sort string, tags []TagReq, per, page int) (datasets []Dataset, count int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Relation("Repository.Tags")

	if user != nil {
		query = query.Where("dataset.private = ? or dataset.user_id = ?", false, user.ID)
	} else {
		query = query.Where("dataset.private = ?", false)
	}

	if search != "" {
		search = strings.ToLower(search)
		query = query.Where(
			"LOWER(dataset.path) like ? or LOWER(dataset.description) like ? or LOWER(dataset.name) like ?",
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
		)
	}
	// TODO：Optimize SQL
	if len(tags) > 0 {
		for _, tag := range tags {
			query = query.Where("dataset.repository_id IN (SELECT repository_id FROM repository_tags JOIN tags ON repository_tags.tag_id = tags.id WHERE tags.category = ? AND tags.name = ?)", tag.Category, tag.Name)
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

func (s *DatasetStore) Public(ctx context.Context, search, sort, tag string, per, page int) (datasets []Dataset, count int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Where("dataset.private = ?", false)
	if search != "" {
		search = strings.ToLower(search)
		query = query.Where(
			"LOWER(dataset.path) like ? or LOWER(dataset.description) like ? or LOWER(dataset.name) like ?",
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
		)
	}
	if tag != "" {
		query = query.
			Join("JOIN repositories ON dataset.repository_id = repositories.id").
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

func (s *DatasetStore) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (datasets []Dataset, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Relation("Repository.Tags").
		Where("dataset.path like ?", fmt.Sprintf("%s/%%", username))

	if onlyPublic {
		query = query.Where("dataset.private = ?", false)
	}
	query = query.Order("dataset.created_at DESC").
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

func (s *DatasetStore) ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (datasets []Dataset, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Relation("Repository.Tags").
		Where("dataset.path like ?", fmt.Sprintf("%s/%%", namespace))

	if onlyPublic {
		query = query.Where("dataset.private = ?", false)
	}
	query = query.Order("dataset.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx, &datasets)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *DatasetStore) Count(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", types.DatasetRepo).
		Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *DatasetStore) PublicCount(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", types.DatasetRepo).
		Where("private = ?", false).
		Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *DatasetStore) Create(ctx context.Context, dataset *Dataset, repo *Repository, userId int64) (newDataset *Dataset, err error) {
	resDataset := new(Dataset)
	repo.UserID = userId
	dataset.UserID = userId
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(tx.NewInsert().Model(repo).Exec(ctx)); err != nil {
			return err
		}
		dataset.RepositoryID = repo.ID
		if err = assertAffectedOneRow(tx.NewInsert().Model(dataset).Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	err = s.db.Operator.Core.NewSelect().
		Model(resDataset).
		Where("dataset.id=?", dataset.ID).
		Relation("Repository").
		Scan(ctx)
	err = s.db.Operator.Core.NewSelect().
		Model(resDataset.Repository).
		WherePK().
		Relation("Tags").
		Scan(ctx)

	return resDataset, nil
}

func (s *DatasetStore) Update(ctx context.Context, dataset *Dataset, repo *Repository) (err error) {
	repo.UpdatedAt = time.Now()
	dataset.UpdatedAt = time.Now()
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(tx.NewUpdate().Model(dataset).WherePK().Exec(ctx)); err != nil {
			return err
		}
		if err = assertAffectedOneRow(tx.NewUpdate().Model(repo).WherePK().Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	return
}

func (s *DatasetStore) UpdateRepoFileDownloads(ctx context.Context, dataset *Dataset, date time.Time, clickDownloadCount int64) (err error) {
	rd := new(RepositoryDownload)
	err = s.db.Operator.Core.NewSelect().
		Model(rd).
		Where("date = ? AND repository_id = ?", date.Format("2006-01-02"), dataset.RepositoryID).
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return
	}

	if errors.Is(err, sql.ErrNoRows) {
		rd.ClickDownloadCount = clickDownloadCount
		rd.Date = date
		rd.RepositoryID = dataset.RepositoryID
		err = s.db.Operator.Core.NewInsert().
			Model(rd).
			Scan(ctx)
		if err != nil {
			return
		}
	} else {
		rd.ClickDownloadCount = rd.ClickDownloadCount + clickDownloadCount
		rd.UpdatedAt = time.Now()
		query := s.db.Operator.Core.NewUpdate().
			Model(rd).
			WherePK()
		slog.Debug(query.String())

		_, err = query.Exec(ctx)
		if err != nil {
			return
		}
	}
	err = s.UpdateDownloads(ctx, dataset)
	if err != nil {
		return
	}

	return
}

func (s *DatasetStore) UpdateRepoCloneDownloads(ctx context.Context, dataset *Dataset, date time.Time, cloneCount int64) (err error) {
	rd := new(RepositoryDownload)
	err = s.db.Operator.Core.NewSelect().
		Model(rd).
		Where("date = ? AND repository_id = ?", date.Format("2006-01-02"), dataset.RepositoryID).
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return
	}

	if errors.Is(err, sql.ErrNoRows) {
		rd.CloneCount = cloneCount
		rd.Date = date
		rd.RepositoryID = dataset.RepositoryID
		err = s.db.Operator.Core.NewInsert().
			Model(rd).
			Scan(ctx)
		if err != nil {
			return
		}
	} else {
		rd.CloneCount = cloneCount
		rd.UpdatedAt = time.Now()
		query := s.db.Operator.Core.NewUpdate().
			Model(rd).
			WherePK()
		slog.Debug(query.String())

		_, err = query.Exec(ctx)
		if err != nil {
			return
		}
	}
	err = s.UpdateDownloads(ctx, dataset)
	if err != nil {
		return
	}

	return
}

func (s *DatasetStore) UpdateDownloads(ctx context.Context, dataset *Dataset) error {
	var downloadCount int64
	err := s.db.Operator.Core.NewSelect().
		ColumnExpr("(SUM(clone_count)+SUM(click_download_count)) AS total_count").
		Model(&RepositoryDownload{}).
		Where("repository_id=?", dataset.RepositoryID).
		Scan(ctx, &downloadCount)
	if err != nil {
		return err
	}
	dataset.Downloads = downloadCount
	_, err = s.db.Operator.Core.NewUpdate().
		Model(dataset).
		WherePK().
		Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *DatasetStore) FindByPath(ctx context.Context, namespace string, repoPath string) (dataset *Dataset, err error) {
	resDataset := new(Dataset)
	err = s.db.Operator.Core.
		NewSelect().
		Model(resDataset).
		Relation("Repository").
		Relation("User").
		Where("dataset.path =?", fmt.Sprintf("%s/%s", namespace, repoPath)).
		Where("dataset.name =?", repoPath).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset: %w", err)
	}
	err = s.db.Operator.Core.NewSelect().
		Model(resDataset.Repository).
		WherePK().
		Relation("Tags").
		Scan(ctx)
	return resDataset, err
}

func (s *DatasetStore) Delete(ctx context.Context, namespace, name string) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(
			tx.NewDelete().
				Model(&Repository{}).
				Where("path = ?", fmt.Sprintf("%v/%v", namespace, name)).
				Where("repository_type = ?", types.DatasetRepo).
				Exec(ctx)); err != nil {
			return err
		}
		if err = assertAffectedOneRow(
			tx.NewDelete().
				Model(&Dataset{}).
				Where("path = ?", fmt.Sprintf("%v/%v", namespace, name)).
				Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	return
}

func (s *DatasetStore) Tags(ctx context.Context, namespace, name string) (tags []Tag, err error) {
	query := s.db.Operator.Core.NewSelect().
		ColumnExpr("tags.*").
		Model(&Dataset{}).
		Join("JOIN repositories ON dataset.repository_id = repositories.id").
		Join("JOIN repository_tags ON repositories.id = repository_tags.repository_id").
		Join("JOIN tags ON repository_tags.tag_id = tags.id").
		Where("repositories.repository_type = ?", types.DatasetRepo).
		Where("dataset.path = ?", fmt.Sprintf("%v/%v", namespace, name))

	slog.Debug(query.String())
	err = query.Scan(ctx, &tags)
	return
}
