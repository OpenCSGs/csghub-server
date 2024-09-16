package database

import (
	"context"
	"fmt"
)

type LLMConfigStore struct {
	db *DB
}

type LLMConfig struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	ModelName   string `bun:",notnull" json:"model_name"`
	ApiEndpoint string `bun:",notnull" json:"api_endpoint"`
	AuthHeader  string `bun:",notnull" json:"auth_header"`
	Type        int    `bun:",notnull" json:"type"` // 1: optimization, 2: comparson
	Enabled     bool   `bun:",notnull" json:"enabled"`
	times
}

func NewLLMConfigStore() *LLMConfigStore {
	return &LLMConfigStore{db: defaultDB}
}

func (s *LLMConfigStore) GetOptimization(ctx context.Context) (*LLMConfig, error) {
	var config LLMConfig
	err := s.db.Operator.Core.NewSelect().Model(&config).Where("type = 1 and enabled = true").Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select optimization llm, %w", err)
	}
	return &config, nil
}
