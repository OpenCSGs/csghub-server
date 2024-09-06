package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type TagStore struct {
	db *DB
}

func NewTagStore() *TagStore {
	return &TagStore{
		db: defaultDB,
	}
}

type TagScope string

const (
	ModelTagScope   TagScope = "model"
	DatasetTagScope TagScope = "dataset"
	CodeTagScope    TagScope = "code"
	SpaceTagScope   TagScope = "space"
)

const defaultTagGroup = ""

type Tag struct {
	ID       int64    `bun:",pk,autoincrement" json:"id"`
	Name     string   `bun:",notnull" json:"name" yaml:"name"`
	Category string   `bun:",notnull" json:"category" yaml:"category"`
	Group    string   `bun:",notnull" json:"group" yaml:"group"`
	Scope    TagScope `bun:",notnull" json:"scope" yaml:"scope"`
	BuiltIn  bool     `bun:",notnull" json:"built_in" yaml:"built_in"`
	ShowName string   `bun:"" json:"show_name" yaml:"show_name"`
	times
}

// TagCategory represents the category of tags
type TagCategory struct {
	ID    int64    `bun:",pk,autoincrement" json:"id"`
	Name  string   `bun:",notnull" json:"name" yaml:"name"`
	Scope TagScope `bun:",notnull" json:"scope" yaml:"scope"`
}

// Alltags returns all tags in the database
func (ts *TagStore) AllTags(ctx context.Context) ([]Tag, error) {
	var tags []Tag
	err := ts.db.Operator.Core.NewSelect().Model(&Tag{}).Scan(ctx, &tags)
	if err != nil {
		slog.Error("Failed to select tags", "error", err)
		return nil, err
	}
	return tags, nil
}

func (ts *TagStore) AllTagsByScope(ctx context.Context, scope TagScope) ([]*Tag, error) {
	var tags []*Tag
	err := ts.db.Operator.Core.NewSelect().Model(&tags).
		Where("scope =?", scope).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select tags by scope,cause: %w", err)
	}
	return tags, nil
}

func (ts *TagStore) AllTagsByScopeAndCategory(ctx context.Context, scope TagScope, category string) ([]*Tag, error) {
	var tags []*Tag
	err := ts.db.Operator.Core.NewSelect().Model(&tags).
		Where("scope =? and category = ?", scope, category).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select tags by scope,cause: %w", err)
	}
	return tags, nil
}

func (ts *TagStore) AllModelTags(ctx context.Context) ([]*Tag, error) {
	return ts.AllTagsByScope(ctx, ModelTagScope)
}

func (ts *TagStore) AllDatasetTags(ctx context.Context) ([]*Tag, error) {
	return ts.AllTagsByScope(ctx, DatasetTagScope)
}

func (ts *TagStore) AllCodeTags(ctx context.Context) ([]*Tag, error) {
	return ts.AllTagsByScope(ctx, CodeTagScope)
}

func (ts *TagStore) AllSpaceTags(ctx context.Context) ([]*Tag, error) {
	return ts.AllTagsByScope(ctx, SpaceTagScope)
}

func (ts *TagStore) AllModelCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.allCategories(ctx, ModelTagScope)
}

func (ts *TagStore) AllDatasetCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.allCategories(ctx, DatasetTagScope)
}

func (ts *TagStore) AllCodeCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.allCategories(ctx, CodeTagScope)
}

func (ts *TagStore) AllSpaceCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.allCategories(ctx, SpaceTagScope)
}

func (ts *TagStore) allCategories(ctx context.Context, scope TagScope) ([]TagCategory, error) {
	var tags []TagCategory
	err := ts.db.Operator.Core.NewSelect().Model(&TagCategory{}).
		Where("scope = ?", scope).
		Scan(ctx, &tags)
	if err != nil {
		slog.Error("Failed to select tags", "error", err)
		return nil, err
	}
	return tags, nil
}

func (ts *TagStore) CreateTag(ctx context.Context, category, name, group string, scope TagScope) (Tag, error) {
	tag := Tag{
		Name:     name,
		Category: category,
		Group:    group,
		Scope:    scope,
	}
	_, err := ts.db.Operator.Core.NewInsert().Model(&tag).Exec(ctx)
	return tag, err
}

func (ts *TagStore) SaveTags(ctx context.Context, tags []*Tag) error {
	if len(tags) == 0 {
		return nil
	}
	_, err := ts.db.Operator.Core.NewInsert().Model(&tags).Exec(ctx)
	if err != nil {
		slog.Error("Failed to save tags", slog.Any("tags", tags), slog.Any("error", err))
		return fmt.Errorf("failed to save tags,cause: %w", err)
	}
	return nil
}

func (ts *TagStore) UpsertTags(ctx context.Context, tagScope TagScope, categoryTagMap map[string][]string) ([]Tag, error) {
	var tags []Tag
	for category, tagNames := range categoryTagMap {
		ctags := make([]Tag, 0)
		err := ts.db.Operator.Core.NewSelect().Model(&ctags).
			Where("caregory = ? and scope = ?", category, tagScope).
			Scan(ctx)
		if err != nil {
			slog.Error("Failed to select tags", slog.String("category", category),
				slog.Any("scope", tagScope), slog.Any("error", err.Error()))
			return nil, fmt.Errorf("failed to select tags, cause: %w", err)
		}
		tags = append(tags, ctags...)
		for _, tagName := range tagNames {
			if !slices.ContainsFunc(tags, func(t Tag) bool { return t.Name == tagName }) {
				newTag, err := ts.CreateTag(ctx, category, tagName, defaultTagGroup, tagScope)
				if err != nil {
					slog.Error("Failed to create new tag", slog.String("category", category), slog.String("tagName", tagName),
						slog.Any("scope", tagScope), slog.Any("error", err.Error))
					return nil, fmt.Errorf("failed to create new tag, cause: %w", err)
				}
				tags = append(tags, newTag)
			}
		}
	}

	return tags, nil
}

