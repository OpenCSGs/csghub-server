package component

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type repoFileComponentImpl struct {
	rfs database.RepoFileStore
	rs  database.RepoStore
	gs  gitserver.GitServer
}

type RepoFileComponent interface {
	GenRepoFileRecords(ctx context.Context, repoType types.RepositoryType, namespace, name string) error
	GenRepoFileRecordsBatch(ctx context.Context, repoType types.RepositoryType, lastRepoID int64, concurrency int) error
	DetectRepoSensitiveCheckStatus(ctx context.Context, repoType types.RepositoryType, namespace, name string) error
}

func NewRepoFileComponent(conf *config.Config) (RepoFileComponent, error) {
	c := &repoFileComponentImpl{
		rfs: database.NewRepoFileStore(),
		rs:  database.NewRepoStore(),
	}
	gs, err := git.NewGitServer(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server, error: %w", err)
	}

	c.gs = gs
	return c, nil
}
func (c *repoFileComponentImpl) GenRepoFileRecords(ctx context.Context, repoType types.RepositoryType, namespace, name string) error {
	repo, err := c.rs.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	return c.createRepoFileRecords(ctx, *repo)
}

func (c *repoFileComponentImpl) GenRepoFileRecordsBatch(ctx context.Context, repoType types.RepositoryType, lastRepoID int64, concurrency int) error {
	tokens := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		tokens <- struct{}{}
	}
	wg := &sync.WaitGroup{}
	//TODO: load last repo id from redis cache
	batch := 10
	for {
		repos, err := c.rs.BatchGet(ctx, repoType, lastRepoID, batch)
		if err != nil {
			return fmt.Errorf("failed to get repos in batch, error: %w", err)
		}
		for _, repo := range repos {
			//wait
			<-tokens
			wg.Add(1)
			go func(repo database.Repository) {
				slog.Info("start to get files of repository", slog.Any("repoType", repoType), slog.String("path", repo.Path))
				//get file paths of repo
				err := c.createRepoFileRecords(ctx, repo)
				if err != nil {
					slog.Error("fail to get all files of repository",
						slog.String("path", repo.Path), slog.String("repo_type", string(repo.RepositoryType)),
						slog.String("error", err.Error()))
				}
				tokens <- struct{}{}
				wg.Done()
			}(repo)

		}

		if len(repos) < batch {
			break
		}
		lastRepoID = repos[len(repos)-1].ID
	}

	wg.Wait()
	return nil
}

func (c *repoFileComponentImpl) createRepoFileRecords(ctx context.Context, repo database.Repository) error {
	namespace, name := repo.NamespaceAndName()
	var (
		files  []*types.File
		cursor string
	)

	for {
		resp, err := c.gs.GetTree(ctx, types.GetTreeRequest{
			Namespace: namespace,
			Name:      name,
			RepoType:  repo.RepositoryType,
			Ref:       repo.DefaultBranch,
			Recursive: true,
			Limit:     types.MaxFileTreeSize,
			Cursor:    cursor,
		})
		if resp == nil {
			break
		}

		cursor = resp.Cursor
		if err != nil {
			return fmt.Errorf("failed to get repo %s/%s/%s file tree,%w", repo.RepositoryType, namespace, name, err)
		}

		for _, file := range resp.Files {
			if file.Type == "dir" {
				continue
			}
			files = append(files, file)
		}

		if resp.Cursor == "" {
			break
		}
	}
	//get all files
	for _, file := range files {
		// save repo files into db
		rf := database.RepositoryFile{
			RepositoryID:    repo.ID,
			Path:            file.Path,
			FileType:        file.Type,
			Size:            file.Size,
			CommitSha:       file.SHA,
			LfsRelativePath: file.LfsRelativePath,
			Branch:          repo.DefaultBranch,
		}

		var exists bool
		var err error
		if exists, err = c.rfs.Exists(ctx, rf); err != nil {
			slog.Error("failed to check repository file exists", slog.Any("repo_id", repo.ID),
				slog.String("file_path", rf.Path), slog.String("error", err.Error()))
			continue
		}

		if exists {
			slog.Info("skip create exist repository file", slog.Any("repo_id", repo.ID), slog.String("file_path", rf.Path))
			continue
		}
		if err := c.rfs.Create(ctx, &rf); err != nil {
			slog.Error("failed to save repository file", slog.Any("repo_id", repo.ID),
				slog.String("error", err.Error()))
			return fmt.Errorf("failed to save repository file, error: %w", err)
		}
	}
	return nil
}

func (c *repoFileComponentImpl) DetectRepoSensitiveCheckStatus(ctx context.Context, repoType types.RepositoryType, namespace, name string) error {
	repo, err := c.rs.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	//TODO:handler other branches
	branch := repo.DefaultBranch

	status := types.SensitiveCheckFail
	exists, err := c.rfs.ExistsSensitiveCheckRecord(ctx, repo.ID, branch, types.SensitiveCheckFail)
	if err != nil {
		return fmt.Errorf("failed to check repo file sensitive check record exists, error: %w", err)
	}
	if exists {
		repo.SensitiveCheckStatus = status
		_, err = c.rs.UpdateRepo(ctx, *repo)
		return err
	}

	status = types.SensitiveCheckException
	exists, err = c.rfs.ExistsSensitiveCheckRecord(ctx, repo.ID, branch, types.SensitiveCheckException)
	if err != nil {
		return fmt.Errorf("failed to check repo file sensitive check record exists, error: %w", err)
	}
	if exists {
		repo.SensitiveCheckStatus = status
		_, err = c.rs.UpdateRepo(ctx, *repo)
		return err
	}

	repo.SensitiveCheckStatus = types.SensitiveCheckPass
	_, err = c.rs.UpdateRepo(ctx, *repo)
	return err
}
