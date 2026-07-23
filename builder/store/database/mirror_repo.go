package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// MirrorRepoStore creates a mirror repository in one database transaction.
type MirrorRepoStore interface {
	// CreateMirrorRepoRecords creates mirror repository records in one transaction.
	CreateMirrorRepoRecords(ctx context.Context, input CreateMirrorRepoRecordsInput) (*Mirror, error)
}

// MirrorJobClient inserts mirror repository jobs in the same database transaction.
type MirrorJobClient interface {
	// InsertMirrorRepoJobTx inserts one mirror repository job in the provided transaction.
	InsertMirrorRepoJobTx(ctx context.Context, tx *sql.Tx, input MirrorJobInput) (int64, error)
}

// MirrorJobInput describes the mirror repository job queued after mirror rows are created.
type MirrorJobInput struct {
	// MirrorID identifies the mirror record that owns the sync target.
	MirrorID int64
	// RepositoryID identifies the local repository being synchronized.
	RepositoryID int64
	// MirrorTaskID identifies the queued mirror task that tracks this job.
	MirrorTaskID int64
	// RepoType identifies the local repository type.
	RepoType types.RepositoryType
	// SourceURL is the original upstream Git URL for the current sync job.
	SourceURL string
	// RepoPath is the local repository path, such as "namespace/name".
	RepoPath string
	// Priority controls mirror job scheduling order.
	Priority types.MirrorPriority
	// Urgent routes the job to the urgent repository queue.
	Urgent bool
}

// CreateMirrorRepoRecordsInput contains the database rows needed to create a mirror repo.
type CreateMirrorRepoRecordsInput struct {
	// Repository is inserted when CreateRepository is true; otherwise it is the existing target repo.
	Repository *Repository
	// CreateRepository controls whether repository and type-specific rows are inserted.
	CreateRepository bool

	// MCPServer is inserted when creating an MCP server repository.
	MCPServer *MCPServer
	// MCPServerProperties are inserted after the MCP server row is created.
	MCPServerProperties []MCPServerProperty
	Mirror              Mirror
	// Urgent routes the initial repository job to the urgent queue.
	Urgent bool
}

type mirrorRepoStoreImpl struct {
	db        *DB
	jobClient MirrorJobClient
}

// NewMirrorRepoStore creates a MirrorRepoStore using the default database.
func NewMirrorRepoStore(jobClient MirrorJobClient) MirrorRepoStore {
	return &mirrorRepoStoreImpl{db: defaultDB, jobClient: jobClient}
}

// NewMirrorRepoStoreWithDB creates a MirrorRepoStore using the provided database.
func NewMirrorRepoStoreWithDB(db *DB, jobClient MirrorJobClient) MirrorRepoStore {
	return &mirrorRepoStoreImpl{db: db, jobClient: jobClient}
}

