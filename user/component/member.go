package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type MemberComponent struct {
	memberStore   *database.MemberStore
	orgStore      *database.OrgStore
	userStore     *database.UserStore
	gitServer     gitserver.GitServer
	gitMemberShip membership.GitMemerShip
	config        *config.Config
}

func NewMemberComponent(config *config.Config) (*MemberComponent, error) {
	var gms membership.GitMemerShip
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server:%w", err)
	}
	if config.GitServer.Type == types.GitServerTypeGitea {
		gms, err = git.NewMemberShip(*config)
		if err != nil {
			return nil, fmt.Errorf("failed to create git membership:%w", err)
		}
	}
	return &MemberComponent{
		memberStore:   database.NewMemberStore(),
		orgStore:      database.NewOrgStore(),
		userStore:     database.NewUserStore(),
		gitServer:     gs,
		gitMemberShip: gms,
		config:        config,
	}, nil
}

func (c *MemberComponent) OrgMembers(ctx context.Context, orgName, currentUser string, pageSize, page int) ([]types.Member, int, error) {
	var (
		org  database.Organization
		user database.User
		err  error
	)
	org, err = c.orgStore.FindByPath(ctx, orgName)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find org,caused by:%w", err)
	}
	moreDetail := false
	user, err = c.userStore.FindByUsername(ctx, currentUser)
	if err == nil && user.ID > 0 {
		m, err := c.memberStore.Find(ctx, org.ID, user.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to find member", "error", err)
		}
		//if current user belongs to org, show more detail member info
		if m != nil {
			moreDetail = true
		}
	}

	dbmembers, total, err := c.memberStore.OrganizationMembers(ctx, org.ID, pageSize, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find org members,caused by:%w", err)
	}
	var members []types.Member
	for _, dbmember := range dbmembers {
		m := types.Member{
			UUID:     dbmember.User.UUID,
			Avatar:   dbmember.User.Avatar,
			Username: dbmember.User.Username,
			Nickname: dbmember.User.NickName,
		}
		if moreDetail {
			m.Role = dbmember.Role
			m.LastLoginAt = dbmember.User.LastLoginAt
		}
		members = append(members, m)
	}
	return members, total, nil
}

func (c *MemberComponent) InitRoles(ctx context.Context, org *database.Organization) error {
	if c.config.GitServer.Type == types.GitServerTypeGitea {
		return c.gitMemberShip.AddRoles(ctx, org.Name,
			[]membership.Role{membership.RoleAdmin, membership.RoleRead, membership.RoleWrite})
	} else {
		return nil
	}
}

func (c *MemberComponent) SetAdmin(ctx context.Context, org *database.Organization, user *database.User) error {
	var (
		err error
	)
	err = c.memberStore.Add(ctx, org.ID, user.ID, string(membership.RoleAdmin))
	if err != nil {
		err = fmt.Errorf("failed to create member,caused by:%w", err)
		return err
	}
	if c.config.GitServer.Type == types.GitServerTypeGitea {
		return c.gitMemberShip.AddMember(ctx, org.Name, user.Username, membership.RoleAdmin)
	} else {
		return nil
	}
}

func (c *MemberComponent) ChangeMemberRole(ctx context.Context, orgName, userName, operatorName, oldRole, newRole string) error {
	err := c.Delete(ctx, orgName, userName, operatorName, oldRole)
	if err != nil {
		return fmt.Errorf("failed to delete old role,error:%w", err)
	}
	err = c.AddMember(ctx, orgName, userName, operatorName, newRole)
	if err != nil {
		return fmt.Errorf("failed to create new role,error:%w", err)
	}

	return nil
}

func (c *MemberComponent) GetMemberRole(ctx context.Context, orgName, userName string) (membership.Role, error) {
	var (
		org  database.Organization
		user database.User
		err  error
	)
	org, err = c.orgStore.FindByPath(ctx, orgName)
	if err != nil {
		return membership.RoleUnkown, fmt.Errorf("failed to find org,caused by:%w", err)
	}
	user, err = c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return membership.RoleUnkown, fmt.Errorf("failed to find user,caused by:%w", err)
	}
	m, err := c.memberStore.Find(ctx, org.ID, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return membership.RoleUnkown, fmt.Errorf("failed to check memberhsip existance,caused by:%w", err)
	}
	if m == nil {
		return membership.RoleUnkown, nil
	}
	return c.toGitRole(m.Role), nil
}

