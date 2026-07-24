package gitaly

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	gitalyauth "gitlab.com/gitlab-org/gitaly/v16/auth"
	gitalyclient "gitlab.com/gitlab-org/gitaly/v16/client"
	gitalypb "gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"gitlab.com/gitlab-org/gitaly/v16/streamio"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/utils/common"
)

const timeoutTime = 10 * time.Second

// RepositoryExists reports whether the requested repository exists in Gitaly storage.
func (c *Client) RepositoryExists(ctx context.Context, req gitserver.CheckRepoReq) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()

	relativePath, err := c.resolveRelativePath(ctx, req.RelativePath, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return false, err
	}
	r, err := c.repoClient.RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
		},
	})

	if err != nil {
		return false, errorx.ErrGitCheckRepositoryExistsFailed(err, errorx.Ctx())
	}
	if r == nil {
		return false, errors.New("empty response for check repository exists")
	}
	return r.Exists, nil
}

// CreateRepo creates an empty repository at the resolved Gitaly storage path.
func (c *Client) CreateRepo(ctx context.Context, req gitserver.CreateRepoReq) (*gitserver.CreateRepoResp, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	relativePath, err := c.resolveRelativePath(ctx, req.RelativePath, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}
	gitalyReq := &gitalypb.CreateRepositoryRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
		},
		DefaultBranch: []byte(req.DefaultBranch),
	}

	_, err = c.repoClient.CreateRepository(ctx, gitalyReq)
	if err != nil {
		return nil, errorx.ErrGitCreateRepositoryFailed(err, errorx.Ctx())
	}
	repoTypeS := fmt.Sprintf("%ss", string(req.RepoType))

	return &gitserver.CreateRepoResp{
		Username:      req.Username,
		Namespace:     req.Namespace,
		Name:          req.Name,
		Nickname:      req.Nickname,
		Description:   req.Description,
		License:       req.License,
		DefaultBranch: req.DefaultBranch,
		RepoType:      req.RepoType,
		GitPath:       common.BuildRelativePath(repoTypeS, req.Namespace, req.Name),
		Private:       req.Private,
	}, nil
}

func (c *Client) UpdateRepo(ctx context.Context, req gitserver.UpdateRepoReq) (*gitserver.CreateRepoResp, error) {
	var err error
	if req.DefaultBranch != "" {
		err = c.SetDefaultBranch(ctx, gitserver.SetDefaultBranchReq{
			Namespace:  req.Namespace,
			Name:       req.Name,
			BranchName: req.DefaultBranch,
			RepoType:   req.RepoType,
		})
	}
	return nil, err
}

func (c *Client) DeleteRepo(ctx context.Context, relativePath string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	gitalyReq := &gitalypb.RemoveRepositoryRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
		},
	}
	_, err := c.repoClient.RemoveRepository(ctx, gitalyReq)
	if err != nil {
		return errorx.ErrGitDeleteRepositoryFailed(err, errorx.Ctx())
	}

	return nil
}

// GetRepo returns Git metadata for the repository at the resolved Gitaly storage path.
func (c *Client) GetRepo(ctx context.Context, req gitserver.GetRepoReq) (*gitserver.CreateRepoResp, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	relativePath, err := c.resolveRelativePath(ctx, req.RelativePath, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}
	gitalyReq := &gitalypb.FindDefaultBranchNameRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
		},
	}
	resp, err := c.refClient.FindDefaultBranchName(ctx, gitalyReq)
	if err != nil {
		return nil, errorx.ErrGitGetRepositoryFailed(err, errorx.Ctx())
	}

	return &gitserver.CreateRepoResp{DefaultBranch: string(resp.Name)}, nil
}

