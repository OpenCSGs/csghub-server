package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type Space struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",notnull" json:"repository_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	// gradio, streamlit, docker etc
	Sdk           string `bun:",notnull" json:"sdk"`
	SdkVersion    string `bun:",notnull" json:"sdk_version"`
	DriverVersion string `bun:",notnull" json:"driver_version"`
	// PythonVersion string `bun:",notnull" json:"python_version"`
	Template      string `bun:",notnull" json:"template"`
	CoverImageUrl string `bun:"" json:"cover_image_url"`
	Env           string `bun:",notnull" json:"env"`
	Hardware      string `bun:",notnull" json:"hardware"`
	Secrets       string `bun:",notnull" json:"secrets"`
	HasAppFile    bool   `bun:"," json:"has_app_file"`
	SKU           string `bun:"," json:"sku"`
	OrderDetailID int64  `json:"order_detail_id"`
	Variables     string `bun:",nullzero" json:"variables"`
	ClusterID     string `bun:",nullzero" json:"cluster_id"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, Space{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Space{})
	})
}
