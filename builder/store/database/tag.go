package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type tagStoreImpl struct {
	db *DB
}

type TagStore interface {
	// Alltags returns all tags in the database
	AllTags(ctx context.Context, filter *types.TagFilter) ([]*Tag, error)
	AllModelTags(ctx context.Context) ([]*Tag, error)
	AllPromptTags(ctx context.Context) ([]*Tag, error)
	AllDatasetTags(ctx context.Context) ([]*Tag, error)
	AllCodeTags(ctx context.Context) ([]*Tag, error)
	AllSpaceTags(ctx context.Context) ([]*Tag, error)
	AllCategories(ctx context.Context, scope types.TagScope) ([]TagCategory, error)
	AllModelCategories(ctx context.Context) ([]TagCategory, error)
	AllPromptCategories(ctx context.Context) ([]TagCategory, error)
	AllDatasetCategories(ctx context.Context) ([]TagCategory, error)
	AllCodeCategories(ctx context.Context) ([]TagCategory, error)
	AllSpaceCategories(ctx context.Context) ([]TagCategory, error)
	CreateTag(ctx context.Context, category, name, group string, scope types.TagScope) (Tag, error)
	SaveTags(ctx context.Context, tags []*Tag) error
	// SetMetaTags will delete existing tags and create new ones
	SetMetaTags(ctx context.Context, repoType types.RepositoryType, namespace, name string, tags []*Tag) (repoTags []*RepositoryTag, err error)
	SetLibraryTag(ctx context.Context, repoType types.RepositoryType, namespace, name string, newTag, oldTag *Tag) (err error)
	UpsertRepoTags(ctx context.Context, repoID int64, oldTagIDs, newTagIDs []int64) (err error)
	RemoveRepoTags(ctx context.Context, repoID int64, tagIDs []int64) (err error)
	RemoveRepoTagsByCategory(ctx context.Context, repoID int64, category []string) (err error)
	FindOrCreate(ctx context.Context, tag Tag) (*Tag, error)
	FindTag(ctx context.Context, name, scope, category string) (*Tag, error)
	FindTagByID(ctx context.Context, id int64) (*Tag, error)
	UpdateTagByID(ctx context.Context, tag *Tag) (*Tag, error)
	DeleteTagByID(ctx context.Context, id int64) error
	CreateCategory(ctx context.Context, category TagCategory) (*TagCategory, error)
	UpdateCategory(ctx context.Context, category TagCategory) (*TagCategory, error)
	DeleteCategory(ctx context.Context, id int64) error
}

func NewTagStore() TagStore {
	return &tagStoreImpl{
		db: defaultDB,
	}
}

func NewTagStoreWithDB(db *DB) TagStore {
	return &tagStoreImpl{
		db: db,
	}
}

type Tag struct {
	ID       int64          `bun:",pk,autoincrement" json:"id"`
	Name     string         `bun:",notnull" json:"name" yaml:"name"`
	Category string         `bun:",notnull" json:"category" yaml:"category"`
	Group    string         `bun:",notnull" json:"group" yaml:"group"`
	Scope    types.TagScope `bun:",notnull" json:"scope" yaml:"scope"`
	BuiltIn  bool           `bun:",notnull" json:"built_in" yaml:"built_in"`
	ShowName string         `bun:"" json:"show_name" yaml:"show_name"`
	times
}

// TagCategory represents the category of tags
type TagCategory struct {
	ID       int64          `bun:",pk,autoincrement" json:"id"`
	Name     string         `bun:",notnull" json:"name" yaml:"name"`
	ShowName string         `bun:"" json:"show_name" yaml:"show_name"`
	Scope    types.TagScope `bun:",notnull" json:"scope" yaml:"scope"`
	Enabled  bool           `bun:"default:true" json:"enabled" yaml:"enabled"`
}

