package gitaly

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
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

	t.Run("all entries filtered returns error", func(t *testing.T) {
		// Create a zip where all entries are the prefix directory itself
		// (which gets stripped to empty string and skipped)
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		_, err := w.Create("my-repo/")
		require.NoError(t, err)
		err = w.Close()
		require.NoError(t, err)

		_, err = stripZipPrefix(buf.Bytes(), "my-repo")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no files matched prefix")
	})
}

func TestGitalyRepo_GetLastCommitSize(t *testing.T) {
	repoInfoReq := gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "repo",
		Ref:       "main",
		Path:      "",
		RepoType:  types.ModelRepo,
		File:      false,
	}

	t.Run("success", func(t *testing.T) {
		tester := newGitalyTester(t)
		ctx := context.TODO()

		tester.mocks.repoStore.EXPECT().FindByPath(mock.Anything, types.ModelRepo, "ns", "repo").Return(&database.Repository{
			ID:     1,
			Hashed: false,
		}, nil)

		// ListFiles returns all file paths at HEAD
		tester.mocks.commitClient.EXPECT().ListFiles(mock.Anything, &gitalypb.ListFilesRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "st",
				RelativePath: "models_ns/repo.git",
			},
			Revision: []byte("main"),
		}).Return(&MockGrpcStreamClient[*gitalypb.ListFilesResponse]{
			data: []*gitalypb.ListFilesResponse{
				{Paths: [][]byte{[]byte("README.md"), []byte("model.pth")}},
			},
		}, nil)

		// GetBlobs returns blob sizes for all files
		tester.mocks.blobClient.EXPECT().GetBlobs(mock.Anything, &gitalypb.GetBlobsRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "st",
				RelativePath: "models_ns/repo.git",
			},
			RevisionPaths: []*gitalypb.GetBlobsRequest_RevisionPath{
				{Revision: "main", Path: []byte("README.md")},
				{Revision: "main", Path: []byte("model.pth")},
			},
			Limit: 0,
		}).Return(&MockGrpcStreamClient[*gitalypb.GetBlobsResponse]{
			data: []*gitalypb.GetBlobsResponse{
				{Size: 1024, Oid: "oid1", Path: []byte("README.md"), Type: gitalypb.ObjectType_BLOB},
				{Size: 2048, Oid: "oid2", Path: []byte("model.pth"), Type: gitalypb.ObjectType_BLOB},
			},
		}, nil)

		// GetLFSPointers returns LFS file sizes
		tester.mocks.blobClient.EXPECT().GetLFSPointers(mock.Anything, &gitalypb.GetLFSPointersRequest{
			Repository: &gitalypb.Repository{
				StorageName:  "st",
				RelativePath: "models_ns/repo.git",
			},
			BlobIds: []string{"oid1", "oid2"},
		}).Return(&MockGrpcStreamClient[*gitalypb.GetLFSPointersResponse]{
			data: []*gitalypb.GetLFSPointersResponse{
				{
					LfsPointers: []*gitalypb.LFSPointer{
						{Size: 130, FileSize: 1000000, Oid: "oid2"},
					},
				},
			},
		}, nil)

		size, err := tester.GetLastCommitSize(ctx, repoInfoReq)
		require.NoError(t, err)
		// blob: 1024 + 2048 = 3072, LFS: 1000000, total: 1003072
		require.Equal(t, int64(1003072), size)
	})

	t.Run("empty repo", func(t *testing.T) {
		tester := newGitalyTester(t)
		ctx := context.TODO()

		tester.mocks.repoStore.EXPECT().FindByPath(mock.Anything, types.ModelRepo, "ns", "repo").Return(&database.Repository{
			ID:     1,
			Hashed: false,
		}, nil)

		tester.mocks.commitClient.EXPECT().ListFiles(mock.Anything, mock.Anything).Return(&MockGrpcStreamClient[*gitalypb.ListFilesResponse]{
			data: []*gitalypb.ListFilesResponse{},
		}, nil)

		size, err := tester.GetLastCommitSize(ctx, repoInfoReq)
		require.NoError(t, err)
		require.Equal(t, int64(0), size)
	})

	t.Run("ListFiles error", func(t *testing.T) {
		tester := newGitalyTester(t)
		ctx := context.TODO()

		tester.mocks.repoStore.EXPECT().FindByPath(mock.Anything, types.ModelRepo, "ns", "repo").Return(&database.Repository{
			ID:     1,
			Hashed: false,
		}, nil)

		tester.mocks.commitClient.EXPECT().ListFiles(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("list files failed"))

		_, err := tester.GetLastCommitSize(ctx, repoInfoReq)
		require.Error(t, err)
	})

	t.Run("batches GetBlobs calls for large file counts", func(t *testing.T) {
		tester := newGitalyTester(t)
		ctx := context.TODO()

		tester.mocks.repoStore.EXPECT().FindByPath(mock.Anything, types.ModelRepo, "ns", "repo").Return(&database.Repository{
			ID:     1,
			Hashed: false,
		}, nil)

		// Build 1001 paths to trigger 2 batches (batch size = 1000)
		var allPaths [][]byte
		for j := 0; j < 1001; j++ {
			allPaths = append(allPaths, []byte(fmt.Sprintf("file_%d.txt", j)))
		}
		tester.mocks.commitClient.EXPECT().ListFiles(mock.Anything, mock.Anything).Return(&MockGrpcStreamClient[*gitalypb.ListFilesResponse]{
			data: []*gitalypb.ListFilesResponse{
				{Paths: allPaths},
			},
		}, nil)

		// First batch: 1000 files
		tester.mocks.blobClient.EXPECT().GetBlobs(mock.Anything, mock.MatchedBy(func(req *gitalypb.GetBlobsRequest) bool {
			return len(req.RevisionPaths) == 1000
		})).Return(&MockGrpcStreamClient[*gitalypb.GetBlobsResponse]{
			data: []*gitalypb.GetBlobsResponse{
				{Size: 1, Oid: "oid1"},
			},
		}, nil)

		// Second batch: 1 file
		tester.mocks.blobClient.EXPECT().GetBlobs(mock.Anything, mock.MatchedBy(func(req *gitalypb.GetBlobsRequest) bool {
			return len(req.RevisionPaths) == 1
		})).Return(&MockGrpcStreamClient[*gitalypb.GetBlobsResponse]{
			data: []*gitalypb.GetBlobsResponse{
				{Size: 1, Oid: "oid2"},
			},
		}, nil)

		tester.mocks.blobClient.EXPECT().GetLFSPointers(mock.Anything, mock.Anything).Return(&MockGrpcStreamClient[*gitalypb.GetLFSPointersResponse]{
			data: []*gitalypb.GetLFSPointersResponse{},
		}, nil)

		size, err := tester.GetLastCommitSize(ctx, repoInfoReq)
		require.NoError(t, err)
		require.Equal(t, int64(2), size)
	})

	t.Run("GetBlobs error", func(t *testing.T) {
		tester := newGitalyTester(t)
		ctx := context.TODO()

		tester.mocks.repoStore.EXPECT().FindByPath(mock.Anything, types.ModelRepo, "ns", "repo").Return(&database.Repository{
			ID:     1,
			Hashed: false,
		}, nil)

		tester.mocks.commitClient.EXPECT().ListFiles(mock.Anything, mock.Anything).Return(&MockGrpcStreamClient[*gitalypb.ListFilesResponse]{
			data: []*gitalypb.ListFilesResponse{
				{Paths: [][]byte{[]byte("README.md")}},
			},
		}, nil)

		tester.mocks.blobClient.EXPECT().GetBlobs(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("get blobs failed"))

		_, err := tester.GetLastCommitSize(ctx, repoInfoReq)
		require.Error(t, err)
	})
}
