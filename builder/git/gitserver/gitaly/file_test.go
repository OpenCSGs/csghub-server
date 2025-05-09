package gitaly

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	gitalypb_mock "opencsg.com/csghub-server/_mocks/gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type MockGrpcStreamClient[T any] struct {
	grpc.ClientStream
	data  []T
	index int
}

func (m *MockGrpcStreamClient[T]) Recv() (T, error) {
	var t T
	if m.index >= len(m.data) {
		return t, io.EOF
	}
	t = m.data[m.index]
	m.index += 1
	return t, nil
}

func (m *MockGrpcStreamClient[T]) CloseAndRecv() (T, error) {
	var t T
	if m.index >= len(m.data) {
		return t, io.EOF
	}
	t = m.data[m.index]
	m.index += 1
	return t, nil
}

// R request type, T response type
type MockGrpcStreamClientFull[R any, T any] struct {
	grpc.ClientStream
	data  []T
	index int
	sends []R
}

func (m *MockGrpcStreamClientFull[R, T]) Recv() (T, error) {
	var t T
	if m.index >= len(m.data) {
		return t, io.EOF
	}
	t = m.data[m.index]
	m.index += 1
	return t, nil
}

func (m *MockGrpcStreamClientFull[R, T]) Send(data R) error {
	m.sends = append(m.sends, data)
	return nil
}

func (m *MockGrpcStreamClientFull[R, T]) CloseAndRecv() (T, error) {
	var t T
	if m.index >= len(m.data) {
		return t, io.EOF
	}
	t = m.data[m.index]
	m.index += 1
	return t, nil
}

type gitalyTester struct {
	*Client
	mocks struct {
		repoClient      *gitalypb_mock.MockRepositoryServiceClient
		commitClient    *gitalypb_mock.MockCommitServiceClient
		blobClient      *gitalypb_mock.MockBlobServiceClient
		refClient       *gitalypb_mock.MockRefServiceClient
		diffClient      *gitalypb_mock.MockDiffServiceClient
		operationClient *gitalypb_mock.MockOperationServiceClient
		smartHttpClient *gitalypb_mock.MockSmartHTTPServiceClient
		remoteClient    *gitalypb_mock.MockRemoteServiceClient
	}
}

func newGitalyTester(t *testing.T) *gitalyTester {
	tester := &gitalyTester{}
	tester.mocks.repoClient = gitalypb_mock.NewMockRepositoryServiceClient(t)
	tester.mocks.commitClient = gitalypb_mock.NewMockCommitServiceClient(t)
	tester.mocks.blobClient = gitalypb_mock.NewMockBlobServiceClient(t)
	tester.mocks.refClient = gitalypb_mock.NewMockRefServiceClient(t)
	tester.mocks.diffClient = gitalypb_mock.NewMockDiffServiceClient(t)
	tester.mocks.operationClient = gitalypb_mock.NewMockOperationServiceClient(t)
	tester.mocks.smartHttpClient = gitalypb_mock.NewMockSmartHTTPServiceClient(t)
	tester.mocks.remoteClient = gitalypb_mock.NewMockRemoteServiceClient(t)
	tester.Client = &Client{
		config:          &config.Config{},
		repoClient:      tester.mocks.repoClient,
		commitClient:    tester.mocks.commitClient,
		blobClient:      tester.mocks.blobClient,
		refClient:       tester.mocks.refClient,
		diffClient:      tester.mocks.diffClient,
		operationClient: tester.mocks.operationClient,
		smartHttpClient: tester.mocks.smartHttpClient,
		remoteClient:    tester.mocks.remoteClient,
	}
	tester.config.GitalyServer.Storage = "st"
	return tester
}

func TestGitalyFile_GetRepoFileRaw(t *testing.T) {
	tester := newGitalyTester(t)
	ctx := context.TODO()

	tester.mocks.commitClient.EXPECT().TreeEntry(mock.Anything, &gitalypb.TreeEntryRequest{
		Repository: &gitalypb.Repository{
			StorageName:  "st",
			RelativePath: "s_ns/n.git",
		},
		Revision: []byte("main"),
		Path:     []byte("foo"),
	}).Return(&MockGrpcStreamClient[*gitalypb.TreeEntryResponse]{
		data: []*gitalypb.TreeEntryResponse{
			{Data: []byte("go")},
			{Data: []byte("od")},
		},
	}, nil)
	data, err := tester.GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "foo",
	})
	require.NoError(t, err)
	require.Equal(t, "good", data)
}

