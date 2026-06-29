package database

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/types"
)

type metadataStoreImpl struct {
	db *DB
}

type MetadataStore interface {
	FindByRepoID(ctx context.Context, repoID int64) (*Metadata, error)
	Upsert(ctx context.Context, metadata *Metadata) error
	// UpdatePDRecommendation sets the PD recommendation for a repository's metadata.
	// When allowOverwrite is false (auto-scan from config.json), it only updates if
	// the existing PDRecommendation is nil/empty, preserving manually adjusted values.
	// When allowOverwrite is true (loaded from local config files), it overwrites
	// the existing value to ensure file-based configs take precedence.
	UpdatePDRecommendation(ctx context.Context, repoID int64, rec *types.PDRecommendation, allowOverwrite bool) error
	// UpdateModelArchType updates the model architecture type (dense/moe/hybrid)
	// for a repository's metadata.
	UpdateModelArchType(ctx context.Context, repoID int64, archType types.ModelArchType) error
}

func NewMetadataStore() MetadataStore {
	return &metadataStoreImpl{
		db: defaultDB,
	}
}

func NewMetadataStoreWithDB(db *DB) MetadataStore {
	return &metadataStoreImpl{
		db: db,
	}
}

type Metadata struct {
	ID                int64                  `bun:",pk,autoincrement" json:"id"`
	RepositoryID      int64                  `bun:",notnull,unique" json:"repository_id"`
	Repository        *Repository            `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	ModelParams       float32                `bun:"," json:"model_params"`
	TensorType        string                 `bun:"," json:"tensor_type"`
	MiniGPUMemoryGB   float32                `bun:"," json:"mini_gpu_memory_gb"`
	MiniGPUFinetuneGB float32                `bun:"," json:"mini_gpu_finetune_gb"`
	Architecture      string                 `bun:"," json:"architecture"`
	ModelType         string                 `bun:"," json:"model_type"`
	ClassName         string                 `bun:"," json:"class_name"`
	Quantizations     []types.Quantization   `bun:"type:jsonb" json:"quantizations,omitempty"`
	ModelArchType     types.ModelArchType    `bun:"," json:"model_arch_type"`
	PDRecommendation  *types.PDRecommendation `bun:"type:jsonb,nullzero" json:"pd_recommendation,omitempty"`
	times
}

func (m *metadataStoreImpl) FindByRepoID(ctx context.Context, repoID int64) (*Metadata, error) {
	var metadata Metadata
	err := m.db.Operator.Core.NewSelect().
		Model(&metadata).
		Relation("Repository").
		Where("repository_id=?", repoID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}
func (m *metadataStoreImpl) Upsert(ctx context.Context, metadata *Metadata) error {
	_, err := m.db.Operator.Core.NewInsert().
		Model(metadata).
		On("CONFLICT (repository_id) DO UPDATE").
		Exec(ctx)
	return err
}

// UpdatePDRecommendation updates the pd_recommendation column. When allowOverwrite
// is false (auto-scan from config.json), it only updates if the existing value is
// nil/empty, preserving manually adjusted values. When allowOverwrite is true
// (loaded from local config files), it overwrites unconditionally.
func (m *metadataStoreImpl) UpdatePDRecommendation(ctx context.Context, repoID int64, rec *types.PDRecommendation, allowOverwrite bool) error {
	if !allowOverwrite {
		// Check if the existing recommendation is already set
		existing, err := m.FindByRepoID(ctx, repoID)
		if err != nil {
			return fmt.Errorf("fail to find metadata for pd recommendation update, %w", err)
		}
		if existing != nil && !existing.PDRecommendation.IsEmpty() {
			// Already set, skip to preserve manual adjustments
			return nil
		}
	}

	_, err := m.db.Operator.Core.NewUpdate().
		Model((*Metadata)(nil)).
		Set("pd_recommendation = ?", rec).
		Where("repository_id = ?", repoID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("fail to update pd recommendation, %w", err)
	}
	return nil
}

// UpdateModelArchType updates the model architecture type for a repository's metadata.
func (m *metadataStoreImpl) UpdateModelArchType(ctx context.Context, repoID int64, archType types.ModelArchType) error {
	_, err := m.db.Operator.Core.NewUpdate().
		Model((*Metadata)(nil)).
		Set("model_arch_type = ?", archType).
		Where("repository_id = ?", repoID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("fail to update model arch type, %w", err)
	}
	return nil
}
