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

func (c *MemberComponent) Create(ctx context.Context, orgName, userName, operatorName string, role string) error {
	var (
		org  database.Organization
		op   database.User
		user database.User
		err  error
	)
	//get op user
	org, err = c.orgStore.FindByPath(ctx, orgName)
	if err != nil {
		return err
	}
	op, err = c.userStore.FindByUsername(ctx, operatorName)
	if err != nil {
		return err
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
		return err
	}
	m, err := c.memberStore.Find(ctx, org.ID, user.ID)
	if err != nil && err != sql.ErrNoRows {
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

func (c *MemberComponent) Delete(ctx context.Context) (err error) {
	return
}

func (c *MemberComponent) allowAddMember(u *database.Member) bool {
	//TODO: check more roles
	return u != nil && u.Role == "admin"
}

func (c *MemberComponent) toGitRole(role string) membership.Role {
	switch role {
	case "admin":
		return membership.RoleAdmin
	case "write":
		return membership.RoleWrite
	default:
		return membership.RoleRead
	}
}
