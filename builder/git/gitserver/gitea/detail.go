package gitea

import (
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
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
		Private:       giteaRepo.Private,
		Downloads:     0,
		LastUpdatedAt: giteaRepo.Updated.Format("006-01-02 15:04:05"),
		HTTPCloneURL:  PortalCloneUrl(giteaRepo.CloneURL, types.ModelRepo, c.config.GitServer.URL, c.config.Frontend.URL),
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
		Private:       giteaRepo.Private,
		Downloads:     0,
		LastUpdatedAt: giteaRepo.Updated.Format("2006-01-02 15:04:05"),
		HTTPCloneURL:  PortalCloneUrl(giteaRepo.CloneURL, types.DatasetRepo, c.config.GitServer.URL, c.config.Frontend.URL),
		SSHCloneURL:   giteaRepo.SSHURL,
		Size:          giteaRepo.Size,
		DefaultBranch: giteaRepo.DefaultBranch,
	}
	return
}
