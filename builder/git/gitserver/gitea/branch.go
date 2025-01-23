package gitea

import (
	"context"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func (c *Client) GetRepoBranches(ctx context.Context, req gitserver.GetBranchesReq) ([]types.Branch, error) {
	var branches []types.Branch
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	giteaBranches, _, err := c.giteaClient.ListRepoBranches(
		namespace,
		req.Name,
		gitea.ListRepoBranchesOptions{
			ListOptions: gitea.ListOptions{
				PageSize: req.Per,
				Page:     req.Page,
			},
		},
	)
	for _, giteaBranch := range giteaBranches {
		branches = append(branches, types.Branch{
			Name:    giteaBranch.Name,
			Message: giteaBranch.Commit.Message,
			Commit: types.RepoBranchCommit{
				ID: giteaBranch.Commit.ID,
			},
		})
	}
	return branches, err
}

func (c *Client) GetRepoBranchByName(ctx context.Context, req gitserver.GetBranchReq) (*types.Branch, error) {
	var branch types.Branch
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	giteaBranch, _, err := c.giteaClient.GetRepoBranch(
		namespace,
		req.Name,
		req.Ref,
	)
	if err != nil {
		return nil, err
	}

	if giteaBranch == nil {
		return nil, nil
	}

	branch.Name = giteaBranch.Name
	branch.Message = giteaBranch.Commit.Message
	branch.Commit = types.RepoBranchCommit{
		ID: giteaBranch.Commit.ID,
	}

	return &branch, err
}

func (c *Client) DeleteRepoBranch(ctx context.Context, req gitserver.DeleteBranchReq) error {
	return nil
}
