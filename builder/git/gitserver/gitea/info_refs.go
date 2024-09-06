package gitea

import (
	"context"
	"io"

	"opencsg.com/csghub-server/builder/git/gitserver"
)

func (c *Client) InfoRefsResponse(ctx context.Context, req gitserver.InfoRefsReq) (io.Reader, error) {
	return nil, nil
}

func (c *Client) UploadPack(ctx context.Context, req gitserver.UploadPackReq) error {
	return nil
}

func (c *Client) ReceivePack(ctx context.Context, req gitserver.ReceivePackReq) error {
	return nil
}
