package gitea

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/pulltheflower/gitea-go-sdk/gitea"
)

func (c *Client) GetDatasetTags(namespace, name string, per, page int) (tags []*types.DatasetTag, err error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	giteaTags, _, err := c.giteaClient.ListRepoTags(
		namespace,
		name,
		gitea.ListRepoTagsOptions{
			ListOptions: gitea.ListOptions{
				PageSize: per,
				Page:     page,
			},
		},
	)
	if err != nil {
		return
	}
	for _, giteaTag := range giteaTags {
		tag := &types.DatasetTag{
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

func (c *Client) GetModelTags(namespace, name string, per, page int) (tags []*types.ModelTag, err error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	giteaTags, _, err := c.giteaClient.ListRepoTags(
		namespace,
		name,
		gitea.ListRepoTagsOptions{
			ListOptions: gitea.ListOptions{
				PageSize: per,
				Page:     page,
			},
		},
	)
	if err != nil {
		return
	}
	for _, giteaTag := range giteaTags {
		tag := &types.ModelTag{
			Name:    giteaTag.Name,
			Message: giteaTag.Message,
			Commit: types.ModelTagCommit{
				ID: giteaTag.Commit.SHA,
			},
		}
		tags = append(tags, tag)
	}
	return
}