func (c *MemberComponent) AddMembers(ctx context.Context, orgName string, users []string, operatorName string, role string) error {
	var (
		org  database.Organization
		op   database.User
		user database.User
		err  error
	)
	org, err = c.orgStore.FindByPath(ctx, orgName)
	if err != nil {
		return fmt.Errorf("failed to find org,org:%s,caused by:%w", orgName, err)
	}
	op, err = c.userStore.FindByUsername(ctx, operatorName)
	if err != nil {
		return fmt.Errorf("failed to find op user,user:%s,caused by:%w", operatorName, err)
	}
	opMember, err := c.memberStore.Find(ctx, org.ID, op.ID)
	if err != nil {
		return fmt.Errorf("failed to get op user membership,user:%s,caused by:%w", operatorName, err)
	}
	if !c.allowAddMember(opMember) {
		return fmt.Errorf("add member operation not allowed, user:%s", operatorName)
	}

	for _, userName := range users {
		user, err = c.userStore.FindByUsername(ctx, userName)
		if err != nil {
			return fmt.Errorf("failed to find user, user:%s,caused by:%w", userName, err)
		}
		m, err := c.memberStore.Find(ctx, org.ID, user.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to check memberhsip existance, user:%s,caused by:%w", userName, err)
		}
		//skip existing member
		if m != nil {
			continue
		}
		err = c.memberStore.Add(ctx, org.ID, user.ID, role)
		if err != nil {
			err = fmt.Errorf("failed to create db member, org:%s, user:%s,caused by:%w", orgName, userName, err)
			return err
		}
		if c.config.GitServer.Type == types.GitServerTypeGitea {
			err = c.gitMemberShip.AddMember(ctx, orgName, userName, c.toGitRole(role))
			if err != nil {
				return fmt.Errorf("failed to add git member, org:%s, user:%s caused by:%w", orgName, userName, err)
			}
		}
	}

	return nil
}

func (c *MemberComponent) AddMember(ctx context.Context, orgName, userName, operatorName string, role string) error {
	return c.AddMembers(ctx, orgName, []string{userName}, operatorName, role)
}

func (c *MemberComponent) Update(ctx context.Context) (org *database.Member, err error) {
	return
}

func (c *MemberComponent) Delete(ctx context.Context, orgName, userName, operatorName string, role string) error {
	var (
		org  database.Organization
		op   database.User
		user database.User
		err  error
	)
	org, err = c.orgStore.FindByPath(ctx, orgName)
	if err != nil {
		return fmt.Errorf("failed to find org,caused by:%w", err)
	}
	op, err = c.userStore.FindByUsername(ctx, operatorName)
	if err != nil {
		return fmt.Errorf("failed to find user,caused by:%w", err)
	}
	opMember, err := c.memberStore.Find(ctx, org.ID, op.ID)
	if err != nil {
		return fmt.Errorf("failed to get op user membership,caused by:%w", err)
	}
	if !c.allowAddMember(opMember) {
		return errors.New("operation not allowed")
	}
	user, err = c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return fmt.Errorf("failed to find user,caused by:%w", err)
	}
	m, err := c.memberStore.Find(ctx, org.ID, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check memberhsip existance,caused by:%w", err)
	}
	//skip if not a member
	if m == nil {
		return nil
	}
	err = c.memberStore.Delete(ctx, org.ID, user.ID, role)
	if err != nil {
		err = fmt.Errorf("failed to delete member,caused by:%w", err)
		return err
	}
	if c.config.GitServer.Type == types.GitServerTypeGitea {
		return c.gitMemberShip.RemoveMember(ctx, orgName, userName, c.toGitRole(role))
	} else {
		return nil
	}
}

func (c *MemberComponent) allowAddMember(u *database.Member) bool {
	//TODO: check more roles
	return u != nil && u.Role == string(membership.RoleAdmin)
}

func (c *MemberComponent) toGitRole(role string) membership.Role {
	switch role {
	case "admin":
		return membership.RoleAdmin
	case "write":
		return membership.RoleWrite
	case "read":
		return membership.RoleRead
	default:
		return membership.RoleUnkown
	}
}
