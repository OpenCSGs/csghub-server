package gitaly

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	gitalyclient "gitlab.com/gitlab-org/gitaly/v16/client"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

const (
	// The context timeout is 5 second,
	// 200 should be a reasonable number. Can cnage there value
	// based on gitaly performance.
	SHOW_COMMIT_FILE_COUNT_LIMIT = 200
	GET_REPO_FILE_TREE_TIMEOUT   = 5 * time.Second
)

// This method returns three parameters instead of two, as in v1.
// The first and last parameters are the same as in v1, while the middle parameter (withCommits bool) is new.
// If the input path contains many files, the commits will be skipped and not included
// in the response files; in this case, withCommits will be false.
func (c *Client) GetRepoFileTreeV2(ctx context.Context, req gitserver.GetRepoInfoByPathReq) ([]*types.File, bool, error) {

	withCommit := false
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storge,
		RelativePath: "dronescapes/.git",
	}
	if !req.File {
		req.Path = req.Path + "/"
	}

	if req.Ref == "" {
		req.Ref = "main"
	}

	if req.Path == "/" {
		req.Path = "."
	}

	// Retrieve the last commit for the tree.
	// This commit will serve as the snapshot, as we will be calling the Gitaly API
	// three times. The repository might change during these calls, so use a real commit
	// should be better than a reference name like "main".
	resp, err := c.commitClient.LastCommitForPath(ctx, &gitalypb.LastCommitForPathRequest{
		Repository: repository,
		Revision:   []byte(req.Ref),
	})
	if err != nil {
		return nil, withCommit, err
	}
	req.Ref = resp.Commit.Id

	var files []*types.File
	ctx, cancel := context.WithTimeout(ctx, GET_REPO_FILE_TREE_TIMEOUT)
	defer cancel()

	// get all files first
	entryStream, err := c.commitClient.GetTreeEntries(ctx, &gitalypb.GetTreeEntriesRequest{
		Repository: repository,
		Revision:   []byte(req.Ref),
		Path:       []byte(req.Path),
		PaginationParams: &gitalypb.PaginationParameter{
			Limit: 1000,
		},
		Sort: gitalypb.GetTreeEntriesRequest_TREES_FIRST,
	})
	if err != nil {
		return nil, withCommit, err
	}
	pathFileMap := map[string]*types.File{}
	var revisionPaths []*gitalypb.GetBlobsRequest_RevisionPath
	for {
		resp, err := entryStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
		}
		if len(resp.Entries) > 0 {
			for _, entry := range resp.Entries {
				var fileType string
				if entry.Type == gitalypb.TreeEntry_BLOB {
					fileType = "file"
				} else {
					fileType = "dir"
				}
				file := &types.File{
					Name: filepath.Base(string(entry.Path)),
					Type: fileType,
					Path: string(entry.Path),
					Mode: strconv.Itoa(int(entry.Mode)),
				}
				files = append(files, file)
				pathFileMap[file.Path] = file
				revisionPaths = append(revisionPaths, &gitalypb.GetBlobsRequest_RevisionPath{
					Revision: req.Ref,
					Path:     entry.Path,
				})
			}

		}
	}

	if len(files) <= SHOW_COMMIT_FILE_COUNT_LIMIT {

		req := &gitalypb.ListLastCommitsForTreeRequest{
			Repository: repository,
			Revision:   req.Ref,
			Path:       []byte(req.Path),
			Limit:      1000,
		}

		commitStream, err := c.commitClient.ListLastCommitsForTree(ctx, req)
		if err != nil {
			return nil, withCommit, err
		}
		for {
			commitResp, err := commitStream.Recv()
			if err != nil {
				if err == io.EOF {
					// only set withCommit to true when the request is fully done.
					withCommit = true
					break
				}
			}
			if commitResp == nil {
				return nil, withCommit, errors.New("bad request")
			}
			commits := commitResp.Commits
			if len(commits) > 0 {
				for _, r := range commits {
					f, ok := pathFileMap[string(r.PathBytes)]
					if ok {
						commit := r.Commit
						f.Commit = types.Commit{
							ID:             commit.Id,
							CommitterName:  string(commit.Committer.Name),
							CommitterEmail: string(commit.Committer.Email),
							CommitterDate:  commit.Committer.Date.AsTime().Format(time.RFC3339),
							CreatedAt:      commit.Committer.Date.AsTime().Format(time.RFC3339),
							Message:        string(commit.Subject),
							AuthorName:     string(commit.Author.Name),
							AuthorEmail:    string(commit.Author.Email),
							AuthoredDate:   commit.Author.Date.AsTime().Format(time.RFC3339),
						}
						f.LastCommitSHA = commit.Id
					}
				}
			}
		}
	}

	// Get blobs with file size
	listBlobsReq := &gitalypb.GetBlobsRequest{
		Repository:    repository,
		RevisionPaths: revisionPaths,
		Limit:         1024,
	}

	listBlobsStream, err := c.blobClient.GetBlobs(ctx, listBlobsReq)
	if err != nil {
		return nil, withCommit, err
	}
	for {
		listBlobResp, err := listBlobsStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, withCommit, err
		}
		if listBlobResp != nil {
			var (
				fileSize        int64
				isLfs           bool
				lfsPointerSize  int
				LfsRelativePath string
			)
			fileSize = listBlobResp.Size
			if listBlobResp.Size <= 1024 {
				p, _ := ReadPointerFromBuffer(listBlobResp.Data)
				if p.Valid() {
					fileSize = p.Size
					isLfs = true
					LfsRelativePath = p.RelativePath()
					lfsPointerSize = int(listBlobResp.Size)
				}
			}

			f, ok := pathFileMap[string(listBlobResp.Path)]
			if ok {

				f.Size = fileSize
				f.Lfs = isLfs
				f.LfsPointerSize = lfsPointerSize
				f.LfsRelativePath = LfsRelativePath
				f.SHA = listBlobResp.Oid
			}
		}
	}

	return files, withCommit, nil
}