// CreateMirrorRepoRecords creates all business-visible mirror repository rows atomically.
func (s *mirrorRepoStoreImpl) CreateMirrorRepoRecords(ctx context.Context, input CreateMirrorRepoRecordsInput) (*Mirror, error) {
	if input.Repository == nil {
		return nil, fmt.Errorf("repository is required")
	}
	if input.CreateRepository && input.Repository.ID != 0 {
		return nil, fmt.Errorf("repository must not already be created")
	}
	if !input.CreateRepository && input.Repository.ID == 0 {
		return nil, fmt.Errorf("repository must already exist")
	}

	mirror := input.Mirror
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if input.CreateRepository {
			createdRepo, err := createRepository(ctx, tx, *input.Repository)
			if err != nil {
				return err
			}
			input.Repository = &createdRepo

			if err := createRecomRepoScore(ctx, tx, input.Repository.ID); err != nil {
				return err
			}

			if err := s.createTypedRepo(ctx, tx, input); err != nil {
				return err
			}
		} else {
			if err := lockRepoWithoutMirror(ctx, tx, input.Repository.ID); err != nil {
				return err
			}
			if err := updateRepoSyncStatus(ctx, tx, input.Repository.ID, types.SyncStatusPending); err != nil {
				return fmt.Errorf("failed to update repository sync status: %w", err)
			}
			input.Repository.SyncStatus = types.SyncStatusPending
			if err := updateRepoSourceFields(ctx, tx, *input.Repository); err != nil {
				return fmt.Errorf("failed to update repository source path: %w", err)
			}
		}

		mirror.RepositoryID = input.Repository.ID
		mirror.Repository = input.Repository
		mirror.Status = types.MirrorQueued
		if _, err := tx.NewInsert().Model(&mirror).Exec(ctx, &mirror); err != nil {
			return fmt.Errorf("failed to create mirror: %w", err)
		}

		task, err := createMirrorTask(ctx, tx, mirror, input.Urgent)
		if err != nil {
			return err
		}
		mirror.CurrentTaskID = task.ID
		if err := updateMirrorCurrentTask(ctx, tx, mirror.ID, task.ID); err != nil {
			return err
		}

		if s.jobClient != nil {
			repoJobID, err := s.jobClient.InsertMirrorRepoJobTx(ctx, tx.Tx, MirrorJobInput{
				MirrorID:     mirror.ID,
				RepositoryID: input.Repository.ID,
				MirrorTaskID: task.ID,
				RepoType:     input.Repository.RepositoryType,
				SourceURL:    mirror.SourceUrl,
				RepoPath:     input.Repository.Path,
				Priority:     mirror.Priority,
				Urgent:       input.Urgent,
			})
			if err != nil {
				return fmt.Errorf("failed to insert mirror repo job: %w", err)
			}
			task.RepoJobID = repoJobID
			if _, err := tx.NewUpdate().
				Model(&task).
				Column("repo_job_id").
				WherePK().
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to update mirror repo job id: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return &mirror, nil
}

// lockRepoWithoutMirror prevents adding a second mirror to an existing repository.
func lockRepoWithoutMirror(ctx context.Context, tx bun.Tx, repoID int64) error {
	repo := Repository{ID: repoID}
	if err := tx.NewSelect().Model(&repo).WherePK().For("UPDATE").Scan(ctx); err != nil {
		return fmt.Errorf("failed to lock repository: %w", err)
	}
	exists, err := tx.NewSelect().
		Model((*Mirror)(nil)).
		Where("repository_id = ?", repoID).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check existing mirror: %w", err)
	}
	if exists {
		return fmt.Errorf("repository already has mirror")
	}
	return nil
}

// createRepository inserts the base repository row using the same defaults as RepoStore.CreateRepo.
func createRepository(ctx context.Context, tx bun.Tx, input Repository) (Repository, error) {
	input.Migrated = true
	input.Hashed = true
	input.SyncStatus = types.SyncStatusPending
	res, err := tx.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return Repository{}, fmt.Errorf("failed to create repository: %w", err)
	}
	return input, nil
}

// createMirrorTask inserts the initial queued task for a newly created mirror.
func createMirrorTask(ctx context.Context, tx bun.Tx, mirror Mirror, urgent bool) (MirrorTask, error) {
	task := MirrorTask{
		MirrorID: mirror.ID,
		Priority: mirror.Priority,
		Status:   types.MirrorQueued,
		IsUrgent: urgent,
	}
	if err := tx.NewInsert().Model(&task).Scan(ctx, &task); err != nil {
		return MirrorTask{}, fmt.Errorf("failed to create mirror task: %w", err)
	}
	return task, nil
}

// updateMirrorCurrentTask records the task that currently drives the mirror state.
func updateMirrorCurrentTask(ctx context.Context, tx bun.Tx, mirrorID, taskID int64) error {
	mirror := Mirror{ID: mirrorID, CurrentTaskID: taskID}
	if _, err := tx.NewUpdate().Model(&mirror).WherePK().Column("current_task_id").Exec(ctx); err != nil {
		return fmt.Errorf("failed to update mirror current task: %w", err)
	}
	return nil
}

// createRecomRepoScore inserts the initial total recommendation score for the repository.
func createRecomRepoScore(ctx context.Context, tx bun.Tx, repoID int64) error {
	score := RecomRepoScore{
		RepositoryID: repoID,
		WeightName:   RecomWeightTotal,
		Score:        0,
	}
	res, err := tx.NewInsert().Model(&score).Exec(ctx, &score)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("failed to create repository recommendation score: %w", err)
	}
	return nil
}

