package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type Metadata struct {
	ID              int64                `bun:",pk,autoincrement" json:"id"`
	RepositoryID    int64                `bun:",notnull,unique" json:"repository_id"`
	ModelParams     float32              `bun:"," json:"model_params"`
	TensorType      string               `bun:"," json:"tensor_type"`
	MiniGPUMemoryGB float32              `bun:"," json:"mini_gpu_memory_gb"`
	Architecture    string               `bun:"," json:"architecture"`
	ModelType       string               `bun:"," json:"model_type"`
	ClassName       string               `bun:"," json:"class_name"`
	Quantizations   []types.Quantization `bun:"type:jsonb" json:"quantizations,omitempty"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, Metadata{})
		if err != nil {
			return fmt.Errorf("create table Metadata fail: %w", err)
		}
		_, err = db.NewCreateIndex().
			Model((*Metadata)(nil)).
			Index("idx_metadata_repo_id_arch_model_type").
			Column("repository_id", "architecture", "class_name", "model_type").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_metadata_repo_id_arch_model_type fail: %w", err)
		}
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Metadata{})
	})
}
