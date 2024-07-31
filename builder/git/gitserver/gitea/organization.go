package gitea

import (
	"errors"
	"log/slog"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// FixOrganization recreate organization data, ignore data duplication error
// !should only be used for online data fixing
func (c *Client) FixOrganization(req *types.CreateOrgReq, user database.User) error {
	var errs error
	orgNames := c.getTargetOrgs(req.Name)
	for _, orgName := range orgNames {
		_, _, err := c.giteaClient.AdminCreateOrg(
			user.Username,
			gitea.CreateOrgOption{
				Name:        orgName,
				Description: req.Description,
				FullName:    req.Nickname,
			},
		)
		if err != nil {
			errs = errors.Join(err)
			slog.Error("fix gitea organization failed", slog.String("orgName", orgName), slog.String("user", user.Username),
				slog.String("error", err.Error()))
		} else {
			slog.Info("fix gitea organization success", slog.String("orgName", req.Name), slog.String("user", user.Username))
		}
	}

	return errs
}

func (c *Client) CreateOrganization(req *types.CreateOrgReq, user database.User) (org *database.Organization, err error) {
	orgNames := c.getTargetOrgs(req.Name)
	for _, orgName := range orgNames {
		_, _, err := c.giteaClient.AdminCreateOrg(
			user.Username,
			gitea.CreateOrgOption{
				Name:        orgName,
				Description: req.Description,
				FullName:    req.Nickname,
			},
		)
		if err != nil {
			slog.Error("create gitea organization failed", slog.String("orgName", orgName), slog.String("userName", user.Username))
			return nil, err
		}
	}

	org = &database.Organization{
		Name:        req.Name,
		Nickname:    req.Nickname,
		Description: req.Description,
		User:        &user,
		UserID:      user.ID,
	}

	return
}

func (c *Client) DeleteOrganization(name string) (err error) {
	orgNames := c.getTargetOrgs(name)
	for _, orgName := range orgNames {
		_, err = c.giteaClient.DeleteOrg(orgName)
		if err != nil {
			return
		}
	}

	return
}

// TODO:remove param `originOrg`
func (c *Client) UpdateOrganization(req *types.EditOrgReq, originOrg *database.Organization) (org *database.Organization, err error) {
	orgNames := c.getTargetOrgs(req.Name)

	for _, orgName := range orgNames {
		_, err = c.giteaClient.EditOrg(
			orgName,
			gitea.EditOrgOption{
				FullName:    *req.Nickname,
				Description: *req.Description,
			},
		)
		if err != nil {
			return
		}
	}

	return originOrg, nil
}

func (c *Client) getTargetOrgs(org string) []string {
	orgs := [4]string{
		common.WithPrefix(org, DatasetOrgPrefix),
		common.WithPrefix(org, ModelOrgPrefix),
		common.WithPrefix(org, SpaceOrgPrefix),
		common.WithPrefix(org, CodeOrgPrefix),
	}
	return orgs[:]
}