func TestGitalyFile_GetRepoFileReader(t *testing.T) {
	tester := newGitalyTester(t)
	ctx := context.TODO()

	mockedStream := &MockGrpcStreamClient[*gitalypb.TreeEntryResponse]{
		data: []*gitalypb.TreeEntryResponse{
			{Data: []byte("go"), Size: 2},
		},
	}
	tester.mocks.commitClient.EXPECT().TreeEntry(mock.Anything, &gitalypb.TreeEntryRequest{
		Repository: &gitalypb.Repository{
			StorageName:  "st",
			RelativePath: "s_ns/n.git",
		},
		Revision: []byte("main"),
		Path:     []byte("foo"),
	}).Return(mockedStream, nil)
	r, size, err := tester.GetRepoFileReader(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "foo",
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), size)
	p := make([]byte, 2)
	_, err = r.Read(p)
	require.NoError(t, err)
	require.Equal(t, "go", string(p))
}

func TestGitalyFile_GetRepoFileContents(t *testing.T) {
	tester := newGitalyTester(t)
	ctx := context.TODO()

	repo := &gitalypb.Repository{
		StorageName:  "st",
		RelativePath: "s_ns/n.git",
	}
	tester.mocks.commitClient.EXPECT().TreeEntry(mock.Anything, &gitalypb.TreeEntryRequest{
		Repository: repo,
		Revision:   []byte("main"),
		Path:       []byte("foo"),
	}).Return(&MockGrpcStreamClient[*gitalypb.TreeEntryResponse]{
		data: []*gitalypb.TreeEntryResponse{
			{Data: []byte("go")},
		},
	}, nil)

	tester.mocks.commitClient.EXPECT().ListLastCommitsForTree(mock.Anything, &gitalypb.ListLastCommitsForTreeRequest{
		Repository:      repo,
		Revision:        "main",
		Path:            []byte("foo"),
		Limit:           1000,
		LiteralPathspec: true,
	}).Return(&MockGrpcStreamClient[*gitalypb.ListLastCommitsForTreeResponse]{
		data: []*gitalypb.ListLastCommitsForTreeResponse{
			{Commits: []*gitalypb.ListLastCommitsForTreeResponse_CommitForTree{
				{PathBytes: []byte("foo"), Commit: &gitalypb.GitCommit{
					Author: &gitalypb.CommitAuthor{
						Name: []byte("user1"),
					},
					Committer: &gitalypb.CommitAuthor{
						Name: []byte("user2"),
					},
					Subject: []byte("foo"),
				}},
			}},
		},
	}, nil)
	tester.mocks.blobClient.EXPECT().GetBlobs(mock.Anything, &gitalypb.GetBlobsRequest{
		Repository: repo,
		RevisionPaths: []*gitalypb.GetBlobsRequest_RevisionPath{
			{Revision: "main", Path: []byte("foo")},
		},
		Limit: 1024,
	}).Return(&MockGrpcStreamClient[*gitalypb.GetBlobsResponse]{
		data: []*gitalypb.GetBlobsResponse{
			{Path: []byte("foo"), Mode: 1, Oid: "o1"},
		},
	}, nil)

	file, err := tester.GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "foo",
	})
	require.NoError(t, err)
	require.Equal(t, &types.File{
		Name: "foo",
		Type: "dir",
		Commit: types.Commit{
			CommitterName: "user2",
			AuthorName:    "user1",
			Message:       "foo",
			AuthoredDate:  "1970-01-01T00:00:00Z",
			CommitterDate: "1970-01-01T00:00:00Z",
			CreatedAt:     "1970-01-01T00:00:00Z",
		},
		Path:    "foo",
		Mode:    "1",
		SHA:     "o1",
		Content: "Z28=",
	}, file)
}

