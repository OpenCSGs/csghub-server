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

	// first get the last commit for tree,
	// this will be the snapshot because we will call gitaly API
	// 3 times and repo might change duraing 3 calls if use
	// something like main as the ref.
	resp, err := c.commitClient.LastCommitForPath(ctx, &gitalypb.LastCommitForPathRequest{
		Repository: repository,
		Revision:   []byte(req.Ref),
	})
	if err != nil {
		return nil, withCommit, err
	}
	req.Ref = resp.Commit.Id

	var files []*types.File
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	// get all files first
	entryStream, err := c.commitClient.GetTreeEntries(ctx, &gitalypb.GetTreeEntriesRequest{
		Repository: repository,
		Revision:   []byte(req.Ref),
		Path:       []byte(req.Path),
		PaginationParams: &gitalypb.PaginationParameter{
			Limit: 1000,
		},
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

	if len(files) <= 1000 {
		// Get last commit for tree. Even though this is a
		// streaming request, gitaly actually first retrive all commits
		// then start sending, which means when first item received from stram,
		// all data are already prepared.
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
		wc := 0
		for {
			commitResp, err := commitStream.Recv()
			if err != nil {
				if err == io.EOF {
					withCommit = true
					fmt.Println("==== total wc", wc)
					break
				}
			}
			if commitResp == nil {
				return nil, withCommit, errors.New("bad request")
			}
			wc++
			commits := commitResp.Commits
			fmt.Println("rcc", len(commits))
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

	listBlobsReq := &gitalypb.GetBlobsRequest{
		Repository:    repository,
		RevisionPaths: revisionPaths,
		Limit:         1024,
	}

	// Get blobs with file size
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

func TestFileTree(t *testing.T) {
	client, err := newTestClient()
	if err != nil {
		t.Fatal(err)
	}
	files, err := client.GetRepoFileTree(context.TODO(), gitserver.GetRepoInfoByPathReq{
		Namespace: "",
		Name:      "dronescapes",
		Ref:       "main",
		Path:      "data/semisupervised_set/depth_dpt/part0",
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("fetch all commits success")
	fmt.Println(len(files))
	t.FailNow()
}