func (c *Client) GetArchive(ctx context.Context, req gitserver.GetArchiveReq) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}

	stream, err := c.repoClient.GetArchive(ctx, &gitalypb.GetArchiveRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
		},
		CommitId:        req.Revision,
		Prefix:          req.Name,
		Format:          gitalypb.GetArchiveRequest_ZIP,
		Path:            []byte("."),
		IncludeLfsBlobs: true,
	})
	if err != nil {
		return nil, errorx.GetArchiveFailed(err, errorx.Ctx().Set("namespace", req.Namespace).Set("name", req.Name).Set("revision", req.Revision))
	}

	buf := bytes.NewBuffer(nil)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errorx.GetArchiveFailed(err, errorx.Ctx().Set("namespace", req.Namespace).Set("name", req.Name).Set("revision", req.Revision))
		}
		buf.Write(resp.GetData())
	}

	maxSize := c.config.Git.MaxArchiveSizeMB * 1024 * 1024
	if int64(buf.Len()) > maxSize {
		return nil, fmt.Errorf("archive size %d exceeds limit of %d MB", buf.Len(), c.config.Git.MaxArchiveSizeMB)
	}

	return stripZipPrefix(buf.Bytes(), req.Name)
}

func stripZipPrefix(zipData []byte, prefix string) ([]byte, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, errorx.GetArchiveFailed(err, errorx.Ctx().Set("prefix", prefix))
	}

	var outBuf bytes.Buffer
	zipWriter := zip.NewWriter(&outBuf)

	prefixDir := prefix + "/"
	totalEntries := len(zipReader.File)
	fileCount := 0

	for _, file := range zipReader.File {
		path := strings.TrimPrefix(file.Name, "/")
		if path == "" {
			continue
		}

		path = strings.TrimPrefix(path, prefixDir)
		if path == "" {
			continue
		}

		writer, err := zipWriter.Create(path)
		if err != nil {
			zipWriter.Close()
			return nil, errorx.GetArchiveFailed(err, errorx.Ctx().Set("prefix", prefix).Set("entry", path))
		}

		rc, err := file.Open()
		if err != nil {
			zipWriter.Close()
			return nil, errorx.GetArchiveFailed(err, errorx.Ctx().Set("prefix", prefix).Set("entry", file.Name))
		}

		_, err = io.Copy(writer, rc)
		rc.Close()
		if err != nil {
			zipWriter.Close()
			return nil, errorx.GetArchiveFailed(err, errorx.Ctx().Set("prefix", prefix).Set("entry", file.Name))
		}
		fileCount++
	}

	if totalEntries > 0 && fileCount == 0 {
		zipWriter.Close()
		return nil, fmt.Errorf("archive is empty: no files matched prefix %q", prefix)
	}

	err = zipWriter.Close()
	if err != nil {
		return nil, errorx.GetArchiveFailed(err, errorx.Ctx().Set("prefix", prefix))
	}

	return outBuf.Bytes(), nil
}

func (c *Client) CopyRepository(ctx context.Context, req gitserver.CopyRepositoryReq) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return err
	}

	sourceRepo := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
	}

	destRepo := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: req.NewPath,
	}

	resp, err := c.repoClient.CreateBundle(ctx, &gitalypb.CreateBundleRequest{
		Repository: sourceRepo,
	})
	if err != nil {
		return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
	}

	client, err := c.repoClient.CreateRepositoryFromBundle(ctx)
	if err != nil {
		return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
	}

	for {
		data, err := resp.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
		}

		err = client.Send(&gitalypb.CreateRepositoryFromBundleRequest{
			Repository: destRepo,
			Data:       data.GetData(),
		})

		if err != nil {
			return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
		}
	}

	_, err = client.CloseAndRecv()
	if err != nil {
		return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
	}

	return nil
}

