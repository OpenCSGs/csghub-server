package gitaly

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
)

func (c *Client) GetRepoBranches(ctx context.Context, req gitserver.GetBranchesReq) ([]types.Branch, error) {
	var branches []types.Branch
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()
	branchesReq := &gitalypb.FindAllBranchesRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
	}
	stream, err := c.refClient.FindAllBranches(ctx, branchesReq)
	if err != nil {
		return nil, err
	}
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if resp != nil {
			for _, branch := range resp.Branches {
				branches = append(branches, types.Branch{
					Name:    filepath.Base(string(branch.Name)),
					Message: string(branch.Target.Subject),
					Commit: types.RepoBranchCommit{
						ID: branch.Target.Id,
					},
				})
			}
		}
	}

	return branches, nil
}
