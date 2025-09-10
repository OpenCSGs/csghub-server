//go:build !ee && !saas

package database

import (
	"context"

	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/enum"
)

type RuleStoreImpl struct {
	db *DB
}

type RuleStore interface {
	FindByRuleType(ctx context.Context, ruleType enum.RuleType) (types.Rule, error)
	Update(ctx context.Context, rule types.Rule) (types.Rule, error)
	Create(ctx context.Context, rule types.Rule) (types.Rule, error)
}

func NewRuleStore() RuleStore {
	return &RuleStoreImpl{db: defaultDB}
}

func NewRuleStoreWithDB(db *DB) RuleStore {
	return &RuleStoreImpl{db: db}
}

type Rule struct {
	ID       int64         `bun:"id,pk,autoincrement"`
	Content  string        `bun:",notnull"`
	RuleType enum.RuleType `bun:",notnull,unique"`

	times
}

func (s *RuleStoreImpl) FindByRuleType(ctx context.Context, ruleType enum.RuleType) (types.Rule, error) {
	return types.Rule{}, nil
}

func (s *RuleStoreImpl) Update(ctx context.Context, rule types.Rule) (types.Rule, error) {
	return types.Rule{}, nil
}

func (s *RuleStoreImpl) Create(ctx context.Context, rule types.Rule) (types.Rule, error) {
	return types.Rule{}, nil
}
