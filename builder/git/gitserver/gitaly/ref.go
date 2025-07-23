package gitaly

import (
	"context"
	"fmt"

	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
)

func (c *Client) UpdateRef(ctx context.Context, req gitserver.UpdateRefReq) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return err
	}
	client, err := c.refClient.UpdateReferences(ctx)
	if err != nil {
		return err
	}

	err = client.Send(&gitalypb.UpdateReferencesRequest{
		Repository: &gitalypb.Repository{
			RelativePath: relativePath,
			StorageName:  c.config.GitalyServer.Storage,
		},
		Updates: []*gitalypb.UpdateReferencesRequest_Update{
			{
				Reference:   []byte(req.Ref),
				OldObjectId: []byte(req.OldObjectId),
				NewObjectId: []byte(req.NewObjectId),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("UpdateReferences send: %v", err)
	}
	_, err = client.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("UpdateReferences recv: %v", err)
	}

	return nil
}
