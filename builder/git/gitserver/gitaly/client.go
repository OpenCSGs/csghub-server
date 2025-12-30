package gitaly

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"
	gitalyauth "gitlab.com/gitlab-org/gitaly/v16/auth"
	gitalyclient "gitlab.com/gitlab-org/gitaly/v16/client"
	gitalypb "gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

var _ gitserver.GitServer = (*Client)(nil)

type Client struct {
	config              *config.Config
	sidechannelRegistry *gitalyclient.SidechannelRegistry
	repoClient          gitalypb.RepositoryServiceClient
	commitClient        gitalypb.CommitServiceClient
	blobClient          gitalypb.BlobServiceClient
	refClient           gitalypb.RefServiceClient
	diffClient          gitalypb.DiffServiceClient
	operationClient     gitalypb.OperationServiceClient
	smartHttpClient     gitalypb.SmartHTTPServiceClient
	remoteClient        gitalypb.RemoteServiceClient
	timeout             time.Duration
	treeTimeout         time.Duration
	repoStore           database.RepoStore
}

func NewClient(config *config.Config) (*Client, error) {
	var sidechannelRegistry *gitalyclient.SidechannelRegistry
	accessLogger := log.New()
	accessLogger.SetLevel(log.InfoLevel)
	sidechannelRegistry = gitalyclient.NewSidechannelRegistry(log.NewEntry(accessLogger))

	addressInfo := map[string]string{
		"address": config.GitalyServer.Address, "token": config.GitalyServer.Token,
	}

	jsonData, err := json.Marshal(map[string]any{
		config.GitalyServer.Storage: addressInfo,
	})
	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(jsonData)

	md := metadata.New(map[string]string{"gitaly-servers": encoded})
	onlyPrimaryMd := metadata.New(map[string]string{"gitaly-route-repository-accessor-policy": "primary-only"})
	streamingAddMd := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		mdo, _ := metadata.FromOutgoingContext(ctx)
		ctx = metadata.NewOutgoingContext(ctx, metadata.Join(mdo, md, onlyPrimaryMd))
		return streamer(ctx, desc, cc, method, opts...)
	}
	unaryAddMd := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		mdo, _ := metadata.FromOutgoingContext(ctx)
		ctx = metadata.NewOutgoingContext(ctx, metadata.Join(mdo, md, onlyPrimaryMd))
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	connOpts := append(gitalyclient.DefaultDialOpts,
		grpc.WithPerRPCCredentials(gitalyauth.RPCCredentialsV2(config.GitalyServer.Token)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		gitalyclient.WithGitalyDNSResolver(gitalyclient.DefaultDNSResolverBuilderConfig()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithStreamInterceptor(streamingAddMd),
		grpc.WithUnaryInterceptor(unaryAddMd),
	)

	conn, connErr := gitalyclient.DialSidechannel(context.Background(), config.GitalyServer.Address, sidechannelRegistry, connOpts)
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

	timeoutTime := time.Duration(config.Git.OperationTimeout) * time.Second
	treeTimeoutTime := time.Duration(config.Git.TreeOperationTimeout) * time.Second
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
		timeout:             timeoutTime,
		repoStore:           database.NewRepoStore(),
		treeTimeout:         treeTimeoutTime,
	}, nil
}
