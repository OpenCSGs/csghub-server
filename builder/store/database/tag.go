package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/uptrace/bun"
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
)

const defaultTagGroup = ""

type Tag struct {
	ID       int64    `bun:",pk,autoincrement" json:"id"`
	Name     string   `bun:",notnull" json:"name" yaml:"name"`
	Category string   `bun:",notnull" json:"category" yaml:"category"`
	Group    string   `bun:",notnull" json:"group" yaml:"group"`
	Scope    TagScope `bun:",notnull" json:"scope" yaml:"scope"`
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

func (ts *TagStore) allTagsByScope(ctx context.Context, scope TagScope) ([]*Tag, error) {
	var tags []*Tag
	err := ts.db.Operator.Core.NewSelect().Model(&tags).
		Where("scope =?", scope).
		Scan(ctx)
	if err != nil {
		slog.Error("Failed to select tags by scope", slog.Any("scope", scope), slog.Any("error", err))
		return nil, fmt.Errorf("failed to select tags by scope,cause: %w", err)
	}
	return tags, nil

}

func (ts *TagStore) AllModelTags(ctx context.Context) ([]*Tag, error) {
	return ts.allTagsByScope(ctx, ModelTagScope)
}

func (ts *TagStore) AllDatasetTags(ctx context.Context) ([]*Tag, error) {
	return ts.allTagsByScope(ctx, DatasetTagScope)
}

func (ts *TagStore) AllModelCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.allCategories(ctx, ModelTagScope)
}

func (ts *TagStore) AllDatasetCategories(ctx context.Context) ([]TagCategory, error) {
	return ts.allCategories(ctx, DatasetTagScope)
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
func (ts *TagStore) SetMetaTags(ctx context.Context, namespace, name string, tags []*Tag) (repoTags []*RepositoryTag, err error) {
	repo := new(Repository)
	err = ts.db.Operator.Core.NewSelect().Model(repo).
		Relation("Tags").
		Where("path =?", fmt.Sprintf("%v/%v", namespace, name)).
		Scan(ctx)
	if err != nil {
		return repoTags, fmt.Errorf("failed to find repository, path:%v/%v,error:%w", namespace, name, err)
	}

	var metaTagIds []int64
	for _, tag := range repo.Tags {
		if tag.Category != "Libraries" {
			metaTagIds = append(metaTagIds, tag.ID)
		}
	}
	//select all repo tags which are not Library tags
	err = ts.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		//remove all tags of the repository not belongs to category "Library", and then add new tags
		tx.NewDelete().
			Model(&RepositoryTag{}).
			Where("repository_id =? and tag_id in (?)", repo.ID, bun.In(metaTagIds)).
			Exec(ctx)
		//no new tags to insert
		if len(tags) == 0 {
			return nil
		}
		for _, tag := range tags {
			if tag.Category == "Libraries" {
				return errors.New("found Library tag when set meta tag, tag name:" + tag.Name)
			}
			repoTag := &RepositoryTag{RepositoryID: repo.ID, TagID: tag.ID, Repository: repo, Tag: tag}
			repoTags = append(repoTags, repoTag)
		}
		//batch insert
		_, err := tx.NewInsert().Model(&repoTags).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to batch insert repository meta tags, path:%v/%v,error:%w", namespace, name, err)
		}
		return nil
	})

	return repoTags, err
}

func (ts *TagStore) SetLibraryTag(ctx context.Context, namespace, name string, newTag, oldTag *Tag) (err error) {
	repo := new(Repository)
	err = ts.db.Operator.Core.NewSelect().Model(repo).
		Where("path =?", fmt.Sprintf("%v/%v", namespace, name)).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to find repository, path:%v/%v,error:%w", namespace, name, err)
	}
	err = ts.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		//TODO:implement tag counting logic
		//decrease count of old tag
		//increase count of new tag
		if err != nil {
			return fmt.Errorf("failed to update repository library tags, path:%v/%v,error:%w", namespace, name, err)
		}
		return nil
	})

	return err
}
