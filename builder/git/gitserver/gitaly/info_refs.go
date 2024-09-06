package gitaly

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	gitalyclient "gitlab.com/gitlab-org/gitaly/v16/client"
	gitalypb "gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"gitlab.com/gitlab-org/gitaly/v16/streamio"
	"opencsg.com/csghub-server/builder/git/gitserver"
)

var (
	uploadPackTimeout = 10 * time.Minute
)

func (c *Client) InfoRefsResponse(ctx context.Context, req gitserver.InfoRefsReq) (io.Reader, error) {
	repoType := fmt.Sprintf("%ss", string(req.RepoType))

	rpcRequest := &gitalypb.InfoRefsRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		GitProtocol: req.GitProtocol,
	}

	switch req.Rpc {
	case "git-upload-pack":
		stream, err := c.smartHttpClient.InfoRefsUploadPack(ctx, rpcRequest)
		return infoRefsReader(stream), err
	case "git-receive-pack":
		stream, err := c.smartHttpClient.InfoRefsReceivePack(ctx, rpcRequest)
		return infoRefsReader(stream), err
	default:
		return nil, fmt.Errorf("InfoRefsResponseWriterTo: Unsupported RPC: %q", req.Rpc)
	}
}

func infoRefsReader(stream infoRefsClient) io.Reader {
	return streamio.NewReader(func() ([]byte, error) {
		resp, err := stream.Recv()
		return resp.GetData(), err
	})
}

type infoRefsClient interface {
	Recv() (*gitalypb.InfoRefsResponse, error)
}

func (c *Client) UploadPack(ctx context.Context, req gitserver.UploadPackReq) error {
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, waiter := c.sidechannelRegistry.Register(ctx, func(conn gitalyclient.SidechannelConn) error {
		if _, err := io.Copy(conn, req.Request.Body); err != nil {
			return fmt.Errorf("copy request body: %w", err)
		}

		if err := conn.CloseWrite(); err != nil {
			return fmt.Errorf("close request body: %w", err)
		}

		if _, err := io.Copy(req.Writer, conn); err != nil {
			return fmt.Errorf("copy response body: %w", err)
		}

		return nil
	})
	defer waiter.Close()

	rpcRequest := &gitalypb.PostUploadPackWithSidechannelRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		GitProtocol: req.GitProtocol,
	}

	_, err := c.smartHttpClient.PostUploadPackWithSidechannel(ctx, rpcRequest)
	if err != nil {
		return fmt.Errorf("PostUploadPackWithSidechannel: %w", err)
	}

	if err = waiter.Close(); err != nil {
		return fmt.Errorf("close sidechannel waiter: %w", err)
	}

	return nil
}

func (c *Client) ReceivePack(ctx context.Context, req gitserver.ReceivePackReq) error {
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	stream, err := c.smartHttpClient.PostReceivePack(ctx)
	if err != nil {
		return err
	}
	glRepository := fmt.Sprintf("%s/%s/%s", req.RepoType, req.Namespace, req.Name)

	rpcRequest := &gitalypb.PostReceivePackRequest{
		Repository: &gitalypb.Repository{
			GlRepository: glRepository,
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		GlId:         fmt.Sprintf("user-%s", strconv.FormatInt(req.UserId, 10)),
		GlUsername:   req.Username,
		GlRepository: glRepository,
		GitProtocol:  req.GitProtocol,
	}

	if err := stream.Send(rpcRequest); err != nil {
		return fmt.Errorf("initial request: %v", err)
	}

	numStreams := 2
	errC := make(chan error, numStreams)

	go func() {
		rr := streamio.NewReader(func() ([]byte, error) {
			response, err := stream.Recv()
			return response.GetData(), err
		})
		_, err := io.Copy(req.Writer, rr)
		errC <- err
	}()

	go func() {
		sw := streamio.NewWriter(func(data []byte) error {
			return stream.Send(&gitalypb.PostReceivePackRequest{Data: data})
		})
		_, err := io.Copy(sw, req.Request.Body)
		stream.CloseSend()
		errC <- err
	}()

	for i := 0; i < numStreams; i++ {
		if err := <-errC; err != nil {
			return err
		}
	}

	return nil
}
