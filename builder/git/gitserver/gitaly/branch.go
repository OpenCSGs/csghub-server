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
	ctx, cancel := context.WithTimeout(ctx, c.timeoutTime)
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

func (c *Client) GetRepoBranchByName(ctx context.Context, req gitserver.GetBranchReq) (*types.Branch, error) {
	var branch types.Branch
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, c.timeoutTime)
	defer cancel()
	branchReq := &gitalypb.FindBranchRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		Name: []byte(req.Ref),
	}
	resp, err := c.refClient.FindBranch(ctx, branchReq)

	if err != nil {
		return nil, err
	}

	// if branch not found, return nil
	if resp == nil || resp.Branch == nil {
		return nil, nil
	}

	branch.Name = filepath.Base(string(resp.Branch.Name))
	branch.Message = string(resp.Branch.TargetCommit.Subject)
	branch.Commit = types.RepoBranchCommit{
		ID: resp.Branch.TargetCommit.Id,
	}

	return &branch, nil
}

func (c *Client) DeleteRepoBranch(ctx context.Context, req gitserver.DeleteBranchReq) error {
	repoType := fmt.Sprintf("%ss", string(req.RepoType))

	deleteBranchReq := &gitalypb.UserDeleteBranchRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
			GlRepository: filepath.Join(repoType, req.Namespace, req.Name),
		},
		BranchName: []byte(req.Ref),
		User: &gitalypb.User{
			GlId:       "user-1",
			Name:       []byte(req.Name),
			GlUsername: req.Username,
			Email:      []byte(req.Email),
		},
	}

	_, err := c.operationClient.UserDeleteBranch(ctx, deleteBranchReq)
	if err != nil {
		return err
	}

	return nil
}
