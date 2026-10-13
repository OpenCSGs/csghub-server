package database

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/errorx"
)

type PromptVersion struct {
	ID int64 `bun:",pk,autoincrement" json:"id"`
	// PromptID identifies the Prompt repository that owns this version.
	PromptID int64 `bun:",notnull" json:"prompt_id"`
	// FilePath identifies the prompt file within the repository.
	FilePath string `bun:",notnull" json:"file_path"`
	// Version is the user-defined version name, for example v1.
	Version string `bun:",notnull" json:"version"`
	// Hash points to the latest Git commit saved for this editable version.
	Hash string `bun:",notnull" json:"hash"`
	// Changelog describes why the version was created.
	Changelog string `bun:",type:text" json:"changelog"`
	// times provides CreatedAt and UpdatedAt timestamps.
	times
}

type PromptVersionStore interface {
	Create(ctx context.Context, input PromptVersion) (*PromptVersion, error)
	ByPromptIDAndFilePath(ctx context.Context, promptID int64, filePath string) ([]PromptVersion, error)
	ByPromptIDFilePathAndVersion(ctx context.Context, promptID int64, filePath, version string) (*PromptVersion, error)
	UpdateHash(ctx context.Context, id int64, hash string) (*PromptVersion, error)
}

type promptVersionStoreImpl struct {
	db *DB
}

func NewPromptVersionStore() PromptVersionStore {
	return &promptVersionStoreImpl{db: defaultDB}
}

func NewPromptVersionStoreWithDB(db *DB) PromptVersionStore {
	return &promptVersionStoreImpl{db: db}
}

func (s *promptVersionStoreImpl) Create(ctx context.Context, input PromptVersion) (*PromptVersion, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		err = errorx.HandleDBError(err, errorx.Ctx().Set("prompt_id", input.PromptID).Set("file_path", input.FilePath).Set("version", input.Version))
		return nil, fmt.Errorf("create prompt version: %w", err)
	}
	return &input, nil
}

func (s *promptVersionStoreImpl) ByPromptIDAndFilePath(ctx context.Context, promptID int64, filePath string) ([]PromptVersion, error) {
	var versions []PromptVersion
	err := s.db.Operator.Core.NewSelect().Model(&versions).
		Where("prompt_id = ? AND file_path = ?", promptID, filePath).
		Order("created_at DESC", "id DESC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list prompt versions: %w", errorx.HandleDBError(err, errorx.Ctx().Set("prompt_id", promptID).Set("file_path", filePath)))
	}
	return versions, nil
}

func (s *promptVersionStoreImpl) ByPromptIDFilePathAndVersion(ctx context.Context, promptID int64, filePath, version string) (*PromptVersion, error) {
	var promptVersion PromptVersion
	err := s.db.Operator.Core.NewSelect().Model(&promptVersion).
		Where("prompt_id = ? AND file_path = ? AND version = ?", promptID, filePath, version).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get prompt version: %w", errorx.HandleDBError(err, errorx.Ctx().Set("prompt_id", promptID).Set("file_path", filePath).Set("version", version)))
	}
	return &promptVersion, nil
}

func (s *promptVersionStoreImpl) UpdateHash(ctx context.Context, id int64, hash string) (*PromptVersion, error) {
	promptVersion := PromptVersion{ID: id, Hash: hash}
	res, err := s.db.Core.NewUpdate().Model(&promptVersion).Column("hash", "updated_at").WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("update prompt version hash: %w", errorx.HandleDBError(err, errorx.Ctx().Set("id", id)))
	}
	return &promptVersion, nil
}
