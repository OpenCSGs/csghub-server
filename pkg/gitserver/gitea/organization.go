package gitea

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/pulltheflower/gitea-go-sdk/gitea"
)

func (c *Client) CreateOrganization(req *types.CreateOrgReq) (org *database.Organization, err error) {
	orgNames := []string{
		common.WithPrefix(req.Name, ModelOrgPrefix),
		common.WithPrefix(req.Name, DatasetOrgPrefix),
		common.WithPrefix(req.Name, SpaceOrgPrefix),
	}

	for _, orgName := range orgNames {
		_, _, err := c.giteaClient.AdminCreateOrg(
			req.User.Username,
			gitea.CreateOrgOption{
				Name:        orgName,
				Description: req.Description,
				FullName:    req.FullName,
			},
		)

		if err != nil {
			return nil, err
		}

	}

	org = &database.Organization{
		Path:        req.Name,
		Name:        req.FullName,
		Description: req.Description,
		User:        &req.User,
		UserID:      req.User.ID,
	}

	return
}

func (c *Client) DeleteOrganization(name string) (err error) {
	orgNames := []string{
		common.WithPrefix(name, ModelOrgPrefix),
		common.WithPrefix(name, DatasetOrgPrefix),
		common.WithPrefix(name, SpaceOrgPrefix),
	}

	for _, orgName := range orgNames {
		_, err = c.giteaClient.DeleteOrg(orgName)
		if err != nil {
			return
		}
	}

	return
}

func (c *Client) UpdateOrganization(name string, req *types.EditOrgReq, originOrg *database.Organization) (org *database.Organization, err error) {
	orgNames := []string{
		common.WithPrefix(name, ModelOrgPrefix),
		common.WithPrefix(name, DatasetOrgPrefix),
		common.WithPrefix(name, SpaceOrgPrefix),
	}

	for _, orgName := range orgNames {
		_, err = c.giteaClient.EditOrg(
			orgName,
			gitea.EditOrgOption{
				FullName:    req.FullName,
				Description: req.Description,
			},
		)
		if err != nil {
			return
		}
	}

	originOrg.Name = req.FullName
	originOrg.Description = req.Description

	return originOrg, nil
}