// Alltags returns all tags in the database
func (ts *tagStoreImpl) AllTags(ctx context.Context, filter *types.TagFilter) ([]*Tag, error) {
	var tags []*Tag
	q := ts.db.Operator.Core.NewSelect().Model(&Tag{})

	if filter != nil {
		if len(filter.Scopes) > 0 {
			q = q.Where("scope in (?)", bun.In(filter.Scopes))
		}
		if len(filter.Categories) > 0 {
			q = q.Where("category in (?)", bun.In(filter.Categories))
		}
		if filter.BuiltIn != nil {
			q = q.Where("built_in = ?", *filter.BuiltIn)
		}
	}

	err := q.Scan(ctx, &tags)
	if err != nil {
		return nil, fmt.Errorf("failed to select tags,cause: %w", err)
	}
	return tags, nil
}

func (ts *tagStoreImpl) AllTagsByScope(ctx context.Context, scope types.TagScope) ([]*Tag, error) {
	filter := &types.TagFilter{
		Scopes: []types.TagScope{scope},
	}
	return ts.AllTags(ctx, filter)
}

func (ts *tagStoreImpl) AllTagsByScopeAndCategory(ctx context.Context, scope types.TagScope, category string) ([]*Tag, error) {
	filter := &types.TagFilter{
		Scopes:     []types.TagScope{scope},
		Categories: []string{category},
	}
	return ts.AllTags(ctx, filter)
}

func (ts *tagStoreImpl) GetTagsByScopeAndCategories(ctx context.Context, scope types.TagScope, categories []string) ([]*Tag, error) {
	filter := &types.TagFilter{
		Scopes:     []types.TagScope{scope},
		Categories: categories,
	}
	return ts.AllTags(ctx, filter)
}

func (ts *tagStoreImpl) AllModelTags(ctx context.Context) ([]*Tag, error) {
	return ts.AllTagsByScope(ctx, types.ModelTagScope)
}

func (ts *tagStoreImpl) AllPromptTags(ctx context.Context) ([]*Tag, error) {
	return ts.AllTagsByScope(ctx, types.PromptTagScope)
}

func (ts *tagStoreImpl) AllDatasetTags(ctx context.Context) ([]*Tag, error) {
	return ts.AllTagsByScope(ctx, types.DatasetTagScope)
}

func (ts *tagStoreImpl) AllCodeTags(ctx context.Context) ([]*Tag, error) {
	return ts.AllTagsByScope(ctx, types.CodeTagScope)
}

func (ts *tagStoreImpl) AllSpaceTags(ctx context.Context) ([]*Tag, error) {
	return ts.AllTagsByScope(ctx, types.SpaceTagScope)
}

func (ts *tagStoreImpl) AllModelCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.AllCategories(ctx, types.ModelTagScope)
}

func (ts *tagStoreImpl) AllPromptCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.AllCategories(ctx, types.PromptTagScope)
}

func (ts *tagStoreImpl) AllDatasetCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.AllCategories(ctx, types.DatasetTagScope)
}

func (ts *tagStoreImpl) AllCodeCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.AllCategories(ctx, types.CodeTagScope)
}

func (ts *tagStoreImpl) AllSpaceCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.AllCategories(ctx, types.SpaceTagScope)
}

func (ts *tagStoreImpl) AllCategories(ctx context.Context, scope types.TagScope) ([]TagCategory, error) {
	var tags []TagCategory
	q := ts.db.Operator.Core.NewSelect().Model(&TagCategory{})
	if len(scope) > 0 {
		q = q.Where("scope = ?", scope)
	}
	err := q.Order("id").Scan(ctx, &tags)
	if err != nil {
		slog.Error("Failed to select tags", "error", err)
		return nil, err
	}
	return tags, nil
}

func (ts *tagStoreImpl) CreateTag(ctx context.Context, category, name, group string, scope types.TagScope) (Tag, error) {
	tag := Tag{
		Name:     name,
		Category: category,
		Group:    group,
		Scope:    scope,
	}
	_, err := ts.db.Operator.Core.NewInsert().Model(&tag).Exec(ctx)
	return tag, err
}

