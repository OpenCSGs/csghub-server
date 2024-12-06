package database

import (
	"context"
	"time"
)

type tagRuleStoreImpl struct {
	db *DB
}

type TagRuleStore interface {
	// find dataset tag by name and related tag with tag name
	FindByRepo(ctx context.Context, category, namespace, repoName, repoType string) (*TagRule, error)
}

func NewTagRuleStore() TagRuleStore {
	return &tagRuleStoreImpl{db: defaultDB}
}

func NewTagRuleStoreWithDB(db *DB) TagRuleStore {
	return &tagRuleStoreImpl{db: db}
}

// internal use for the relation between dataset and tag
type TagRule struct {
	ID               int64     `bun:",pk,autoincrement" json:"id"`
	Namespace        string    `bun:",notnull" json:"namespace"`
	RepoName         string    `bun:",notnull" json:"repo_name"`
	RepoType         string    `bun:",notnull" json:"repo_type"`
	Category         string    `bun:",notnull" json:"category"`
	TagName          string    `bun:",notnull" json:"tag_name"`
	RuntimeFramework string    `bun:"," json:"runtime_framework"`
	Source           string    `bun:"," json:"source"`
	Tag              Tag       `bun:",rel:has-one,join:tag_name=name"`
	CreatedAt        time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
}

// find dataset tag by name and related tag with tag name
func (s *tagRuleStoreImpl) FindByRepo(ctx context.Context, category, namespace, repoName, repoType string) (*TagRule, error) {
	var TagRule TagRule
	err := s.db.Core.NewSelect().
		Model(&TagRule).
		Relation("Tag").
		Where("LOWER(tag_rule.repo_name) = LOWER(?) AND tag_rule.namespace =? AND tag_rule.repo_type = ? AND tag_rule.category = ?", repoName, namespace, repoType, category).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &TagRule, nil
}
