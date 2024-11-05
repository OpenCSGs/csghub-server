package database

import (
	"context"
	"fmt"
)

type PromptPrefixStore struct {
	db *DB
}

type PromptPrefix struct {
	ID int64  `bun:",pk,autoincrement" json:"id"`
	ZH string `bun:",notnull" json:"zh"`
	EN string `bun:",notnull" json:"en"`
}

func NewPromptPrefixStore() *PromptPrefixStore {
	return &PromptPrefixStore{db: defaultDB}
}

func (p *PromptPrefixStore) Get(ctx context.Context) (*PromptPrefix, error) {
	var prefix PromptPrefix
	err := p.db.Operator.Core.NewSelect().Model(&prefix).Order("id desc").Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select latest prompt prefix: %w", err)
	}
	return &prefix, nil
}
