package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

type MemberComponent struct {
	memberStore   *database.MemberStore
	orgStore      *database.OrgStore
	userStore     *database.UserStore
	gitServer     gitserver.GitServer
	gitMemberShip membership.GitMemerShip
}

func NewMemberComponent(config *config.Config) (*MemberComponent, error) {
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server:%w", err)
	}
	gms, err := git.NewMemberShip(*config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git membership:%w", err)
	}
	return &MemberComponent{
		memberStore:   database.NewMemberStore(),
		orgStore:      database.NewOrgStore(),
		userStore:     database.NewUserStore(),
		gitServer:     gs,
		gitMemberShip: gms,
	}, nil
}

func (c *MemberComponent) Index(ctx context.Context) (members []database.Member, err error) {
	return
}

func (c *MemberComponent) InitRoles(ctx context.Context, org *database.Organization) error {
	return c.gitMemberShip.AddRoles(ctx, org.Name,
		[]membership.Role{membership.RoleAdmin, membership.RoleRead, membership.RoleWrite})
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
	return c.gitMemberShip.AddMember(ctx, org.Name, user.Username, membership.RoleAdmin)
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
		return "", fmt.Errorf("failed to find org,caused by:%w", err)
	}
	user, err = c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return "", fmt.Errorf("failed to find user,caused by:%w", err)
	}
	m, err := c.memberStore.Find(ctx, org.ID, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to check memberhsip existance,caused by:%w", err)
	}
	if m == nil {
		return membership.RoleUnkown, nil
	}
	return c.toGitRole(m.Role), nil
}

func (c *MemberComponent) AddMember(ctx context.Context, orgName, userName, operatorName string, role string) error {
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
	//return existing member
	if m != nil {
		return nil
	}
	err = c.memberStore.Add(ctx, org.ID, user.ID, role)
	if err != nil {
		err = fmt.Errorf("failed to create member,caused by:%w", err)
		return err
	}
	return c.gitMemberShip.AddMember(ctx, orgName, userName, c.toGitRole(role))
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
	return c.gitMemberShip.RemoveMember(ctx, orgName, userName, c.toGitRole(role))
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
