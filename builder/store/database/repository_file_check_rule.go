package database

import (
	"context"
	"time"
)

type RepositoryFileCheckRule struct {
	ID        int64     `bun:",pk,autoincrement"`
	RuleType  string    `bun:"rule_type,notnull,unique:idx_rule_type_pattern"`
	Pattern   string    `bun:"pattern,notnull,unique:idx_rule_type_pattern"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}

type RepositoryFileCheckRuleStore interface {
	Create(ctx context.Context, ruleType, pattern string) (*RepositoryFileCheckRule, error)
	List(ctx context.Context) ([]RepositoryFileCheckRule, error)
	ListByRuleType(ctx context.Context, ruleType string) ([]RepositoryFileCheckRule, error)
	Delete(ctx context.Context, ruleType, pattern string) error
	Exists(ctx context.Context, ruleType, pattern string) (bool, error)
}

type repositoryFileCheckRuleStore struct {
	db *DB
}

func NewRepositoryFileCheckRuleStore() RepositoryFileCheckRuleStore {
	return &repositoryFileCheckRuleStore{db: GetDB()}
}

func NewRepositoryFileCheckRuleStoreWithDB(db *DB) RepositoryFileCheckRuleStore {
	return &repositoryFileCheckRuleStore{db: db}
}

func (s *repositoryFileCheckRuleStore) Create(ctx context.Context, ruleType, pattern string) (*RepositoryFileCheckRule, error) {
	rule := &RepositoryFileCheckRule{RuleType: ruleType, Pattern: pattern}
	_, err := s.db.Operator.Core.NewInsert().Model(rule).Exec(ctx)
	return rule, err
}

func (s *repositoryFileCheckRuleStore) List(ctx context.Context) ([]RepositoryFileCheckRule, error) {
	var rules []RepositoryFileCheckRule
	err := s.db.Operator.Core.NewSelect().Model(&rules).Scan(ctx)
	return rules, err
}

func (s *repositoryFileCheckRuleStore) ListByRuleType(ctx context.Context, ruleType string) ([]RepositoryFileCheckRule, error) {
	var rules []RepositoryFileCheckRule
	err := s.db.Operator.Core.NewSelect().Model(&rules).Where("rule_type = ?", ruleType).Scan(ctx)
	return rules, err
}

func (s *repositoryFileCheckRuleStore) Delete(ctx context.Context, ruleType, pattern string) error {
	_, err := s.db.Operator.Core.NewDelete().Model((*RepositoryFileCheckRule)(nil)).
		Where("rule_type = ?", ruleType).
		Where("pattern = ?", pattern).
		Exec(ctx)
	return err
}

func (s *repositoryFileCheckRuleStore) Exists(ctx context.Context, ruleType, pattern string) (bool, error) {
	exists, err := s.db.Operator.Core.NewSelect().Model((*RepositoryFileCheckRule)(nil)).
		Where("rule_type = ?", ruleType).
		Where("pattern = ?", pattern).
		Exists(ctx)
	return exists, err
}
