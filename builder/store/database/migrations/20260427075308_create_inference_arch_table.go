package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// InferenceArch represents the allowed inference architectures configuration for migration
type InferenceArch struct {
	ID        int       `bun:",pk,autoincrement" json:"id"`
	Patterns  string    `bun:",notnull,default:''" json:"patterns"` // Multiple regex patterns separated by newlines
	CreatedAt time.Time `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",notnull,default:current_timestamp" json:"updated_at"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] create_inference_arch_table")
		// Create inference_arch table
		_, err := db.NewCreateTable().Model((*InferenceArch)(nil)).IfNotExists().Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] create_inference_arch_table")
		// Drop inference_arch table
		_, err := db.NewDropTable().Model((*InferenceArch)(nil)).IfExists().Exec(ctx)
		return err
	})
}
