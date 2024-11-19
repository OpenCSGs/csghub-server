package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/checker"
)

type repoComponentImpl struct {
	checker sensitive.SensitiveChecker
	rs      database.RepoStore
	rfs     database.RepoFileStore
	rfcs    database.RepoFileCheckStore
	git     gitserver.GitServer
}

type RepoComponent interface {
	UpdateRepoSensitiveCheckStatus(ctx context.Context, repoType types.RepositoryType, namespace string, name string, status types.SensitiveCheckStatus) error
	CheckRepoFiles(ctx context.Context, repoType types.RepositoryType, namespace string, name string, options CheckOption) error
	CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error)
}

func NewRepoComponent(cfg *config.Config) (RepoComponent, error) {
	c := &repoComponentImpl{checker: sensitive.NewAliyunGreenChecker(cfg)}
	gs, err := git.NewGitServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server for sensitive component: %w", err)
	}
	c.rs = database.NewRepoStore()
	c.rfs = database.NewRepoFileStore()
	c.rfcs = database.NewRepoFileCheckStore()
	c.git = gs

	return c, nil
}

type CheckOption struct {
	ForceCheck bool
	// NotCheckFileExts []string
	// ImageFileExts    []string
	LastRepoFileID int64
	BatchSize      int64
	// MaxConcurrent  int
}

func (c *repoComponentImpl) UpdateRepoSensitiveCheckStatus(ctx context.Context, repoType types.RepositoryType, namespace string, name string, status types.SensitiveCheckStatus) error {
	repo, err := c.rs.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to get repo, error: %w", err)
	}

	repo.SensitiveCheckStatus = status
	_, err = c.rs.UpdateRepo(ctx, *repo)
	return err
}

func (c *repoComponentImpl) CheckRepoFiles(ctx context.Context, repoType types.RepositoryType, namespace string, name string, options CheckOption) error {
	if options.BatchSize == 0 {
		options.BatchSize = 10
	}
	// get repo id
	repo, err := c.rs.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to get repo, error: %w", err)
	}
	repoID := repo.ID
	for {
		var files []*database.RepositoryFile
		var err error
		if options.ForceCheck {
			files, err = c.rfs.BatchGet(ctx, repoID, options.LastRepoFileID, options.BatchSize)
		} else {
			files, err = c.rfs.BatchGetUnchcked(ctx, repoID, options.LastRepoFileID, options.BatchSize)
		}

		if err != nil {
			return fmt.Errorf("failed to get repo files,repoID: %d, lastRepoFileID: %d, error: %w", repoID, options.LastRepoFileID, err)
		}

		for _, file := range files {
			c.processFile(ctx, file)
		}

		count := len(files)
		if count < int(options.BatchSize) {
			break
		}

		options.LastRepoFileID = files[count-1].ID
	}

	return nil
}

func (c *repoComponentImpl) processFile(ctx context.Context, file *database.RepositoryFile) {
	reader := NewRepoFileContentReader(file, c.git)
	checker := checker.GetFileChecker(file.FileType, file.Path, file.LfsRelativePath)
	status, msg := checker.Run(reader)
	if status == types.SensitiveCheckException {
		slog.Error("failed to check repo file content", slog.Int64("repo_id", file.RepositoryID), slog.Int64("repo_file_id", file.ID), slog.String("file", file.Path),
			slog.String("err", msg))
	}

	err := c.saveCheckResult(ctx, file, status, msg)
	if err != nil {
		slog.Error("save check result failed", "error", err)
	}
}

func (c *repoComponentImpl) saveCheckResult(ctx context.Context, file *database.RepositoryFile, status types.SensitiveCheckStatus, msg string) error {
	fcr := &database.RepositoryFileCheck{
		RepoFileID: file.ID,
		Status:     status,
		Message:    msg,
		CreatedAt:  time.Time{},
	}

	fail := status == types.SensitiveCheckFail
	if fail {
		// TODO: public a sensitive check status event to message queue
		// set repository to private and set sensitive_check_status to fail
		file.Repository.Private = true
		file.Repository.SensitiveCheckStatus = types.SensitiveCheckFail
		if _, err := c.rs.UpdateRepo(ctx, *file.Repository); err != nil {
			slog.Error("failed to update repo sensitive check status", slog.Any("repository_id", file.Repository.ID),
				slog.Int64("repository_file_id", file.ID), slog.String("path", file.Path),
				slog.Any("error", err))
		}
		slog.Info("detect sensitive file content, set repository to private",
			slog.String("path", file.Path), slog.Int64("repository_file_id", file.ID),
			slog.Any("repository_id", file.Repository.ID))
	}

	// create repository file check record
	err := c.rfcs.Upsert(ctx, *fcr)
	if err != nil {
		slog.Error("failed to create or update repository file check record", slog.Any("error", err),
			slog.String("path", file.Path), slog.Int64("repo_file_id", file.ID))
	}
	return err
}

func (cc *repoComponentImpl) CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error) {
	fields := req.GetSensitiveFields()
	for _, field := range fields {
		pass, err := cc.checker.PassTextCheck(ctx, sensitive.Scenario(field.Scenario), field.Value())
		if err != nil {
			slog.Error("fail to check request sensitivity", slog.String("field", field.Name), slog.Any("error", err))
			return false, fmt.Errorf("fail to check '%s' sensitivity, error: %w", field.Name, err)
		}
		if pass.IsSensitive {
			slog.Error("found sensitive words in request", slog.String("field", field.Name))
			return false, errors.New("found sensitive words in field: " + field.Name)
		}
	}
	return true, nil
}
