package gitea

import (
	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func (c *Client) CreateOrganization(req *types.CreateOrgReq, user database.User) (org *database.Organization, err error) {
	orgNames := []string{
		common.WithPrefix(req.Name, ModelOrgPrefix),
		common.WithPrefix(req.Name, DatasetOrgPrefix),
		common.WithPrefix(req.Name, SpaceOrgPrefix),
	}

	for _, orgName := range orgNames {
		_, _, err := c.giteaClient.AdminCreateOrg(
			user.Username,
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
		User:        &user,
		UserID:      user.ID,
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

func (c *Client) UpdateOrganization(req *types.EditOrgReq, originOrg *database.Organization) (org *database.Organization, err error) {
	orgNames := []string{
		common.WithPrefix(req.Path, ModelOrgPrefix),
		common.WithPrefix(req.Path, DatasetOrgPrefix),
		common.WithPrefix(req.Path, SpaceOrgPrefix),
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
