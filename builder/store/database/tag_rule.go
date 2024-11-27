package database

import (
	"context"
	"time"
)

type TagRuleStore struct {
	db *DB
}

func NewTagRuleStore() *TagRuleStore {
	return &TagRuleStore{db: defaultDB}
}

// internal use for the relation between dataset and tag
type TagRule struct {
	ID               int64     `bun:",pk,autoincrement" json:"id"`
	RepoName         string    `bun:",notnull" json:"repo_name"`
	RepoType         string    `bun:",notnull" json:"repo_type"`
	Category         string    `bun:",notnull" json:"category"`
	TagName          string    `bun:",notnull" json:"tag_name"`
	RuntimeFramework string    `bun:"," json:"runtime_framework"`
	Tag              Tag       `bun:",rel:has-one,join:tag_name=name"`
	CreatedAt        time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
}

// find dataset tag by name and related tag with tag name
func (s *TagRuleStore) FindByRepo(ctx context.Context, category string, repoName string, repoType string) (*TagRule, error) {
	var TagRule TagRule
	err := s.db.Core.NewSelect().
		Model(&TagRule).
		Relation("Tag").
		Where("LOWER(tag_rule.repo_name) = LOWER(?) AND tag_rule.repo_type = ?", repoName, repoType).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &TagRule, nil
}
