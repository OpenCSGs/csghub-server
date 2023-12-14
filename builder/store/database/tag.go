package database

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
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
