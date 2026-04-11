package database

import (
	"context"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

const (
	RuleTypeNamespace = "namespace"
	RuleTypeModelName = "model_name"
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
	MatchRegex(ctx context.Context, ruleType, targetString string) (bool, error)
	ListBySensitiveCheckTargets(ctx context.Context, namespaces []string, modelID string) ([]RepositoryFileCheckRule, error)
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
	pattern = strings.ToLower(pattern)
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
	pattern = strings.ToLower(pattern)
	_, err := s.db.Operator.Core.NewDelete().Model((*RepositoryFileCheckRule)(nil)).
		Where("rule_type = ?", ruleType).
		Where("pattern = ?", pattern).
		Exec(ctx)
	return err
}

func (s *repositoryFileCheckRuleStore) Exists(ctx context.Context, ruleType, pattern string) (bool, error) {
	pattern = strings.ToLower(pattern)
	exists, err := s.db.Operator.Core.NewSelect().Model((*RepositoryFileCheckRule)(nil)).
		Where("rule_type = ?", ruleType).
		Where("pattern = ?", pattern).
		Exists(ctx)
	return exists, err
}

func (s *repositoryFileCheckRuleStore) MatchRegex(ctx context.Context, ruleType, targetString string) (bool, error) {
	exists, err := s.db.Operator.Core.NewSelect().Model((*RepositoryFileCheckRule)(nil)).
		Where("rule_type = ?", ruleType).
		Where("? ~* pattern", targetString).
		Exists(ctx)
	return exists, err
}

func (s *repositoryFileCheckRuleStore) ListBySensitiveCheckTargets(ctx context.Context, namespaces []string, modelID string) ([]RepositoryFileCheckRule, error) {
	loweredNamespaces := make([]string, 0, len(namespaces))
	for _, namespace := range namespaces {
		namespace = strings.ToLower(strings.TrimSpace(namespace))
		if namespace == "" {
			continue
		}
		loweredNamespaces = append(loweredNamespaces, namespace)
	}
	modelID = strings.ToLower(strings.TrimSpace(modelID))
	var rules []RepositoryFileCheckRule
	query := s.db.Operator.Core.NewSelect().Model(&rules)
	if len(loweredNamespaces) > 0 {
		query = query.WhereGroup(" OR ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("rule_type = ? AND pattern IN (?)", RuleTypeNamespace, bun.In(loweredNamespaces)).
				WhereOr("rule_type = ? AND pattern = ?", RuleTypeModelName, modelID)
		})
	} else {
		query = query.Where("rule_type = ? AND pattern = ?", RuleTypeModelName, modelID)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, err
	}
	return rules, nil
}
