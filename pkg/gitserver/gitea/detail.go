package gitea

import "git-devops.opencsg.com/product/community/starhub-server/pkg/types"

func (c *Client) GetModelDetail(namespace, name string) (model *types.ModelDetail, err error) {
	giteaRepo, _, err := c.giteaClient.GetRepo(namespace, name)
	if err != nil {
		return
	}
	model = &types.ModelDetail{
		Path:          giteaRepo.FullName,
		Name:          giteaRepo.Name,
		Introduction:  giteaRepo.Description,
		License:       "",
		DownloadCount: 0,
		LastUpdatedAt: giteaRepo.Updated.Format("2006-01-02 15:04:05"),
		HTTPCloneURL:  giteaRepo.CloneURL,
		SSHCloneURL:   giteaRepo.SSHURL,
		Size:          giteaRepo.Size,
		DefaultBranch: giteaRepo.DefaultBranch,
	}
	return
}

func (c *Client) GetDatasetDetail(namespace, name string) (model *types.DatasetDetail, err error) {
	giteaRepo, _, err := c.giteaClient.GetRepo(namespace, name)
	if err != nil {
		return
	}
	model = &types.DatasetDetail{
		Path:          giteaRepo.FullName,
		Name:          giteaRepo.Name,
		Introduction:  giteaRepo.Description,
		License:       "",
		DownloadCount: 0,
		LastUpdatedAt: giteaRepo.Updated.Format("2006-01-02 15:04:05"),
		HTTPCloneURL:  giteaRepo.CloneURL,
		SSHCloneURL:   giteaRepo.SSHURL,
		Size:          giteaRepo.Size,
		DefaultBranch: giteaRepo.DefaultBranch,
	}
	return
}
