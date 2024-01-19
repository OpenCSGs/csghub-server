package gitea

import (
	"context"
	"fmt"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/membership"
)

var _ membership.GitMemerShip = (*Client)(nil)

func (c *Client) AddMember(ctx context.Context, org, member string, role membership.Role) error {
	//teams are created automatically when create repo
	t, err := c.findTeam(org, role)
	if err != nil {
		return err
	}
	u, _, err := c.giteaClient.GetTeamMember(t.ID, member)
	if err != nil {
		return err
	}
	//silently success if user is already a member
	if u != nil {
		return nil
	}
	_, err = c.giteaClient.AddTeamMember(t.ID, member)
	return err
}

func (c *Client) RemoveMember(ctx context.Context, org, member string, role membership.Role) error {
	t, err := c.findTeam(org, role)
	if err != nil {
		return err
	}
	u, _, err := c.giteaClient.GetTeamMember(t.ID, member)
	if err != nil {
		return err
	}

	//silently success if user is not a member
	if u == nil {
		return nil
	}
	_, err = c.giteaClient.RemoveTeamMember(t.ID, member)
	return err
}

func (c *Client) IsRole(ctx context.Context, org, member string, role membership.Role) (bool, error) {
	return false, nil
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
		return nil, fmt.Errorf("failed to search org team,caused by:%w", err)
	}
	if len(teams) == 0 {
		return nil, fmt.Errorf("gitea team not found by role:%s", role)
	}

	t := teams[0]
	return t, nil
}

func roleToTeamName(role membership.Role) string {
	//TOOD:convert role to team name
	return string(role)
}