func (c *Client) GetRepoSize(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return 0, err
	}

	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
	}

	// Use ListBlobs to get all blobs for the specified branch
	listBlobsReq := &gitalypb.ListBlobsRequest{
		Repository: repository,
		WithPaths:  true,
		Revisions:  []string{req.Ref},
	}

	result, err := c.blobClient.ListBlobs(ctx, listBlobsReq)
	if err != nil {
		return 0, errorx.ErrGitGetBlobsFailed(err, errorx.Ctx())
	}

	var totalSize int64
	for {
		allFilesResp, err := result.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, errorx.ErrGitGetBlobsFailed(err, errorx.Ctx())
		}
		if allFilesResp != nil {
			for _, blob := range allFilesResp.Blobs {
				totalSize += blob.Size
			}
		}
	}

	return totalSize, nil
}

func (c *Client) GetRepoLfsSize(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return 0, err
	}

	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
	}

	pointersReq := &gitalypb.ListLFSPointersRequest{
		Repository: repository,
		Revisions:  []string{req.Ref},
	}

	allPointersStream, err := c.blobClient.ListLFSPointers(ctx, pointersReq)
	if err != nil {
		return 0, errorx.ErrGitGetLfsPointersFailed(err, errorx.Ctx())
	}

	var totalSize int64
	for {
		allPointersResp, err := allPointersStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, errorx.ErrGitGetLfsPointersFailed(err, errorx.Ctx())
		}
		if allPointersResp != nil {
			for _, pointer := range allPointersResp.LfsPointers {
				totalSize += pointer.FileSize
			}
		}
	}

	return totalSize, nil
}

const getBlobsBatchSize = 1000

func (c *Client) GetLastCommitSize(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	errCtx := errorx.Ctx().
		Set("repo_type", req.RepoType).
		Set("namespace", req.Namespace).
		Set("name", req.Name).
		Set("ref", req.Ref)

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return 0, err
	}

	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
	}

	// Get all file paths at HEAD
	listFilesReq := &gitalypb.ListFilesRequest{
		Repository: repository,
		Revision:   []byte(req.Ref),
	}
	filesStream, err := c.commitClient.ListFiles(ctx, listFilesReq)
	if err != nil {
		return 0, errorx.ErrGitListFilesFailed(err, errCtx)
	}

	var allPaths []string
	for {
		filesResp, err := filesStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, errorx.ErrGitListFilesFailed(err, errCtx)
		}
		if filesResp != nil {
			for _, path := range filesResp.Paths {
				allPaths = append(allPaths, string(path))
			}
		}
	}

	if len(allPaths) == 0 {
		return 0, nil
	}

	// Get blob sizes in batches to avoid exceeding gRPC message limits
	var (
		lastCommitSize int64
		blobIDs        []string
	)
	for i := 0; i < len(allPaths); i += getBlobsBatchSize {
		end := i + getBlobsBatchSize
		if end > len(allPaths) {
			end = len(allPaths)
		}
		batch := allPaths[i:end]

		var revisionPaths []*gitalypb.GetBlobsRequest_RevisionPath
		for _, path := range batch {
			revisionPaths = append(revisionPaths, &gitalypb.GetBlobsRequest_RevisionPath{
				Revision: req.Ref,
				Path:     []byte(path),
			})
		}

		blobsStream, err := c.blobClient.GetBlobs(ctx, &gitalypb.GetBlobsRequest{
			Repository:    repository,
			RevisionPaths: revisionPaths,
			Limit:         0,
		})
		if err != nil {
			return 0, errorx.ErrGitGetBlobsFailed(err, errCtx)
		}

		for {
			blobResp, err := blobsStream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				return 0, errorx.ErrGitGetBlobsFailed(err, errCtx)
			}
			if blobResp != nil {
				lastCommitSize += blobResp.Size
				if blobResp.Oid != "" {
					blobIDs = append(blobIDs, blobResp.Oid)
				}
			}
		}
	}

	// Get LFS file sizes in batches
	for i := 0; i < len(blobIDs); i += getBlobsBatchSize {
		end := i + getBlobsBatchSize
		if end > len(blobIDs) {
			end = len(blobIDs)
		}
		batch := blobIDs[i:end]

		pointersStream, err := c.blobClient.GetLFSPointers(ctx, &gitalypb.GetLFSPointersRequest{
			Repository: repository,
			BlobIds:    batch,
		})
		if err != nil {
			return 0, errorx.ErrGitGetLfsPointersFailed(err, errCtx)
		}

		for {
			pointerResp, err := pointersStream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				return 0, errorx.ErrGitGetLfsPointersFailed(err, errCtx)
			}
			if pointerResp != nil {
				for _, pointer := range pointerResp.LfsPointers {
					lastCommitSize += pointer.FileSize
				}
			}
		}
	}

	return lastCommitSize, nil
}

