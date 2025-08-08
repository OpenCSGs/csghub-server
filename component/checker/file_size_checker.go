package checker

import (
	"context"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type FileSizeChecker struct {
	repoStore database.RepoStore
	gitServer gitserver.GitServer
	config    *config.Config
}

func NewFileSizeChecker(config *config.Config) (GitCallbackChecker, error) {
	git, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server: %w", err)
	}
	return &FileSizeChecker{
		repoStore: database.NewRepoStore(),
		gitServer: git,
		config:    config,
	}, nil

}

func (c *FileSizeChecker) Check(ctx context.Context, req types.GitalyAllowedReq) (bool, error) {
	if !c.config.Git.CheckFileSizeEnabled {
		return true, nil
	}
	var (
		filePath     string
		fileOversize bool
		revisions    []string
	)

	repoType, namespace, name := req.GetRepoTypeNamespaceAndName()

	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, err: %v", err)
	}
	if repo == nil {
		return false, errors.New("repo not found")
	}

	revisions = []string{"--not", "--all", "--not", req.GetRevision()}

	files, err := c.gitServer.GetRepoFiles(ctx, gitserver.GetRepoFilesReq{
		Namespace:                             namespace,
		Name:                                  name,
		GitObjectDirectoryRelative:            req.GitEnv.GitObjectDirectoryRelative,
		GitAlternateObjectDirectoriesRelative: req.GitEnv.GitAlternateObjectDirectoriesRelative,
		RepoType:                              repoType,
		Revisions:                             revisions,
	})
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if file.Size > c.config.Git.MaxUnLfsFileSize {
			filePath = file.Path
			fileOversize = true
			break
		}
	}

	if fileOversize {
		return false, fmt.Errorf("file %s is too large, max size is %d bytes", filePath, c.config.Git.MaxUnLfsFileSize)
	}

	return true, nil
}
