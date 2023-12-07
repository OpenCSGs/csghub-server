package gitea

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
)

func (c *Client) GetModelDetail(namespace, name string) (model *types.ModelDetail, err error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	giteaRepo, _, err := c.giteaClient.GetRepo(namespace, name)
	if err != nil {
		return
	}
	model = &types.ModelDetail{
		Path:          common.WithoutPrefix(giteaRepo.FullName, ModelOrgPrefix),
		Name:          giteaRepo.Name,
		Introduction:  giteaRepo.Description,
		License:       "",
		DownloadCount: 0,
		LastUpdatedAt: giteaRepo.Updated.Format("2006-01-02 15:04:05"),
		HTTPCloneURL:  common.PortalCloneUrl(giteaRepo.CloneURL, ModelOrgPrefix),
		SSHCloneURL:   giteaRepo.SSHURL,
		Size:          giteaRepo.Size,
		DefaultBranch: giteaRepo.DefaultBranch,
	}
	return
}

func (c *Client) GetDatasetDetail(namespace, name string) (model *types.DatasetDetail, err error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	giteaRepo, _, err := c.giteaClient.GetRepo(namespace, name)
	if err != nil {
		return
	}
	model = &types.DatasetDetail{
		Path:          common.WithoutPrefix(giteaRepo.FullName, DatasetOrgPrefix),
		Name:          giteaRepo.Name,
		Introduction:  giteaRepo.Description,
		License:       "",
		DownloadCount: 0,
		LastUpdatedAt: giteaRepo.Updated.Format("2006-01-02 15:04:05"),
		HTTPCloneURL:  common.PortalCloneUrl(giteaRepo.CloneURL, DatasetOrgPrefix),
		SSHCloneURL:   giteaRepo.SSHURL,
		Size:          giteaRepo.Size,
		DefaultBranch: giteaRepo.DefaultBranch,
	}
	return
}
