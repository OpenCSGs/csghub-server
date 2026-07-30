package component_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mock_gitserver "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/moderation/component"
)

// mockReadCloser is a mock for io.ReadCloser that simulates behavior of a real file reader.
type mockReadCloser struct {
	reader     io.Reader
	mu         sync.Mutex
	closed     bool
	closeCount int
}

func newMockReadCloser(content string) *mockReadCloser {
	return &mockReadCloser{
		reader: strings.NewReader(content),
	}
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.reader.Read(p)
}

func (m *mockReadCloser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return io.ErrClosedPipe
	}
	m.closed = true
	m.closeCount++
	return nil
}

func (m *mockReadCloser) GetCloseCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeCount
}

func TestRepoFileContentReader(t *testing.T) {
	dummyRepoFile := &database.RepositoryFile{
		Repository: &database.Repository{
			Path: "test_ns/test_repo",
			Name: "test_repo",
		},
		Path: "test_file.txt",
	}
	req := gitserver.GetRepoInfoByPathReq{
		Namespace: "test_ns",
		Name:      "test_repo",
		Path:      "test_file.txt",
		RepoType:  dummyRepoFile.Repository.RepositoryType,
		Ref:       dummyRepoFile.Repository.DefaultBranch,
	}

	t.Run("Read and Close", func(t *testing.T) {
		assert := assert.New(t)
		mockCloser := newMockReadCloser("file content")
		mockGit := mock_gitserver.NewMockGitServer(t)
		mockGit.EXPECT().GetRepoFileReader(mock.Anything, req).Return(mockCloser, 0, nil).Once()
		reader := component.NewRepoFileContentReader(context.Background(), dummyRepoFile, mockGit)

		buf := make([]byte, 12)
		n, err := reader.Read(buf)
		assert.NoError(err)
		assert.Equal(12, n)
		assert.Equal("file content", string(buf))

		err = reader.Close()
		assert.NoError(err)
		assert.Equal(1, mockCloser.GetCloseCount())

		// Read after close
		n, err = reader.Read(buf)
		assert.Error(err)
		assert.ErrorIs(err, io.ErrClosedPipe)
		assert.Equal(0, n)
	})

	t.Run("Close is idempotent", func(t *testing.T) {
		assert := assert.New(t)
		mockCloser := newMockReadCloser("file content")
		mockGit := mock_gitserver.NewMockGitServer(t)
		mockGit.EXPECT().GetRepoFileReader(mock.Anything, req).Return(mockCloser, 0, nil).Once()
		reader := component.NewRepoFileContentReader(context.Background(), dummyRepoFile, mockGit)

		// Read to initialize
		_, err := io.Copy(io.Discard, reader)
		assert.NoError(err)

		// Close multiple times
		assert.NoError(reader.Close())
		assert.NoError(reader.Close())

		assert.Equal(1, mockCloser.GetCloseCount(), "Close should be called only once")
	})

	t.Run("Close is thread-safe", func(t *testing.T) {
		assert := assert.New(t)
		mockCloser := newMockReadCloser("file content")
		mockGit := mock_gitserver.NewMockGitServer(t)
		mockGit.EXPECT().GetRepoFileReader(mock.Anything, req).Return(mockCloser, 0, nil).Once()
		reader := component.NewRepoFileContentReader(context.Background(), dummyRepoFile, mockGit)

		// Read to initialize
		_, err := io.Copy(io.Discard, reader)
		assert.NoError(err)

		// Concurrently close
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				assert.NoError(reader.Close())
			}()
		}
		wg.Wait()

		assert.Equal(1, mockCloser.GetCloseCount(), "Close should be called only once in concurrent situation")
	})

	t.Run("Close before reader init returns no error", func(t *testing.T) {
		assert := assert.New(t)
		mockGit := mock_gitserver.NewMockGitServer(t)
		reader := component.NewRepoFileContentReader(context.Background(), dummyRepoFile, mockGit)

		err := reader.Close()
		assert.NoError(err)
	})

	t.Run("Initialization failure", func(t *testing.T) {
		assert := assert.New(t)
		initErr := errors.New("init failed")
		mockGit := mock_gitserver.NewMockGitServer(t)
		mockGit.EXPECT().GetRepoFileReader(mock.Anything, req).Return(nil, 0, initErr).Once()
		reader := component.NewRepoFileContentReader(context.Background(), dummyRepoFile, mockGit)

		// Read should fail
		buf := make([]byte, 10)
		n, err := reader.Read(buf)
		assert.Error(err)
		assert.Equal(0, n)
		assert.Contains(err.Error(), "not initialized")

		// Close should succeed (as in, not error) because there's nothing to close
		err = reader.Close()
		assert.NoError(err)
	})

	t.Run("Seek(0, Start) reopens stream for retry", func(t *testing.T) {
		assert := assert.New(t)
		content := "file content"

		// First GetRepoFileReader call for initial read
		mockCloser1 := newMockReadCloser(content)
		// Second GetRepoFileReader call after Seek reopens
		mockCloser2 := newMockReadCloser(content)
		mockGit := mock_gitserver.NewMockGitServer(t)
		mockGit.EXPECT().GetRepoFileReader(mock.Anything, req).Return(mockCloser1, 0, nil).Once()
		mockGit.EXPECT().GetRepoFileReader(mock.Anything, req).Return(mockCloser2, 0, nil).Once()
		reader := component.NewRepoFileContentReader(context.Background(), dummyRepoFile, mockGit)

		// First read consumes the stream
		buf, err := io.ReadAll(reader)
		assert.NoError(err)
		assert.Equal(content, string(buf))

		// Seek(0, Start) should close the old stream and reset for reopen
		pos, err := reader.Seek(0, io.SeekStart)
		assert.NoError(err)
		assert.Equal(int64(0), pos)
		// Old stream should be closed
		assert.Equal(1, mockCloser1.GetCloseCount())

		// Read again — should get the same content from the reopened stream
		buf2, err := io.ReadAll(reader)
		assert.NoError(err)
		assert.Equal(content, string(buf2))

		assert.NoError(reader.Close())
		assert.Equal(1, mockCloser2.GetCloseCount())
	})

	t.Run("Seek with unsupported offset returns error", func(t *testing.T) {
		assert := assert.New(t)
		mockGit := mock_gitserver.NewMockGitServer(t)
		reader := component.NewRepoFileContentReader(context.Background(), dummyRepoFile, mockGit)

		_, err := reader.Seek(1, io.SeekStart)
		assert.Error(err)

		_, err = reader.Seek(0, io.SeekCurrent)
		assert.Error(err)
	})

}