func (c *Client) CreateFork(ctx context.Context, req gitserver.CreateForkReq) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Build relative path for source repository
	sourceRelativePath, err := c.BuildRelativePath(ctx, req.SourceRepoType, req.SourceNamespace, req.SourceName)
	if err != nil {
		return err
	}

	// Build relative path for target repository
	targetRelativePath, err := c.BuildRelativePath(ctx, req.TargetRepoType, req.TargetNamespace, req.TargetName)
	if err != nil {
		return err
	}

	// Create gitaly fork request
	gitalyReq := &gitalypb.CreateForkRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: targetRelativePath,
		},
		SourceRepository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: sourceRelativePath,
		},
	}

	// Set revision if provided
	if req.Revision != "" {
		gitalyReq.Revision = []byte("refs/heads/" + req.Revision)
	}

	// Call gitaly CreateFork method
	_, err = c.repoClient.CreateFork(ctx, gitalyReq)
	if err != nil {
		return errorx.ErrGitCreateForkFailed(err, errorx.Ctx())
	}

	return nil
}

type ProjectStorageCloneRequest struct {
	CurrentGitalyAddress string
	CurrentGitalyToken   string
	CurrentGitalyStorage string
	NewGitalyAddress     string
	NewGitalyToken       string
	NewGitalyStorage     string
	Concurrency          int
	FilesServer          string
}

type CloneStorageHelper struct {
	from gitalypb.RepositoryServiceClient
	to   gitalypb.RepositoryServiceClient
}

