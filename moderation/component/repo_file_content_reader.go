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
	initCtx     context.Context
	file        *database.RepositoryFile
	git         gitserver.GitServer
	innerReader io.ReadCloser
	once        *sync.Once
	closeOnce   *sync.Once
	closeErr    error
}

var _ io.ReadSeekCloser = (*RepoFileContentReader)(nil)

func NewRepoFileContentReader(ctx context.Context, file *database.RepositoryFile, git gitserver.GitServer) *RepoFileContentReader {
	if ctx == nil {
		ctx = context.Background()
	}
	return &RepoFileContentReader{
		initCtx:   ctx,
		file:      file,
		git:       git,
		once:      &sync.Once{},
		closeOnce: &sync.Once{},
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
	c.closeOnce.Do(func() {
		if c.innerReader != nil {
			c.closeErr = c.innerReader.Close()
		}
	})
	return c.closeErr
}

// Seek implements io.ReadSeeker. Only Seek(0, io.SeekStart) is supported — it
// closes the current underlying stream and resets the reader so that the next
// Read re-opens the file from the git server. This allows retry logic (e.g. in
// ImageFileChecker.checkByStream) to rewind and re-read the full content after
// a transient failure. Any other offset returns an error.
func (c *RepoFileContentReader) Seek(offset int64, whence int) (int64, error) {
	if offset != 0 || whence != io.SeekStart {
		return 0, errors.New("only Seek(0, io.SeekStart) is supported")
	}

	// Close the current inner reader if it was opened. A close failure (e.g.
	// RST, broken pipe) can signal a git-server issue relevant to diagnosing
	// 401 or transient failures, so log it instead of silently discarding.
	if c.innerReader != nil {
		if err := c.innerReader.Close(); err != nil {
			slog.ErrorContext(context.Background(), "failed to close inner reader during seek",
				slog.Any("error", err), slog.String("path", c.file.Path))
		}
		c.innerReader = nil
	}

	// Reset so lazyInit re-opens the stream on the next Read.
	c.once = &sync.Once{}
	c.closeOnce = &sync.Once{}
	c.closeErr = nil
	return 0, nil
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
		var err error
		c.innerReader, _, err = c.git.GetRepoFileReader(c.initCtx, req)
		if err != nil {
			slog.ErrorContext(c.initCtx, "failed to create git file reader", slog.Any("error", err), slog.String("path", c.file.Path), slog.Int64("repository_file_id", c.file.ID))
		}
	})
}
