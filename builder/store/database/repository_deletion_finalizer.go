package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// RepositoryDeletionFinalizer atomically removes database resources belonging
// to a soft-deleted repository and force-deletes the repository row last.
type RepositoryDeletionFinalizer interface {
	FinalizeRepositoryDeletion(ctx context.Context, repositoryID int64) error
}

type repositoryDeletionFinalizer struct{ db *DB }

var repositoryDeletionEditionFinalizers []func(context.Context, bun.IDB, int64) error

func NewRepositoryDeletionFinalizer() RepositoryDeletionFinalizer {
	return NewRepositoryDeletionFinalizerWithDB(defaultDB)
}

func NewRepositoryDeletionFinalizerWithDB(db *DB) RepositoryDeletionFinalizer {
	return &repositoryDeletionFinalizer{db: db}
}

func (f *repositoryDeletionFinalizer) FinalizeRepositoryDeletion(ctx context.Context, repositoryID int64) error {
	if repositoryID <= 0 {
		return fmt.Errorf("repository ID must be positive")
	}
	return f.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return finalizeRepositoryDeletionTx(ctx, tx, repositoryID)
	})
}

func finalizeRepositoryDeletionTx(ctx context.Context, tx bun.IDB, repositoryID int64) error {
	// Deployments and LLM configurations have independent lifecycles. Preserve
	// their records while removing the optional link to the deleted repository.
	for _, relation := range []struct{ name, table string }{
		{"deployments", "deploys"},
		{"LLM configurations", "llm_configs"},
	} {
		if _, err := tx.NewUpdate().Table(relation.table).Set("repo_id = NULL").Where("repo_id = ?", repositoryID).Exec(ctx); err != nil {
			return fmt.Errorf("unlink %s: %w", relation.name, err)
		}
	}

	for _, relation := range []struct{ name, table, column string }{
		{"user likes", "user_likes", "repo_id"},
		{"LFS metadata objects", "lfs_meta_objects", "repository_id"},
		{"files", "files", "repository_id"},
		{"repository tags", "repository_tags", "repository_id"},
		{"repository downloads", "repository_downloads", "repository_id"},
		{"metadata", "metadata", "repository_id"},
		{"repository statistics", "repository_statistics", "repository_id"},
		{"LFS files", "lfs_files", "repository_id"},
		{"LFS locks", "lfs_locks", "repository_id"},
		{"collection repositories", "collection_repositories", "repository_id"},
		{"dataviewers", "dataviewers", "repo_id"},
		{"dataviewer jobs", "dataviewer_jobs", "repo_id"},
		{"MCP scan results", "mcp_scan_results", "repository_id"},
		{"Xnet migration tasks", "xnet_migration_tasks", "repository_id"},
		{"repository recommendation weights", "recom_op_weights", "repository_id"},
		{"repository recommendation scores", "recom_repo_scores", "repository_id"},
	} {
		if _, err := tx.NewDelete().Table(relation.table).Where(relation.column+" = ?", repositoryID).Exec(ctx); err != nil {
			return fmt.Errorf("delete %s: %w", relation.name, err)
		}
	}

	var repositoryFileIDs []int64
	if err := tx.NewSelect().Table("repository_files").Column("id").Where("repository_id = ?", repositoryID).Scan(ctx, &repositoryFileIDs); err != nil {
		return fmt.Errorf("find repository files: %w", err)
	}
	if len(repositoryFileIDs) > 0 {
		if _, err := tx.NewDelete().Table("repository_file_checks").Where("repo_file_id IN (?)", bun.In(repositoryFileIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("delete repository file checks: %w", err)
		}
		if _, err := tx.NewDelete().Table("repository_files").Where("id IN (?)", bun.In(repositoryFileIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("delete repository files: %w", err)
		}
	}

	for _, relation := range []struct{ name, table, fromColumn, toColumn string }{
		{"repository relations", "repo_relations", "from_repo_id", "to_repo_id"},
		{"model trees", "model_trees", "source_repo_id", "target_repo_id"},
	} {
		if _, err := tx.NewDelete().Table(relation.table).
			Where(relation.fromColumn+" = ?", repositoryID).WhereOr(relation.toColumn+" = ?", repositoryID).Exec(ctx); err != nil {
			return fmt.Errorf("delete %s: %w", relation.name, err)
		}
	}

	var mirrorIDs []int64
	if err := tx.NewSelect().Table("mirrors").Column("id").Where("repository_id = ?", repositoryID).Scan(ctx, &mirrorIDs); err != nil {
		return fmt.Errorf("find repository mirrors: %w", err)
	}
	if len(mirrorIDs) > 0 {
		if _, err := tx.NewDelete().Table("mirror_tasks").Where("mirror_id IN (?)", bun.In(mirrorIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("delete mirror tasks: %w", err)
		}
		if _, err := tx.NewDelete().Table("mirrors").Where("id IN (?)", bun.In(mirrorIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("delete mirrors: %w", err)
		}
	}

	for _, relation := range []struct {
		name, column string
		model        any
	}{
		{"models", "repository_id", (*Model)(nil)},
		{"datasets", "repository_id", (*Dataset)(nil)},
		{"codes", "repository_id", (*Code)(nil)},
		{"spaces", "repository_id", (*Space)(nil)},
		{"prompts", "repository_id", (*Prompt)(nil)},
		{"mcp servers", "repository_id", (*MCPServer)(nil)},
		{"skills", "repository_id", (*Skill)(nil)},
		{"repository runtime frameworks", "repo_id", (*RepositoriesRuntimeFramework)(nil)},
	} {
		if _, err := tx.NewDelete().Model(relation.model).Where(relation.column+" = ?", repositoryID).ForceDelete().Exec(ctx); err != nil {
			return fmt.Errorf("delete %s: %w", relation.name, err)
		}
	}
	for _, finalize := range repositoryDeletionEditionFinalizers {
		if err := finalize(ctx, tx, repositoryID); err != nil {
			return err
		}
	}

	// The repository row is the retry/idempotency marker and must be removed last.
	if _, err := tx.NewDelete().Model((*Repository)(nil)).Where("id = ?", repositoryID).ForceDelete().Exec(ctx); err != nil {
		return fmt.Errorf("delete repository: %w", err)
	}
	return nil
}
