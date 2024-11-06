package component

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
)

type RepoFileContentReader struct {
	file        *database.RepositoryFile
	git         gitserver.GitServer
	innerReader io.ReadCloser
	once        *sync.Once
}

var _ io.ReadCloser = (*RepoFileContentReader)(nil)

func NewRepoFileContentReader(file *database.RepositoryFile, git gitserver.GitServer) *RepoFileContentReader {
	return &RepoFileContentReader{
		file: file,
		git:  git,
		once: &sync.Once{},
	}
}

func (c *RepoFileContentReader) Read(p []byte) (n int, err error) {
	c.lazyInit()

	if c.innerReader == nil {
		return 0, errors.New("failed to read file content as git file reader not initialized")
	}
	return c.innerReader.Read(p)
}

func (c *RepoFileContentReader) Close() error {
	if c.innerReader == nil {
		return errors.New("failed to close reader as git file reader not initialized")
	}
	return c.innerReader.Close()
}

func (c *RepoFileContentReader) lazyInit() {
	c.once.Do(func() {
		namespace, name := c.file.Repository.NamespaceAndName()
		req := gitserver.GetRepoInfoByPathReq{
			Namespace: namespace,
			Name:      name,
			Path:      c.file.Path,
			RepoType:  c.file.Repository.RepositoryType,
			Ref:       c.file.Repository.DefaultBranch,
		}

		ctx := context.Background()
		var err error
		c.innerReader, _, err = c.git.GetRepoFileReader(ctx, req)
		if err != nil {
			slog.Error("failed to create git file reader", slog.Any("error", err), slog.String("path", c.file.Path), slog.Int64("repository_file_id", c.file.ID))
		}
	})
}
