package gitea

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/common/utils/common"
)

var _ membership.GitMemerShip = (*Client)(nil)

func (c *Client) AddRoles(ctx context.Context, org string, roles []membership.Role) error {
	var errs error
	for _, to := range c.getTargetOrgs(org) {
		errs = errors.Join(errs,
			c.addRoles(ctx, to, roles))
	}
	return errs
}

func (c *Client) addRoles(ctx context.Context, org string, roles []membership.Role) error {
	var errs error
	for _, role := range roles {
		opt := c.getTeamOptByRole(role)
		_, resp, err := c.giteaClient.CreateTeam(org, opt)
		if err != nil {
			slog.Error("gitea create team failed", slog.String("org", org), slog.Any("body", resp.Body),
				slog.Int("code", resp.StatusCode), slog.String("error", err.Error()))
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (c *Client) getTeamOptByRole(role membership.Role) gitea.CreateTeamOption {
	var opt gitea.CreateTeamOption
	opt.Name = string(role)
	opt.IncludesAllRepositories = true
	opt.Units = append(opt.Units, gitea.RepoUnitCode)
	switch role {
	case membership.RoleAdmin:
		opt.CanCreateOrgRepo = true
		opt.Permission = gitea.AccessModeAdmin
	case membership.RoleWrite:
		opt.CanCreateOrgRepo = false
		opt.Permission = gitea.AccessModeWrite
	case membership.RoleRead:
		opt.CanCreateOrgRepo = false
		opt.Permission = gitea.AccessModeRead
	}
	return opt
}

func (c *Client) AddMember(ctx context.Context, org, member string, role membership.Role) error {
	var err error
	for _, to := range c.getTargetOrgs(org) {
		err = errors.Join(err,
			c.addMember(ctx, to, member, role))
	}

	return err
}

func (c *Client) addMember(ctx context.Context, org, member string, role membership.Role) error {
	// teams are created automatically when create repo
	t, err := c.findTeam(org, role)
	if err != nil {
		slog.ErrorContext(ctx, "fail to get team from gitea", slog.Any("err", err))
		return err
	}
	_, resp, err := c.giteaClient.GetTeamMember(t.ID, member)
	// silently success if user is already a member
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	// if not a member
	if resp.StatusCode == http.StatusNotFound {
		_, err = c.giteaClient.AddTeamMember(t.ID, member)
		if err != nil {
			slog.ErrorContext(ctx, "fail to add team member to gitea", slog.Any("err", err))
		}
		return err
	}
	// unkown server error happend
	slog.ErrorContext(ctx, "fail to get team member from gitea", slog.Any("err", err))
	return err
}

func (c *Client) RemoveMember(ctx context.Context, org, member string, role membership.Role) error {
	var err error
	for _, to := range c.getTargetOrgs(org) {
		err = errors.Join(err,
			c.removeMember(ctx, to, member, role))
	}
	return err
}

func (c *Client) removeMember(ctx context.Context, org, member string, role membership.Role) error {
	t, err := c.findTeam(org, role)
	if err != nil {
		return err
	}
	u, _, err := c.giteaClient.GetTeamMember(t.ID, member)
	if err != nil {
		return err
	}

	// silently success if user is not a member
	if u == nil {
		return nil
	}
	_, err = c.giteaClient.RemoveTeamMember(t.ID, member)
	return err
}

func (c *Client) IsRole(ctx context.Context, org, member string, role membership.Role) (bool, error) {
	return false, nil
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

func (c *Client) findTeam(org string, role membership.Role) (*gitea.Team, error) {
	opt := &gitea.SearchTeamsOptions{
		Query: roleToTeamName(role),
		ListOptions: gitea.ListOptions{
			PageSize: 1,
		},
		IncludeDescription: false,
	}
	teams, _, err := c.giteaClient.SearchOrgTeams(org, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to search org team, error:%w", err)
	}
	if len(teams) == 0 {
		return nil, fmt.Errorf("gitea team not found by role:%s", role)
	}

	t := teams[0]
	return t, nil
}

func roleToTeamName(role membership.Role) string {
	return string(role)
}
