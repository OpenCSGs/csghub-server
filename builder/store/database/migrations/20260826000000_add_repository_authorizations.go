package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// RepositoryAuthorization represents a direct repository authorization row used by the migration.
type RepositoryAuthorization struct {
	ID           int64  `bun:",pk,autoincrement"`
	RepositoryID int64  `bun:",notnull"`
	SubjectType  string `bun:",notnull"`
	SubjectID    int64  `bun:",notnull"`
	SubjectUUID  string `bun:",notnull"`
	Role         string `bun:",notnull"`
	CreateUserID int64  `bun:",notnull"`
	times
}

// init registers the repository inheritance and direct authorization schema migration.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `
				ALTER TABLE repositories
				ADD COLUMN IF NOT EXISTS org_inherit_blocked BOOLEAN NOT NULL DEFAULT FALSE
			`); err != nil {
				return fmt.Errorf("add repositories.org_inherit_blocked: %w", err)
			}

			if _, err := tx.NewCreateTable().Model(&RepositoryAuthorization{}).IfNotExists().Exec(ctx); err != nil {
				return fmt.Errorf("create repository_authorizations table: %w", err)
			}

			if _, err := tx.ExecContext(ctx, `
				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1
						FROM pg_constraint
						WHERE conname = 'repository_authorizations_role_check'
						  AND conrelid = 'repository_authorizations'::regclass
					) THEN
						ALTER TABLE repository_authorizations
						ADD CONSTRAINT repository_authorizations_role_check
						CHECK (role IN ('read', 'write'));
					END IF;
				END $$
			`); err != nil {
				return fmt.Errorf("add repository authorization role constraint: %w", err)
			}

			if _, err := tx.ExecContext(ctx, `
				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1
						FROM pg_constraint
						WHERE conname = 'repository_authorizations_subject_type_check'
						  AND conrelid = 'repository_authorizations'::regclass
					) THEN
						ALTER TABLE repository_authorizations
						ADD CONSTRAINT repository_authorizations_subject_type_check
						CHECK (subject_type IN ('user', 'organization'));
					END IF;
				END $$
			`); err != nil {
				return fmt.Errorf("add repository authorization subject type constraint: %w", err)
			}

			if _, err := tx.NewCreateIndex().
				Model(&RepositoryAuthorization{}).
				Index("repository_authorizations_repo_subject_idx").
				Column("repository_id", "subject_type", "subject_id").
				Unique().
				IfNotExists().
				Exec(ctx); err != nil {
				return fmt.Errorf("add repository authorization unique index: %w", err)
			}

			if _, err := tx.NewCreateIndex().
				Model(&RepositoryAuthorization{}).
				Index("repository_authorizations_subject_idx").
				Column("subject_type", "subject_id").
				IfNotExists().
				Exec(ctx); err != nil {
				return fmt.Errorf("add repository authorization subject index: %w", err)
			}

			return nil
		})
	}, func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.NewDropTable().Model(&RepositoryAuthorization{}).IfExists().Cascade().Exec(ctx); err != nil {
				return fmt.Errorf("drop repository_authorizations table: %w", err)
			}

			if _, err := tx.ExecContext(ctx, `
				ALTER TABLE repositories
				DROP COLUMN IF EXISTS org_inherit_blocked
			`); err != nil {
				return fmt.Errorf("drop repositories.org_inherit_blocked: %w", err)
			}

			return nil
		})
	})
}
