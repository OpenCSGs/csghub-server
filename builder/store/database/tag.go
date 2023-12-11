package database

import (
	"context"
	"log/slog"
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
	ModelTagScope    TagScope = "model"
	DatabaseTagScope TagScope = "database"
)

type Tag struct {
	ID       int64    `bun:",pk,autoincrement" json:"id"`
	Name     string   `bun:",notnull" json:"name"`
	Category string   `bun:",notnull" json:"category"`
	Group    string   `bun:",notnull" json:"group"`
	Scope    TagScope `bun:",notnull" json:"scope"`
	times
}

func (ts *TagStore) AllTags(ctx context.Context) ([]Tag, error) {
	var tags []Tag
	err := ts.db.Operator.Core.NewSelect().Model(&Tag{}).Scan(ctx, &tags)
	if err != nil {
		slog.Error("Failed to select tags", "error", err)
		return nil, err
	}
	return tags, nil
}