func newTestClient() (*Client, error) {
	var sidechannelRegistry *gitalyclient.SidechannelRegistry
	accessLogger := log.New()
	accessLogger.SetLevel(log.InfoLevel)
	sidechannelRegistry = gitalyclient.NewSidechannelRegistry(log.NewEntry(accessLogger))
	connOpts := append(gitalyclient.DefaultDialOpts,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		gitalyclient.WithGitalyDNSResolver(gitalyclient.DefaultDNSResolverBuilderConfig()),
	)

	conn, connErr := gitalyclient.DialSidechannel(context.Background(), "tcp://127.0.0.1:3297", sidechannelRegistry, connOpts)
	repoClient := gitalypb.NewRepositoryServiceClient(conn)
	commitClient := gitalypb.NewCommitServiceClient(conn)
	blobClient := gitalypb.NewBlobServiceClient(conn)
	refClient := gitalypb.NewRefServiceClient(conn)
	diffClient := gitalypb.NewDiffServiceClient(conn)
	operationClient := gitalypb.NewOperationServiceClient(conn)
	smartHttpClient := gitalypb.NewSmartHTTPServiceClient(conn)
	remoteClient := gitalypb.NewRemoteServiceClient(conn)

	if connErr != nil {
		return nil, connErr
	}

	config := &config.Config{}
	config.GitalyServer.Storge = "default"

	return &Client{
		config:              config,
		sidechannelRegistry: sidechannelRegistry,
		repoClient:          repoClient,
		commitClient:        commitClient,
		blobClient:          blobClient,
		refClient:           refClient,
		diffClient:          diffClient,
		operationClient:     operationClient,
		smartHttpClient:     smartHttpClient,
		remoteClient:        remoteClient,
	}, nil
}

// Please note that this is not a true unit test; its sole purpose is to demonstrate and test the current draft PR locally.
// The test repository is: https://opencsg.com/datasets/AIWizards/dronescapes/files/main/
// This test should be replaced with a complete unit test for production code.
func TestFileTree(t *testing.T) {
	client, err := newTestClient()
	if err != nil {
		t.Fatal(err)
	}

	// small dir, v1 and v2 should be same
	for _, ref := range []string{"main", "450f959b4e29efaad2d9e0ef90330dd80201f8bb"} {
		for _, path := range []string{"", "dronescapes_reader"} {

			t.Run(ref+":"+path, func(t *testing.T) {
				v1Files, err := client.GetRepoFileTree(context.TODO(), gitserver.GetRepoInfoByPathReq{
					Namespace: "",
					Name:      "dronescapes",
					Ref:       ref,
					Path:      path,
				})
				if err != nil {
					t.Fatal(err)
				}

				v2Files, withCommits, err := client.GetRepoFileTreeV2(context.TODO(), gitserver.GetRepoInfoByPathReq{
					Namespace: "",
					Name:      "dronescapes",
					Ref:       ref,
					Path:      path,
				})
				if err != nil {
					t.Fatal(err)
				}
				if !withCommits {
					t.Fatal("with commits is false")
				}
				if len(v1Files) != len(v2Files) {
					t.Fatal("file count not equal")
				}

				for i := 0; i < len(v1Files); i++ {
					if *v1Files[i] != *v2Files[i] {
						fmt.Println(v1Files[i])
						fmt.Println(v2Files[i])
						t.Fatal("file not equal")
					}
				}

			})

		}
	}

	// large dir, v1 err and v2 no commits info
	t.Run("large", func(t *testing.T) {
		_, err := client.GetRepoFileTree(context.TODO(), gitserver.GetRepoInfoByPathReq{
			Namespace: "",
			Name:      "dronescapes",
			Ref:       "main",
			Path:      "data/semisupervised_set/depth_dpt/part0",
		})
		if err == nil {
			t.Fatal("v1 should return error")
		}

		v2Files, withCommits, err := client.GetRepoFileTreeV2(context.TODO(), gitserver.GetRepoInfoByPathReq{
			Namespace: "",
			Name:      "dronescapes",
			Ref:       "main",
			Path:      "data/semisupervised_set/depth_dpt/part0",
		})
		if withCommits {
			t.Fatal("commits should be skipped for large dir")
		}

		if len(v2Files) != 1000 {
			t.Fatal("should return 1000 files")
		}

		for _, f := range v2Files {
			if f.Commit.ID != "" {
				t.Fatal("v2 file commit should be empty")
			}
		}
	})
}