func repoClient(address, token, storage string) (gitalypb.RepositoryServiceClient, error) {
	var sidechannelRegistry *gitalyclient.SidechannelRegistry
	accessLogger := log.New()
	accessLogger.SetLevel(log.InfoLevel)
	sidechannelRegistry = gitalyclient.NewSidechannelRegistry(log.NewEntry(accessLogger))
	addressInfo := map[string]string{"address": address, "token": token}
	jsonData, err := json.Marshal(map[string]any{storage: addressInfo})
	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(jsonData)

	md := metadata.New(map[string]string{"gitaly-servers": encoded})
	streamingAddMd := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		mdo, _ := metadata.FromOutgoingContext(ctx)
		ctx = metadata.NewOutgoingContext(ctx, metadata.Join(mdo, md))
		return streamer(ctx, desc, cc, method, opts...)
	}
	unaryAddMd := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		mdo, _ := metadata.FromOutgoingContext(ctx)
		ctx = metadata.NewOutgoingContext(ctx, metadata.Join(mdo, md))
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	connOpts := append(gitalyclient.DefaultDialOpts,
		grpc.WithPerRPCCredentials(gitalyauth.RPCCredentialsV2(token)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		gitalyclient.WithGitalyDNSResolver(gitalyclient.DefaultDNSResolverBuilderConfig()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithStreamInterceptor(streamingAddMd),
		grpc.WithUnaryInterceptor(unaryAddMd),
	)

	conn, err := gitalyclient.DialSidechannel(context.Background(), address, sidechannelRegistry, connOpts)
	if err != nil {
		return nil, err
	}
	return gitalypb.NewRepositoryServiceClient(conn), nil

}

func (h *CloneStorageHelper) CloneRepoStorage(ctx context.Context, path string, req *ProjectStorageCloneRequest) error {
	r, err := h.from.RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
		Repository: &gitalypb.Repository{
			StorageName:  req.CurrentGitalyStorage,
			RelativePath: path,
		},
	})
	if err != nil {
		return err
	}
	if !r.Exists {
		return errors.New("repo not exists on current Gitaly instance")
	}

	r, err = h.to.RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
		Repository: &gitalypb.Repository{
			StorageName:  req.NewGitalyStorage,
			RelativePath: path,
		},
	})
	if err != nil {
		return err
	}
	if r.Exists {
		return nil
	}

	stream, err := h.from.GetSnapshot(ctx, &gitalypb.GetSnapshotRequest{
		Repository: &gitalypb.Repository{
			StorageName:  req.CurrentGitalyStorage,
			RelativePath: path,
		},
	})
	if err != nil {
		return err
	}
	reader := streamio.NewReader(func() ([]byte, error) {
		response, err := stream.Recv()
		return response.GetData(), err
	})
	fileName := fmt.Sprintf("%s.tar", strings.ReplaceAll(path, "/", "_"))
	outFile, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(fileName) }()
	_, err = io.Copy(outFile, reader)
	_ = outFile.Close()
	if err != nil {

		return err
	}

	_, err = h.to.CreateRepositoryFromSnapshot(ctx,
		&gitalypb.CreateRepositoryFromSnapshotRequest{
			Repository: &gitalypb.Repository{
				StorageName:  req.NewGitalyStorage,
				RelativePath: path,
			},
			HttpUrl: req.FilesServer + fileName,
		})
	if err != nil {
		return err
	}
	return nil
}

func (h *CloneStorageHelper) TransferRepoBundle(ctx context.Context, fromPath, toPath string, req *ProjectStorageCloneRequest) error {
	r, err := h.from.RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
		Repository: &gitalypb.Repository{
			StorageName:  req.CurrentGitalyStorage,
			RelativePath: fromPath,
		},
	})
	if err != nil {
		return err
	}
	if !r.Exists {
		return errors.New("repo not exists on current Gitaly instance")
	}

	r, err = h.to.RepositoryExists(ctx, &gitalypb.RepositoryExistsRequest{
		Repository: &gitalypb.Repository{
			StorageName:  req.NewGitalyStorage,
			RelativePath: toPath,
		},
	})
	if err != nil {
		return err
	}
	if r.Exists {
		return nil
	}

	resp, err := h.from.CreateBundle(ctx, &gitalypb.CreateBundleRequest{
		Repository: &gitalypb.Repository{
			StorageName:  req.CurrentGitalyStorage,
			RelativePath: fromPath,
		},
	})
	if err != nil {
		return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
	}

	client, err := h.to.CreateRepositoryFromBundle(ctx)
	if err != nil {
		return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
	}

	for {
		data, err := resp.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
		}
		err = client.Send(&gitalypb.CreateRepositoryFromBundleRequest{
			Repository: &gitalypb.Repository{
				StorageName:  req.NewGitalyStorage,
				RelativePath: toPath,
			},
			Data: data.GetData(),
		})

		if err != nil {
			return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
		}
	}

	_, err = client.CloseAndRecv()
	if err != nil {
		return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
	}

	return nil
}

func NewCloneStorageHelper(req *ProjectStorageCloneRequest) (*CloneStorageHelper, error) {
	from, err := repoClient(
		req.CurrentGitalyAddress, req.CurrentGitalyToken, req.CurrentGitalyStorage,
	)
	if err != nil {
		return nil, err
	}
	to, err := repoClient(
		req.NewGitalyAddress, req.NewGitalyToken, req.NewGitalyStorage,
	)
	if err != nil {
		return nil, err
	}
	return &CloneStorageHelper{from: from, to: to}, nil
}