func (ts *tagStoreImpl) SaveTags(ctx context.Context, tags []*Tag) error {
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

// SetMetaTags will delete existing tags and create new ones
func (ts *tagStoreImpl) SetMetaTags(ctx context.Context, repoType types.RepositoryType, namespace, name string, tags []*Tag) (repoTags []*RepositoryTag, err error) {
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
	exCategories := map[string]bool{
		"framework":         true,
		"runtime_framework": true,
		"evaluation":        true,
	}
	for _, tag := range repo.Tags {
		if !exCategories[tag.Category] {
			metaTagIds = append(metaTagIds, tag.ID)
		}
	}
	// select all repo tags which are not Library tags
	err = ts.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// remove all tags of the repository not belongs to category "Library", and then add new tags
		_, err := tx.NewDelete().
			Model(&RepositoryTag{}).
			Where("repository_id =? and tag_id in (?)", repo.ID, bun.In(metaTagIds)).
			Exec(ctx)
		if err != nil {
			return err
		}
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
		_, err = tx.NewInsert().Model(&repoTags).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to batch insert repository meta tags, path:%v/%v,error:%w", namespace, name, err)
		}
		return nil
	})

	return repoTags, err
}

func (ts *tagStoreImpl) SetLibraryTag(ctx context.Context, repoType types.RepositoryType, namespace, name string, newTag, oldTag *Tag) (err error) {
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

func (ts *tagStoreImpl) UpsertRepoTags(ctx context.Context, repoID int64, oldTagIDs, newTagIDs []int64) (err error) {
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

func (ts *tagStoreImpl) RemoveRepoTags(ctx context.Context, repoID int64, tagIDs []int64) (err error) {
	if len(tagIDs) == 0 {
		return nil
	}
	_, err = ts.db.Operator.Core.NewDelete().
		Model(&RepositoryTag{}).
		Where("repository_id =? and tag_id in (?)", repoID, bun.In(tagIDs)).
		Exec(ctx)

	return err
}

// RemoveRepoTagsByCategory
func (ts *tagStoreImpl) RemoveRepoTagsByCategory(ctx context.Context, repoID int64, category []string) error {
	_, err := ts.db.Operator.Core.NewDelete().
		Model(&RepositoryTag{}).
		Where("repository_id =? and tag_id in (select id from tags where category in (?))", repoID, bun.In(category)).
		Exec(ctx)
	return err
}

func (ts *tagStoreImpl) FindOrCreate(ctx context.Context, tag Tag) (*Tag, error) {
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
	return &tag, err
}

// find tag by name
func (ts *tagStoreImpl) FindTag(ctx context.Context, name, scope, category string) (*Tag, error) {
	var tag Tag
	err := ts.db.Operator.Core.NewSelect().
		Model(&tag).
		Where("name = ? and scope = ? and category = ?", name, scope, category).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// find tag by id
func (ts *tagStoreImpl) FindTagByID(ctx context.Context, id int64) (*Tag, error) {
	var tag Tag
	err := ts.db.Operator.Core.NewSelect().
		Model(&tag).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select tag by id %d error: %w", id, err)
	}
	return &tag, nil
}

func (ts *tagStoreImpl) UpdateTagByID(ctx context.Context, tag *Tag) (*Tag, error) {
	_, err := ts.db.Operator.Core.NewUpdate().
		Model(tag).WherePK().Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update tag by id %d error: %w", tag.ID, err)
	}
	return tag, nil
}

func (ts *tagStoreImpl) DeleteTagByID(ctx context.Context, id int64) error {
	err := ts.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewDelete().
			Model(&Tag{}).
			Where("id = ?", id).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("delete tag by id %d error: %w", id, err)
		}
		_, err = tx.NewDelete().
			Model(&RepositoryTag{}).
			Where("tag_id = ?", id).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("delete repository_tag by tag_id %d error: %w", id, err)
		}
		return nil
	})
	return err

}

func (ts *tagStoreImpl) CreateCategory(ctx context.Context, category TagCategory) (*TagCategory, error) {
	_, err := ts.db.Operator.Core.NewInsert().Model(&category).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("insert category error: %w", err)
	}
	return &category, nil
}

func (ts *tagStoreImpl) UpdateCategory(ctx context.Context, category TagCategory) (*TagCategory, error) {
	_, err := ts.db.Operator.Core.NewUpdate().Model(&category).WherePK().Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update category by id %d, error: %w", category.ID, err)
	}
	return &category, nil
}

func (ts *tagStoreImpl) DeleteCategory(ctx context.Context, id int64) error {
	_, err := ts.db.Operator.Core.NewDelete().Model(&TagCategory{}).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete category by id %d, error: %w", id, err)
	}
	return nil
}
