package gitaly

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func (c *Client) GetRepoBranches(ctx context.Context, req gitserver.GetBranchesReq) ([]types.Branch, error) {
	var branches []types.Branch
	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	branchesReq := &gitalypb.FindAllBranchesRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
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
			return nil, errorx.FindBranchFailed(err, errorx.Ctx().
				Set("repo_type", req.RepoType).
				Set("path", relativePath),
			)
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
	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	branchReq := &gitalypb.FindBranchRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
		},
		Name: []byte(req.Ref),
	}
	resp, err := c.refClient.FindBranch(ctx, branchReq)

	if err != nil {
		return nil, errorx.FindBranchFailed(err, errorx.Ctx().
			Set("repo_type", req.RepoType).
			Set("path", relativePath).
			Set("branch", req.Ref),
		)
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
	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return err
	}

	deleteBranchReq := &gitalypb.UserDeleteBranchRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
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

	_, err = c.operationClient.UserDeleteBranch(ctx, deleteBranchReq)
	if err != nil {
		return errorx.DeleteBranchFailed(err, errorx.Ctx().
			Set("repo_type", req.RepoType).
			Set("path", relativePath).
			Set("branch", req.Ref),
		)
	}

	return nil
}

func (c *Client) CreateBranch(ctx context.Context, req gitserver.CreateBranchReq) error {
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return err
	}
	client, err := c.refClient.UpdateReferences(ctx)
	if err != nil {
		return err
	}
	createBranchReq := &gitalypb.UpdateReferencesRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
			GlRepository: filepath.Join(repoType, req.Namespace, req.Name),
		},
		Updates: []*gitalypb.UpdateReferencesRequest_Update{
			{
				Reference:   []byte("refs/heads/" + req.BranchName),
				NewObjectId: []byte(req.CommitID),
			},
		},
	}

	err = client.Send(createBranchReq)
	if err != nil {
		return errorx.CreateBranchFailed(err, errorx.Ctx().
			Set("repo_type", req.RepoType).
			Set("path", relativePath).
			Set("branch", req.BranchName).
			Set("commit_id", req.CommitID),
		)
	}

	_, err = client.CloseAndRecv()
	if err != nil {
		return errorx.CreateBranchFailed(err, errorx.Ctx().
			Set("repo_type", req.RepoType).
			Set("path", relativePath).
			Set("branch", req.BranchName).
			Set("commit_id", req.CommitID),
		)
	}
	return nil
}
