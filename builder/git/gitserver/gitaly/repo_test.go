package gitaly

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	gitalypb_mock "opencsg.com/csghub-server/_mocks/gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestGitalyRepo_CloneProjectStorage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.TODO()
		fromMock := gitalypb_mock.NewMockRepositoryServiceClient(t)
		toMock := gitalypb_mock.NewMockRepositoryServiceClient(t)
		helper := &CloneStorageHelper{
			from: fromMock,
			to:   toMock,
		}
		req := &ProjectStorageCloneRequest{
			CurrentGitalyAddress: "gca",
			CurrentGitalyToken:   "gct",
			CurrentGitalyStorage: "gcs",
			NewGitalyAddress:     "nca",
			NewGitalyToken:       "nct",
			NewGitalyStorage:     "ncs",
			FilesServer:          "http://foo.com/",
		}
		fromMock.EXPECT().RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "gcs",
				RelativePath: "foo",
			}}).Return(&gitalypb.RepositoryExistsResponse{Exists: true}, nil)
		toMock.EXPECT().RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "ncs",
				RelativePath: "foo",
			}}).Return(&gitalypb.RepositoryExistsResponse{Exists: false}, nil)
		fromMock.EXPECT().GetSnapshot(ctx, &gitalypb.GetSnapshotRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "gcs",
				RelativePath: "foo",
			},
		}).Return(&MockGrpcStreamClient[*gitalypb.GetSnapshotResponse]{
			data: []*gitalypb.GetSnapshotResponse{
				{Data: []byte("foobar")},
			},
		}, nil)
		toMock.EXPECT().CreateRepositoryFromSnapshot(ctx, &gitalypb.CreateRepositoryFromSnapshotRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "ncs",
				RelativePath: "foo",
			},
			HttpUrl: req.FilesServer + "foo.tar",
		}).Return(&gitalypb.CreateRepositoryFromSnapshotResponse{}, nil)
		err := helper.CloneRepoStorage(ctx, "foo", req)
		require.NoError(t, err)
	})
	t.Run("exists", func(t *testing.T) {
		ctx := context.TODO()
		fromMock := gitalypb_mock.NewMockRepositoryServiceClient(t)
		toMock := gitalypb_mock.NewMockRepositoryServiceClient(t)
		helper := &CloneStorageHelper{
			from: fromMock,
			to:   toMock,
		}
		req := &ProjectStorageCloneRequest{
			CurrentGitalyAddress: "gca",
			CurrentGitalyToken:   "gct",
			CurrentGitalyStorage: "gcs",
			NewGitalyAddress:     "nca",
			NewGitalyToken:       "nct",
			NewGitalyStorage:     "ncs",
			FilesServer:          "http://foo.com/",
		}
		fromMock.EXPECT().RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "gcs",
				RelativePath: "foo",
			}}).Return(&gitalypb.RepositoryExistsResponse{Exists: true}, nil)
		toMock.EXPECT().RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "ncs",
				RelativePath: "foo",
			}}).Return(&gitalypb.RepositoryExistsResponse{Exists: true}, nil)
		err := helper.CloneRepoStorage(ctx, "foo", req)
		require.NoError(t, err)
	})
}

