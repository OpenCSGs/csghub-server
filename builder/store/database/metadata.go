package database

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type metadataStoreImpl struct {
	db *DB
}

type MetadataStore interface {
	FindByRepoID(ctx context.Context, repoID int64) (*Metadata, error)
	Upsert(ctx context.Context, metadata *Metadata) error
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
	ID                int64                `bun:",pk,autoincrement" json:"id"`
	RepositoryID      int64                `bun:",notnull,unique" json:"repository_id"`
	Repository        *Repository          `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	ModelParams       float32              `bun:"," json:"model_params"`
	TensorType        string               `bun:"," json:"tensor_type"`
	MiniGPUMemoryGB   float32              `bun:"," json:"mini_gpu_memory_gb"`
	MiniGPUFinetuneGB float32              `bun:"," json:"mini_gpu_finetune_gb"`
	Architecture      string               `bun:"," json:"architecture"`
	ModelType         string               `bun:"," json:"model_type"`
	ClassName         string               `bun:"," json:"class_name"`
	Quantizations     []types.Quantization `bun:"type:jsonb" json:"quantizations,omitempty"`
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