// SetMetaTags will delete existing tags and create new ones
func (ts *TagStore) SetMetaTags(ctx context.Context, repoType types.RepositoryType, namespace, name string, tags []*Tag) (repoTags []*RepositoryTag, err error) {
	repo := new(Repository)
	err = ts.db.Operator.Core.NewSelect().Model(repo).
		Column("id").
		Relation("Tags").
		Where("LOWER(git_path) = LOWER(?)", fmt.Sprintf("%ss_%v/%v", string(repoType), namespace, name)).
		Scan(ctx)
	if err != nil {
		return repoTags, fmt.Errorf("failed to find repository, path:%v/%v,error:%w", namespace, name, err)
	}

	var metaTagIds []int64
	for _, tag := range repo.Tags {
		if tag.Category != "framework" {
			metaTagIds = append(metaTagIds, tag.ID)
		}
	}
	// select all repo tags which are not Library tags
	err = ts.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// remove all tags of the repository not belongs to category "Library", and then add new tags
		tx.NewDelete().
			Model(&RepositoryTag{}).
			Where("repository_id =? and tag_id in (?)", repo.ID, bun.In(metaTagIds)).
			Exec(ctx)
		// no new tags to insert
		if len(tags) == 0 {
			return nil
		}
		for _, tag := range tags {
			if tag.Category == "framework" {
				return errors.New("found framework tag when set meta tag, tag name:" + tag.Name)
			}
			repoTag := &RepositoryTag{RepositoryID: repo.ID, TagID: tag.ID, Repository: repo, Tag: tag, Count: 1}
			repoTags = append(repoTags, repoTag)
		}
		// batch insert
		_, err := tx.NewInsert().Model(&repoTags).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to batch insert repository meta tags, path:%v/%v,error:%w", namespace, name, err)
		}
		return nil
	})

	return repoTags, err
}

func (ts *TagStore) SetLibraryTag(ctx context.Context, repoType types.RepositoryType, namespace, name string, newTag, oldTag *Tag) (err error) {
	slog.Debug("set library tag", slog.Any("newTag", newTag), slog.Any("oldTag", oldTag))
	repo := new(Repository)
	err = ts.db.Operator.Core.NewSelect().Model(repo).
		Column("id").
		Where("LOWER(git_path) = LOWER(?)", fmt.Sprintf("%ss_%v/%v", string(repoType), namespace, name)).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to find repository, path:%v/%v,error:%w", namespace, name, err)
	}
	err = ts.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		// decrease count of old tag
		if oldTag != nil {
			oldRepoTag := RepositoryTag{RepositoryID: repo.ID, TagID: oldTag.ID}
			_, err = tx.NewUpdate().Model(&oldRepoTag).
				Set("count = count-1").
				Where("repository_id = ? and tag_id = ? and count > 0", repo.ID, oldTag.ID).
				Exec(ctx)
			if err != nil {
				return err
			}
		}
		// increase count of new tag
		if newTag != nil {
			newRepoTag := RepositoryTag{RepositoryID: repo.ID, TagID: newTag.ID}
			err = tx.NewSelect().Model(&newRepoTag).
				Where("repository_id = ? and tag_id = ?", repo.ID, newTag.ID).
				Scan(ctx)

			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if newRepoTag.ID == 0 {
				newRepoTag.Count = 1
				_, err = tx.NewInsert().Model(&newRepoTag).Exec(ctx)
			} else {
				_, err = tx.NewUpdate().Model(&newRepoTag).Where("id = ?", newRepoTag.ID).Set("count = count+1").Exec(ctx)
			}
		}
		return err
	})
	if err != nil {
		slog.Error("Failed to update repository library tags", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("oldTag", oldTag), slog.Any("newTag", newTag), slog.Any("error", err))
		return fmt.Errorf("failed to update repository library tags, path:%v/%v,error:%w", namespace, name, err)
	}

	return err
}

func (ts *TagStore) UpsertRepoTags(ctx context.Context, repoID int64, oldTagIDs, newTagIDs []int64) (err error) {
	err = ts.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		if len(oldTagIDs) > 0 {
			for _, tagID := range oldTagIDs {
				_, err = tx.NewUpdate().Model((*RepositoryTag)(nil)).
					Where("repository_id = ? and tag_id = ? and count > 0", repoID, tagID).
					Set("count = count-1").
					Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to delete repository tags,error:%w", err)
				}
			}
		}
		// increase count of new tag
		if len(newTagIDs) > 0 {
			for _, tagID := range newTagIDs {
				newRepoTag := RepositoryTag{
					RepositoryID: repoID,
					TagID:        tagID,
					Count:        1,
				}
				_, err = tx.NewInsert().Model(&newRepoTag).
					On("CONFLICT (repository_id, tag_id) DO UPDATE SET count = repository_tag.count+1").
					Exec(ctx)

				if err != nil {
					return fmt.Errorf("failed to upsert repository tags,error:%w", err)
				}

			}
		}
		return nil
	})

	return err
}

func (ts *TagStore) FindOrCreate(ctx context.Context, tag Tag) (*Tag, error) {
	var resTag Tag
	err := ts.db.Operator.Core.NewSelect().
		Model(&resTag).
		Where("name = ? and category = ? and built_in = ? and scope = ?", tag.Name, tag.Category, tag.BuiltIn, tag.Scope).
		Scan(ctx)
	if err == nil {
		return &resTag, nil
	}
	_, err = ts.db.Operator.Core.NewInsert().
		Model(&tag).
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &resTag, err
}
