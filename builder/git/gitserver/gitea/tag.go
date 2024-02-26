package gitea

import (
	"context"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func (c *Client) GetRepoTags(ctx context.Context, req gitserver.GetRepoTagsReq) (tags []*types.Tag, err error) {
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	giteaTags, _, err := c.giteaClient.ListRepoTags(
		namespace,
		req.Name,
		gitea.ListRepoTagsOptions{
			ListOptions: gitea.ListOptions{
				PageSize: req.Per,
				Page:     req.Page,
			},
		},
	)
	if err != nil {
		return
	}
	for _, giteaTag := range giteaTags {
		tag := &types.Tag{
			Name:    giteaTag.Name,
			Message: giteaTag.Message,
			Commit: types.DatasetTagCommit{
				ID: giteaTag.Commit.SHA,
			},
		}
		tags = append(tags, tag)
	}
	return
}