func TestGitalyFile_CreateRepoFile(t *testing.T) {
	tester := newGitalyTester(t)

	m := &MockGrpcStreamClientFull[
		*gitalypb.UserCommitFilesRequest, *gitalypb.UserCommitFilesResponse,
	]{data: []*gitalypb.UserCommitFilesResponse{{}}}
	tester.mocks.operationClient.EXPECT().UserCommitFiles(mock.Anything).Return(
		m, nil,
	)
	err := tester.CreateRepoFile(&types.CreateFileReq{
		Namespace: "ns",
		Name:      "n",
		Branch:    "main",
		Message:   "new",
		FilePath:  "foo",
		Content:   "bar",
	})
	require.NoError(t, err)
	repository := &gitalypb.Repository{
		StorageName:  "st",
		RelativePath: "s_ns/n.git",
		GlRepository: "s/ns/n",
	}

	header := &gitalypb.UserCommitFilesRequestHeader{
		Repository: repository,
		User: &gitalypb.User{
			GlId: "user-1",
			Name: []byte("n"),
		},
		BranchName:       []byte("main"),
		CommitMessage:    []byte("new"),
		CommitAuthorName: []byte("n"),
		StartRepository:  repository,
		StartBranchName:  []byte("main"),
		Timestamp:        timestamppb.New(time.Now()),
	}

	actions := []*gitalypb.UserCommitFilesRequest{
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Header{
				Header: header,
			},
		},
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
				Action: &gitalypb.UserCommitFilesAction{
					UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Header{
						Header: &gitalypb.UserCommitFilesActionHeader{
							Action:        gitalypb.UserCommitFilesActionHeader_CREATE,
							Base64Content: true,
							FilePath:      []byte("foo"),
						},
					},
				},
			},
		},
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
				Action: &gitalypb.UserCommitFilesAction{
					UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Content{
						Content: []byte("bar"),
					},
				},
			},
		},
	}

	expectedHeader, err := json.Marshal(actions[0])
	require.NoError(t, err)
	eh, err := sjson.Delete(string(expectedHeader), "UserCommitFilesRequestPayload.Header.timestamp")
	require.NoError(t, err)
	actualHeader, err := json.Marshal(m.sends[0])
	require.NoError(t, err)
	ah, err := sjson.Delete(string(actualHeader), "UserCommitFilesRequestPayload.Header.timestamp")
	require.NoError(t, err)
	require.JSONEq(t, eh, ah)
	require.Equal(t, actions[1], m.sends[1])
	require.Equal(t, actions[2], m.sends[2])
}

func TestGitalyFile_UpdateRepoFile(t *testing.T) {
	tester := newGitalyTester(t)

	m := &MockGrpcStreamClientFull[
		*gitalypb.UserCommitFilesRequest, *gitalypb.UserCommitFilesResponse,
	]{data: []*gitalypb.UserCommitFilesResponse{{}}}
	tester.mocks.operationClient.EXPECT().UserCommitFiles(mock.Anything).Return(
		m, nil,
	)
	err := tester.UpdateRepoFile(&types.UpdateFileReq{
		Namespace: "ns",
		Name:      "n",
		Branch:    "main",
		Message:   "new",
		FilePath:  "foo",
		Content:   "bar",
		Username:  "user-1",
	})
	require.NoError(t, err)
	repository := &gitalypb.Repository{
		StorageName:  "st",
		RelativePath: "s_ns/n.git",
		GlRepository: "s/ns/n",
	}

	header := &gitalypb.UserCommitFilesRequestHeader{
		Repository: repository,
		User: &gitalypb.User{
			GlId:       "user-1",
			Name:       []byte("user-1"),
			GlUsername: "user-1",
		},
		BranchName:       []byte("main"),
		CommitMessage:    []byte("new"),
		CommitAuthorName: []byte("user-1"),
		StartRepository:  repository,
		StartBranchName:  []byte("main"),
		Timestamp:        timestamppb.New(time.Now()),
	}

	actions := []*gitalypb.UserCommitFilesRequest{
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Header{
				Header: header,
			},
		},
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
				Action: &gitalypb.UserCommitFilesAction{
					UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Header{
						Header: &gitalypb.UserCommitFilesActionHeader{
							Action:        gitalypb.UserCommitFilesActionHeader_UPDATE,
							Base64Content: true,
							FilePath:      []byte("foo"),
						},
					},
				},
			},
		},
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
				Action: &gitalypb.UserCommitFilesAction{
					UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Content{
						Content: []byte("bar"),
					},
				},
			},
		},
	}

	expectedHeader, err := json.Marshal(actions[0])
	require.NoError(t, err)
	eh, err := sjson.Delete(string(expectedHeader), "UserCommitFilesRequestPayload.Header.timestamp")
	require.NoError(t, err)
	actualHeader, err := json.Marshal(m.sends[0])
	require.NoError(t, err)
	ah, err := sjson.Delete(string(actualHeader), "UserCommitFilesRequestPayload.Header.timestamp")
	require.NoError(t, err)
	require.JSONEq(t, eh, ah)
	require.Equal(t, actions[1], m.sends[1])
	require.Equal(t, actions[2], m.sends[2])
}