func TestGitalyRepo_GetArchiveUsesRelativePrefix(t *testing.T) {
	tester := newGitalyTester(t)
	ctx := context.TODO()

	// Create a valid zip with entries under the "baidu-search/" prefix
	prefixedZip := makeTestZipWithPrefix(t, "baidu-search", []testZipEntry{
		{name: "README.md", body: "hello"},
		{name: "SKILL.md", body: "skill content"},
	})

	tester.mocks.repoStore.EXPECT().FindByPath(mock.Anything, types.SkillRepo, "zhzhang", "baidu-search").Return(&database.Repository{
		ID:     1,
		Hashed: false,
	}, nil)
	tester.mocks.repoClient.EXPECT().GetArchive(mock.Anything, &gitalypb.GetArchiveRequest{
		Repository: &gitalypb.Repository{
			StorageName:  "st",
			RelativePath: "skills_zhzhang/baidu-search.git",
		},
		CommitId:        "commit-id",
		Prefix:          "baidu-search",
		Format:          gitalypb.GetArchiveRequest_ZIP,
		Path:            []byte("."),
		IncludeLfsBlobs: true,
	}).Return(&MockGrpcStreamClient[*gitalypb.GetArchiveResponse]{
		data: []*gitalypb.GetArchiveResponse{
			{Data: prefixedZip},
		},
	}, nil)

	content, err := tester.GetArchive(ctx, gitserver.GetArchiveReq{
		Namespace: "zhzhang",
		Name:      "baidu-search",
		Revision:  "commit-id",
		RepoType:  types.SkillRepo,
	})

	require.NoError(t, err)
	// Verify the zip has been stripped of the prefix directory
	stripped := readZipEntryNames(t, content)
	require.Equal(t, []string{"README.md", "SKILL.md"}, stripped)
}

type testZipEntry struct {
	name string
	body string
}

func makeTestZipWithPrefix(t *testing.T, prefix string, entries []testZipEntry) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		file, err := w.Create(prefix + "/" + e.name)
		require.NoError(t, err)
		_, err = file.Write([]byte(e.body))
		require.NoError(t, err)
	}
	err := w.Close()
	require.NoError(t, err)
	return buf.Bytes()
}

func readZipEntryNames(t *testing.T, data []byte) []string {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return names
}

func TestStripZipPrefix(t *testing.T) {
	t.Run("strips prefix from entries", func(t *testing.T) {
		zipData := makeTestZipWithPrefix(t, "my-skill", []testZipEntry{
			{name: "README.md", body: "hello"},
			{name: "src/main.go", body: "package main"},
		})
		stripped, err := stripZipPrefix(zipData, "my-skill")
		require.NoError(t, err)
		require.Equal(t, []string{"README.md", "src/main.go"}, readZipEntryNames(t, stripped))
	})

	t.Run("skips entries with only prefix", func(t *testing.T) {
		zipData := makeTestZipWithPrefix(t, "my-skill", []testZipEntry{
			{name: "README.md", body: "hello"},
		})
		stripped, err := stripZipPrefix(zipData, "my-skill")
		require.NoError(t, err)
		require.Equal(t, []string{"README.md"}, readZipEntryNames(t, stripped))
	})

	t.Run("passes through entries without prefix", func(t *testing.T) {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		file, err := w.Create("standalone.txt")
		require.NoError(t, err)
		_, err = file.Write([]byte("data"))
		require.NoError(t, err)
		err = w.Close()
		require.NoError(t, err)

		stripped, err := stripZipPrefix(buf.Bytes(), "other-prefix")
		require.NoError(t, err)
		// Entry without matching prefix keeps its original name
		// but leading / is stripped
		names := readZipEntryNames(t, stripped)
		require.Equal(t, []string{"standalone.txt"}, names)
	})

	t.Run("empty zip returns empty result", func(t *testing.T) {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		err := w.Close()
		require.NoError(t, err)

		stripped, err := stripZipPrefix(buf.Bytes(), "any-prefix")
		require.NoError(t, err)
		require.Empty(t, readZipEntryNames(t, stripped))
	})

	t.Run("strips absolute path prefix", func(t *testing.T) {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		file, err := w.Create("/standalone.txt")
		require.NoError(t, err)
		_, err = file.Write([]byte("data"))
		require.NoError(t, err)
		err = w.Close()
		require.NoError(t, err)

		stripped, err := stripZipPrefix(buf.Bytes(), "other-prefix")
		require.NoError(t, err)
		names := readZipEntryNames(t, stripped)
		require.Equal(t, []string{"standalone.txt"}, names)
	})
}
