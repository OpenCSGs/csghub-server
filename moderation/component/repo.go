package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/checker"
	wfCommon "opencsg.com/csghub-server/moderation/workflow/common"
)

type repoComponentImpl struct {
	checker          sensitive.SensitiveChecker
	rs               database.RepoStore
	rfs              database.RepoFileStore
	rfcs             database.RepoFileCheckStore
	whitelistRule    database.RepositoryFileCheckRuleStore
	git              gitserver.GitServer
	concurrencyLimit int
	config           *config.Config
}

func NewRepoComponent(cfg *config.Config) (RepoComponent, error) {
	c := &repoComponentImpl{checker: sensitive.NewChainChecker(cfg,
		sensitive.WithACAutomaton(sensitive.LoadFromConfig(cfg)),
		sensitive.WithMutableACAutomaton(sensitive.LoadFromDB()),
		sensitive.WithAliYunChecker(),
	)}
	gs, err := git.NewGitServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server for sensitive component: %w", err)
	}
	c.rs = database.NewRepoStore()
	c.rfs = database.NewRepoFileStore()
	c.rfcs = database.NewRepoFileCheckStore()
	c.whitelistRule = database.NewRepositoryFileCheckRuleStore()
	c.git = gs
	c.concurrencyLimit = cfg.Moderation.RepoFileCheckConcurrency
	c.config = cfg
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

type RepoFullCheckRequest struct {
	Namespace string
	Name      string
	RepoType  types.RepositoryType
}

func (c *repoComponentImpl) GetRepo(ctx context.Context, repoType types.RepositoryType, namespace string, name string) (*database.Repository, error) {
	return c.rs.FindByPath(ctx, repoType, namespace, name)
}

func (c *repoComponentImpl) UpdateRepoSensitiveCheckStatus(ctx context.Context, repoId int64, status types.SensitiveCheckStatus) error {
	return c.rs.UpdateRepoSensitiveCheckStatus(ctx, repoId, status)
}

func (c *repoComponentImpl) SkipSensitiveCheckForWhiteList(ctx context.Context, req RepoFullCheckRequest) (bool, error) {
	exists, err := c.whitelistRule.Exists(ctx, database.RuleTypeNamespace, req.Namespace)
	if err != nil {
		return false, fmt.Errorf("failed to check namespace in white list: %w", err)
	}

	if exists {
		repo, err := c.GetRepo(ctx, req.RepoType, req.Namespace, req.Name)
		if err != nil {
			return false, fmt.Errorf("failed to get repo for skip sensitive check, namespace: %s, name: %s, error: %w", req.Namespace, req.Name, err)
		}
		err = c.UpdateRepoSensitiveCheckStatus(ctx, repo.ID, types.SensitiveCheckSkip)
		if err != nil {
			return false, fmt.Errorf("failed to update repo sensitive check status to skip, repo_id: %d, error: %w", repo.ID, err)
		}
		slog.InfoContext(ctx, "namespace in white list, skip repo full check", slog.String("namespace", req.Namespace),
			slog.String("name", req.Name),
			slog.Int64("repo_id", repo.ID))
		return true, nil
	}
	return false, nil
}

func (c *repoComponentImpl) RepoFullCheck(ctx context.Context, req RepoFullCheckRequest) (*types.RepoFullCheckResult, error) {
	skipped, err := c.SkipSensitiveCheckForWhiteList(ctx, req)
	if err != nil {
		return nil, err
	}
	if skipped {
		return &types.RepoFullCheckResult{
			Skipped: true,
		}, nil
	}

	workflowClient := temporal.GetClient()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: wfCommon.RepoFullCheckQueue,
	}

	we, err := workflowClient.ExecuteWorkflow(context.Background(), workflowOptions, wfCommon.RepoFullCheckWorkflow,
		wfCommon.Repo{
			Namespace: req.Namespace,
			Name:      req.Name,
			RepoType:  req.RepoType,
		}, c.config)
	if err != nil {
		return nil, fmt.Errorf("failed to start repo full check workflow, error: %w", err)
	}

	WorkflowID := we.GetID()
	slog.InfoContext(ctx, "start repo full check workflow",
		slog.String("namespace", req.Namespace),
		slog.String("name", req.Name),
		slog.String("workflow_id", WorkflowID))
	return &types.RepoFullCheckResult{
		Skipped:    false,
		WorkflowID: WorkflowID,
	}, nil
}

func (c *repoComponentImpl) CheckRepoFiles(ctx context.Context, repoID int64, options CheckOption) error {
	if options.BatchSize == 0 {
		options.BatchSize = 10
	}
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

		// process files in parallel
		var wg sync.WaitGroup
		// Limit concurrency to avoid overwhelming system resources.
		guard := make(chan struct{}, c.concurrencyLimit)

		for _, file := range files {
			wg.Add(1)
			guard <- struct{}{} // will block if guard channel is full
			go func(f *database.RepositoryFile) {
				defer wg.Done()
				c.processFile(ctx, f)
				<-guard // release a spot
			}(file)
		}
		wg.Wait()

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
	status, msg := checker.Run(ctx, reader)
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
		pass, err := cc.checker.PassTextCheck(ctx, field.Scenario, field.Value())
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

func (c *repoComponentImpl) GetNamespaceWhiteList(ctx context.Context) ([]string, error) {
	namespaceWhiteList, err := c.whitelistRule.ListByRuleType(ctx, database.RuleTypeNamespace)
	if err != nil {
		return nil, err
	}
	patterns := make([]string, len(namespaceWhiteList))
	for i := range len(namespaceWhiteList) {
		patterns[i] = namespaceWhiteList[i].Pattern
	}
	return patterns, nil
}
