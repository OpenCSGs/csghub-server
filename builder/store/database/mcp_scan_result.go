package database

import (
	"context"
	"fmt"
)

type MCPScanResult struct {
	ID           int64  `bun:",pk,autoincrement" json:"id"`
	FilePath     string `bun:",notnull" json:"file_path"`
	RepositoryID int64  `bun:",notnull" json:"repository_id"`
	CommitID     string `bun:",notnull" json:"commit_id"`
	Title        string `bun:",notnull" json:"title"`
	RiskLevel    string `bun:",notnull" json:"risk_level"` // critical, high, medium, low, pass
	Detail       string `bun:",notnull" json:"detail"`
	times
}

type MCPScanResultStore interface {
	Create(ctx context.Context, input MCPScanResult) (*MCPScanResult, error)
	Update(ctx context.Context, input MCPScanResult) (*MCPScanResult, error)
	BatchCreate(ctx context.Context, input []MCPScanResult) error
	// FindByRepoID(ctx context.Context, repoID int64) (*MCPScanResult, error)
	// Delete(ctx context.Context, input MCPScanResult) error
	// DeleteByRepoID(ctx context.Context, repoID int64) error
}

type mcpScanResultStoreImpl struct {
	db *DB
}

func NewMCPScanResultStore() MCPScanResultStore {
	return &mcpScanResultStoreImpl{
		db: defaultDB,
	}
}

func NewMCPScanResultStoreWithDB(db *DB) MCPScanResultStore {
	return &mcpScanResultStoreImpl{
		db: db,
	}
}

func (store *mcpScanResultStoreImpl) Create(ctx context.Context, input MCPScanResult) (*MCPScanResult, error) {
	res, err := store.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("insert mcp scan result in db error: %w", err)
	}
	return &input, nil
}

func (store *mcpScanResultStoreImpl) Update(ctx context.Context, input MCPScanResult) (*MCPScanResult, error) {
	res, err := store.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("update mcp scan result %d error: %w", input.ID, err)
	}
	return &input, nil
}

func (store *mcpScanResultStoreImpl) BatchCreate(ctx context.Context, input []MCPScanResult) error {
	_, err := store.db.Operator.Core.NewInsert().
		Model(&input).
		Exec(ctx)
	return err
}