func TestGitalyFile_DeleteRepoFile(t *testing.T) {
	tester := newGitalyTester(t)

	m := &MockGrpcStreamClientFull[
		*gitalypb.UserCommitFilesRequest, *gitalypb.UserCommitFilesResponse,
	]{data: []*gitalypb.UserCommitFilesResponse{{}}}
	tester.mocks.operationClient.EXPECT().UserCommitFiles(mock.Anything).Return(
		m, nil,
	)
	err := tester.DeleteRepoFile(&types.DeleteFileReq{
		Namespace: "ns",
		Name:      "n",
		Branch:    "main",
		Message:   "new",
		FilePath:  "foo",
		Content:   "bar",
		Username:  "user-1",
	})
	require.NoError(t, err)
	repository := &gitalypb.Repository{
		StorageName:  "st",
		RelativePath: "s_ns/n.git",
		GlRepository: "s/ns/n",
	}

	header := &gitalypb.UserCommitFilesRequestHeader{
		Repository: repository,
		User: &gitalypb.User{
			GlId:       "user-1",
			Name:       []byte("user-1"),
			GlUsername: "user-1",
		},
		BranchName:       []byte("main"),
		CommitMessage:    []byte("new"),
		CommitAuthorName: []byte("user-1"),
		StartRepository:  repository,
		StartBranchName:  []byte("main"),
		Timestamp:        timestamppb.New(time.Now()),
	}

	actions := []*gitalypb.UserCommitFilesRequest{
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Header{
				Header: header,
			},
		},
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
				Action: &gitalypb.UserCommitFilesAction{
					UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Header{
						Header: &gitalypb.UserCommitFilesActionHeader{
							Action:        gitalypb.UserCommitFilesActionHeader_DELETE,
							Base64Content: true,
							FilePath:      []byte("foo"),
						},
					},
				},
			},
		},
	}

	expectedHeader, err := json.Marshal(actions[0])
	require.NoError(t, err)
	eh, err := sjson.Delete(string(expectedHeader), "UserCommitFilesRequestPayload.Header.timestamp")
	require.NoError(t, err)
	actualHeader, err := json.Marshal(m.sends[0])
	require.NoError(t, err)
	ah, err := sjson.Delete(string(actualHeader), "UserCommitFilesRequestPayload.Header.timestamp")
	require.NoError(t, err)
	require.JSONEq(t, eh, ah)
	require.Equal(t, actions[1], m.sends[1])
}

func TestGitalyFile_GetRepoFileTree(t *testing.T) {
	tester := newGitalyTester(t)
	ctx := context.TODO()

	repo := &gitalypb.Repository{
		StorageName:  "st",
		RelativePath: "s_ns/n.git",
	}

	tester.mocks.commitClient.EXPECT().ListLastCommitsForTree(mock.Anything, &gitalypb.ListLastCommitsForTreeRequest{
		Repository:      repo,
		Revision:        "main",
		Path:            []byte("foo/"),
		Limit:           1000,
		LiteralPathspec: true,
	}).Return(&MockGrpcStreamClient[*gitalypb.ListLastCommitsForTreeResponse]{
		data: []*gitalypb.ListLastCommitsForTreeResponse{
			{Commits: []*gitalypb.ListLastCommitsForTreeResponse_CommitForTree{
				{PathBytes: []byte("foo"), Commit: &gitalypb.GitCommit{
					Author: &gitalypb.CommitAuthor{
						Name: []byte("user1"),
					},
					Committer: &gitalypb.CommitAuthor{
						Name: []byte("user2"),
					},
					Subject: []byte("foo"),
				}},
			}},
		},
	}, nil)
	tester.mocks.blobClient.EXPECT().GetBlobs(mock.Anything, &gitalypb.GetBlobsRequest{
		Repository: repo,
		RevisionPaths: []*gitalypb.GetBlobsRequest_RevisionPath{
			{Revision: "main", Path: []byte("foo")},
		},
		Limit: 1024,
	}).Return(&MockGrpcStreamClient[*gitalypb.GetBlobsResponse]{
		data: []*gitalypb.GetBlobsResponse{
			{Path: []byte("foo"), Mode: 1, Oid: "o1"},
		},
	}, nil)

	files, err := tester.GetRepoFileTree(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "foo",
	})
	require.NoError(t, err)
	require.Equal(t, []*types.File{{
		Name: "foo",
		Type: "dir",
		Commit: types.Commit{
			CommitterName: "user2",
			AuthorName:    "user1",
			Message:       "foo",
			AuthoredDate:  "1970-01-01T00:00:00Z",
			CommitterDate: "1970-01-01T00:00:00Z",
			CreatedAt:     "1970-01-01T00:00:00Z",
		},
		Path:    "foo",
		Mode:    "1",
		SHA:     "o1",
		Content: "",
	}}, files)
}

