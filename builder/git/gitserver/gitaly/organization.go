package gitaly

import (
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// FixOrganization recreate organization data, ignore data duplication error
// !should only be used for online data fixing
func (c *Client) FixOrganization(req *types.CreateOrgReq, user database.User) error {
	return nil
}

func (c *Client) CreateOrganization(req *types.CreateOrgReq, user database.User) (org *database.Organization, err error) {
	return &database.Organization{
		Name:        req.Name,
		Nickname:    req.Nickname,
		Description: req.Description,
		User:        &user,
		UserID:      user.ID,
	}, nil
}

func (c *Client) DeleteOrganization(name string) (err error) {
	return
}

func (c *Client) UpdateOrganization(req *types.EditOrgReq, originOrg *database.Organization) (org *database.Organization, err error) {
	return nil, nil
}
