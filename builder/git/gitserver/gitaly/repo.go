package gitaly

import (
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

func (c *Client) RepositoryExists(ctx context.Context, req gitserver.CheckRepoReq) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
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

func (c *Client) CreateRepo(ctx context.Context, req gitserver.CreateRepoReq) (*gitserver.CreateRepoResp, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
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
	return nil, nil
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

func (c *Client) GetRepo(ctx context.Context, req gitserver.GetRepoReq) (*gitserver.CreateRepoResp, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
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

func (c *Client) CopyRepository(ctx context.Context, req gitserver.CopyRepositoryReq) error {
	var bundleData []byte
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

	for {
		data, err := resp.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
		}
		bundleData = append(bundleData, data.GetData()...)
	}

	client, err := c.repoClient.CreateRepositoryFromBundle(ctx)
	if err != nil {
		return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
	}
	err = client.Send(&gitalypb.CreateRepositoryFromBundleRequest{
		Repository: destRepo,
		Data:       bundleData,
	})

	if err != nil {
		return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
	}

	_, err = client.CloseAndRecv()
	if err != nil {
		return errorx.ErrGitCopyRepositoryFailed(err, errorx.Ctx())
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