func TestGitalyFile_GetTree(t *testing.T) {

	cases := []struct {
		path       string
		gitalyPath string
	}{
		{path: "", gitalyPath: "."},
		{path: "foo", gitalyPath: "foo"},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {

			tester := newGitalyTester(t)
			ctx := context.TODO()
			prefix := ""
			if c.path != "" {
				prefix = c.path + "/"
			}

			repo := &gitalypb.Repository{
				StorageName:  "st",
				RelativePath: "s_ns/n.git",
			}
			tester.mocks.commitClient.EXPECT().GetTreeEntries(mock.Anything, &gitalypb.GetTreeEntriesRequest{
				Repository: repo,
				Sort:       gitalypb.GetTreeEntriesRequest_TREES_FIRST,
				Revision:   []byte("main"),
				Path:       []byte(c.gitalyPath),
				PaginationParams: &gitalypb.PaginationParameter{
					PageToken: "c",
					Limit:     int32(500),
				},
			}).Return(&MockGrpcStreamClient[*gitalypb.GetTreeEntriesResponse]{
				data: []*gitalypb.GetTreeEntriesResponse{
					{Entries: []*gitalypb.TreeEntry{
						{Path: []byte(prefix + "a")},
						{Path: []byte(prefix + "b")},
						{Path: []byte(prefix + "c")},
					}, PaginationCursor: &gitalypb.PaginationCursor{NextCursor: "nc"}},
				},
			}, nil)
			tester.mocks.blobClient.EXPECT().GetBlobs(mock.Anything, &gitalypb.GetBlobsRequest{
				Repository: repo,
				RevisionPaths: []*gitalypb.GetBlobsRequest_RevisionPath{
					{Revision: "main", Path: []byte(prefix + "a")},
					{Revision: "main", Path: []byte(prefix + "b")},
					{Revision: "main", Path: []byte(prefix + "c")},
				},
				Limit: 0,
			}).Return(&MockGrpcStreamClient[*gitalypb.GetBlobsResponse]{
				data: []*gitalypb.GetBlobsResponse{
					{Path: []byte(prefix + "a"), Mode: 1, Oid: "o1"},
					{Path: []byte(prefix + "b"), Mode: 1, Oid: "o2"},
					{Path: []byte(prefix + "c"), Mode: 1, Oid: "o1"},
				},
			}, nil)

			pointer := `version https://git-lfs.github.com/spec/v1
oid sha256:a4f0e7e96b4f6af4a1b597c2fc4a42ec9a997c64ab7da96760c40582a0ac27a5
size 507607173`

			tester.mocks.blobClient.EXPECT().GetLFSPointers(
				mock.Anything, &gitalypb.GetLFSPointersRequest{
					BlobIds:    []string{"o1", "o2"},
					Repository: repo,
				}).Return(&MockGrpcStreamClient[*gitalypb.GetLFSPointersResponse]{
				data: []*gitalypb.GetLFSPointersResponse{
					{LfsPointers: []*gitalypb.LFSPointer{
						{
							Size: 1234, Oid: "o1", FileOid: []byte("o11"),
							Data: []byte(pointer),
						},
					}},
				},
			}, nil)

			tree, err := tester.GetTree(ctx, types.GetTreeRequest{
				Namespace: "ns",
				Name:      "n",
				Ref:       "main",
				Path:      c.path,
				Limit:     500,
				Cursor:    "c",
			})
			require.NoError(t, err)
			require.Equal(t, []*types.File{
				{Name: "a", Path: prefix + "a", Type: "dir", Mode: "1",
					SHA: "a4f0e7e96b4f6af4a1b597c2fc4a42ec9a997c64ab7da96760c40582a0ac27a5",
					Lfs: true, Size: 507607173,
					LfsRelativePath: "a4/f0/e7e96b4f6af4a1b597c2fc4a42ec9a997c64ab7da96760c40582a0ac27a5",
					LfsPointerSize:  1234,
				},
				{Name: "b", Path: prefix + "b", Type: "dir", Mode: "1", SHA: "o2"},
				{Name: "c", Path: prefix + "c", Type: "dir", Mode: "1",
					SHA: "a4f0e7e96b4f6af4a1b597c2fc4a42ec9a997c64ab7da96760c40582a0ac27a5",
					Lfs: true, Size: 507607173,
					LfsRelativePath: "a4/f0/e7e96b4f6af4a1b597c2fc4a42ec9a997c64ab7da96760c40582a0ac27a5",
					LfsPointerSize:  1234,
				},
			}, tree.Files)
			require.Equal(t, "nc", tree.Cursor)
		})
	}
}