// createTypedRepo inserts the repo-type-specific row for the already-addressed repository.
func (s *mirrorRepoStoreImpl) createTypedRepo(ctx context.Context, tx bun.Tx, input CreateMirrorRepoRecordsInput) error {
	switch input.Repository.RepositoryType {
	case types.ModelRepo:
		model := Model{RepositoryID: input.Repository.ID, Repository: input.Repository}
		if _, err := tx.NewInsert().Model(&model).Exec(ctx, &model); err != nil {
			return fmt.Errorf("failed to create model: %w", err)
		}
	case types.DatasetRepo:
		dataset := Dataset{RepositoryID: input.Repository.ID, Repository: input.Repository}
		if _, err := tx.NewInsert().Model(&dataset).Exec(ctx, &dataset); err != nil {
			return fmt.Errorf("failed to create dataset: %w", err)
		}
	case types.CodeRepo:
		code := Code{RepositoryID: input.Repository.ID, Repository: input.Repository, LastUpdatedAt: time.Now()}
		if _, err := tx.NewInsert().Model(&code).Exec(ctx, &code); err != nil {
			return fmt.Errorf("failed to create code: %w", err)
		}
	case types.MCPServerRepo:
		if err := s.createMCPServer(ctx, tx, input.Repository, input.MCPServer, input.MCPServerProperties); err != nil {
			return err
		}
	case types.SkillRepo:
		skill := Skill{RepositoryID: input.Repository.ID, Repository: input.Repository}
		if _, err := tx.NewInsert().Model(&skill).Exec(ctx, &skill); err != nil {
			return fmt.Errorf("failed to create skill: %w", err)
		}
	default:
		return fmt.Errorf("unsupported repository type: %s", input.Repository.RepositoryType)
	}

	return nil
}

// createMCPServer inserts the MCP server and its tool properties.
func (s *mirrorRepoStoreImpl) createMCPServer(ctx context.Context, tx bun.Tx, repo *Repository, mcpServer *MCPServer, properties []MCPServerProperty) error {
	if mcpServer == nil {
		mcpServer = &MCPServer{}
	}
	mcpServer.RepositoryID = repo.ID
	mcpServer.Repository = repo
	if _, err := tx.NewInsert().Model(mcpServer).Exec(ctx, mcpServer); err != nil {
		return fmt.Errorf("failed to create mcp server: %w", err)
	}

	for _, property := range properties {
		mcpServerProperty := property
		mcpServerProperty.MCPServerID = mcpServer.ID
		mcpServerProperty.MCPServer = mcpServer
		if _, err := tx.NewInsert().Model(&mcpServerProperty).Exec(ctx, &mcpServerProperty); err != nil {
			return fmt.Errorf("failed to add property to mcp server: %w", err)
		}
	}
	return nil
}

// updateRepoSourceFields stores already-resolved upstream source paths for existing repositories.
func updateRepoSourceFields(ctx context.Context, tx bun.Tx, repo Repository) error {
	query := tx.NewUpdate().
		Model(&Repository{}).
		Where("id = ?", repo.ID)

	hasSourceField := false
	if repo.CSGPath != "" {
		query.Set("csg_path = ?", repo.CSGPath)
		hasSourceField = true
	}
	if repo.HFPath != "" {
		query.Set("hf_path = ?", repo.HFPath)
		hasSourceField = true
	}
	if repo.MSPath != "" {
		query.Set("ms_path = ?", repo.MSPath)
		hasSourceField = true
	}
	if repo.GithubPath != "" {
		query.Set("github_path = ?", repo.GithubPath)
		hasSourceField = true
	}
	if !hasSourceField {
		return nil
	}

	_, err := query.Exec(ctx)
	return err
}

// updateRepoSyncStatus stores the repository sync state inside the mirror transaction.
func updateRepoSyncStatus(ctx context.Context, tx bun.Tx, repoID int64, status types.RepositorySyncStatus) error {
	_, err := tx.NewUpdate().
		Model(&Repository{}).
		Set("sync_status = ?", status).
		Where("id = ?", repoID).
		Exec(ctx)
	return err
}