func TestGitalyFile_GetLogsTree(t *testing.T) {
	cases := []struct {
		path       string
		gitalyPath string
	}{
		{path: "", gitalyPath: "/"},
		{path: "foo", gitalyPath: "foo/"},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {

			tester := newGitalyTester(t)
			ctx := context.TODO()
			prefix := ""
			if c.path != "" {
				prefix = c.path + "/"
			}

			repo := &gitalypb.Repository{
				StorageName:  "st",
				RelativePath: "s_ns/n.git",
			}
			tester.mocks.commitClient.EXPECT().ListLastCommitsForTree(mock.Anything, &gitalypb.ListLastCommitsForTreeRequest{
				Repository: repo,
				Revision:   "main",
				Path:       []byte(c.gitalyPath),
				Offset:     50,
				Limit:      10,
			}).Return(&MockGrpcStreamClient[*gitalypb.ListLastCommitsForTreeResponse]{
				data: []*gitalypb.ListLastCommitsForTreeResponse{
					{Commits: []*gitalypb.ListLastCommitsForTreeResponse_CommitForTree{
						{
							PathBytes: []byte(prefix + "a"),
							Commit: &gitalypb.GitCommit{
								Subject: []byte("foo"),
								Id:      "c",
								Committer: &gitalypb.CommitAuthor{
									Name: []byte("u"),
								},
								Author: &gitalypb.CommitAuthor{
									Name: []byte("v"),
								},
							},
						},
					}},
				},
			}, nil)

			tree, err := tester.GetLogsTree(ctx, types.GetLogsTreeRequest{
				Namespace: "ns",
				Name:      "n",
				Ref:       "main",
				Path:      c.path,
				Limit:     10,
				Offset:    50,
			})
			require.NoError(t, err)
			require.Equal(t, []*types.CommitForTree{
				{
					ID:   "c",
					Name: "a", Path: prefix + "a",
					CommitterName: "u",
					AuthorName:    "v",
					Message:       "foo",
					CommitterDate: "1970-01-01T00:00:00Z",
					AuthoredDate:  "1970-01-01T00:00:00Z",
					CreatedAt:     "1970-01-01T00:00:00Z",
				},
			}, tree.Commits)
		})
	}

}

func TestGitalyFile_GetRepoAllFiles(t *testing.T) {
	tester := newGitalyTester(t)
	ctx := context.TODO()

	tester.mocks.commitClient.EXPECT().ListFiles(mock.Anything, &gitalypb.ListFilesRequest{
		Repository: &gitalypb.Repository{
			StorageName:  "st",
			RelativePath: "models_ns/n.git",
		},
		Revision: []byte("main"),
	}).Return(&MockGrpcStreamClient[*gitalypb.ListFilesResponse]{
		data: []*gitalypb.ListFilesResponse{
			{Paths: [][]byte{[]byte("foo/a")}},
		},
	}, nil)
	data, err := tester.GetRepoAllFiles(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		RepoType:  types.ModelRepo,
	})
	require.NoError(t, err)
	require.Equal(t, []*types.File{
		{Name: "a", Path: "foo/a"},
	}, data)
}

func TestGitalyFile_GetRepoAllLfsPointers(t *testing.T) {
	tester := newGitalyTester(t)
	ctx := context.TODO()

	tester.mocks.blobClient.EXPECT().ListAllLFSPointers(mock.Anything, &gitalypb.ListAllLFSPointersRequest{
		Repository: &gitalypb.Repository{
			StorageName:  "st",
			RelativePath: "models_ns/n.git",
		},
	}).Return(&MockGrpcStreamClient[*gitalypb.ListAllLFSPointersResponse]{
		data: []*gitalypb.ListAllLFSPointersResponse{
			{LfsPointers: []*gitalypb.LFSPointer{
				{Oid: "o1", Size: 5, Data: []byte("go")},
			}},
		},
	}, nil)
	data, err := tester.GetRepoAllLfsPointers(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		RepoType:  types.ModelRepo,
	})
	require.NoError(t, err)
	require.Equal(t, []*types.LFSPointer{
		{Oid: "o1", Size: 5, Data: "go"},
	}, data)
}
