package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
)

type memberComponentImpl struct {
	memberStore   database.MemberStore
	orgStore      database.OrgStore
	userStore     database.UserStore
	gitServer     gitserver.GitServer
	gitMemberShip membership.GitMemerShip
	config        *config.Config
	sysMQ         mq.MessageQueue
}

type MemberComponent interface {
	OrgMembers(ctx context.Context, orgName, currentUser string, pageSize, page int) ([]types.Member, int, error)
	InitRoles(ctx context.Context, org *database.Organization) error
	SetAdmin(ctx context.Context, org *database.Organization, user *database.User) error
	ChangeMemberRole(ctx context.Context, orgName, userName, operatorName, oldRole, newRole string) error
	GetMemberRole(ctx context.Context, orgName, userName string) (membership.Role, error)
	AddMembers(ctx context.Context, orgName string, users []string, operatorName string, role string) error
	AddMember(ctx context.Context, orgName, userName, operatorName string, role string) error
	Delete(ctx context.Context, orgName, userName, operatorName string, role string) error
}

func NewMemberComponent(config *config.Config) (MemberComponent, error) {
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
	return &memberComponentImpl{
		memberStore:   database.NewMemberStore(),
		orgStore:      database.NewOrgStore(),
		userStore:     database.NewUserStore(),
		gitServer:     gs,
		gitMemberShip: gms,
		config:        config,
		sysMQ:         mq.SystemMQ,
	}, nil
}

func (c *memberComponentImpl) OrgMembers(ctx context.Context, orgName, currentUser string, pageSize, page int) ([]types.Member, int, error) {
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

func (c *memberComponentImpl) InitRoles(ctx context.Context, org *database.Organization) error {
	if c.config.GitServer.Type == types.GitServerTypeGitea {
		return c.gitMemberShip.AddRoles(ctx, org.Name,
			[]membership.Role{membership.RoleAdmin, membership.RoleRead, membership.RoleWrite})
	} else {
		return nil
	}
}

func (c *memberComponentImpl) SetAdmin(ctx context.Context, org *database.Organization, user *database.User) error {
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

func (c *memberComponentImpl) ChangeMemberRole(ctx context.Context, orgName, userName, operatorName, oldRole, newRole string) error {
	err := c.Delete(ctx, orgName, userName, operatorName, oldRole)
	if err != nil {
		return fmt.Errorf("failed to delete old role,error:%w", err)
	}
	err = c.AddMember(ctx, orgName, userName, operatorName, newRole)
	if err != nil {
		return fmt.Errorf("failed to create new role,error:%w", err)
	}

	if c.sysMQ != nil {

		org, err := c.orgStore.FindByPath(ctx, orgName)
		if err != nil {
			return fmt.Errorf("failed to find org,org:%s,caused by:%w", orgName, err)
		}
		userUUIDs, err := c.memberStore.UserUUIDsByOrganizationID(ctx, org.ID)
		if err != nil {
			return fmt.Errorf("failed to get uuids by orgid,error:%w", err)
		}

		title := "Organization member role change"
		content := fmt.Sprintf("Changed permission of member %s to %s in organization %s.", userName, newRole, orgName)
		go func(userUUIDs []string, orgName, title, content string) {
			err := c.sendMemberMsg(ctx, userUUIDs, orgName, title, content)
			if err != nil {
				slog.Error("failed to send organization permission change.", slog.String("orgName", orgName), slog.String("content", content), slog.Any("err", err))
			}
		}(userUUIDs, orgName, title, content)
	}

	return nil
}

func (c *memberComponentImpl) GetMemberRole(ctx context.Context, orgName, userName string) (membership.Role, error) {
	var (
		org  database.Organization
		user database.User
		err  error
	)
	org, err = c.orgStore.FindByPath(ctx, orgName)
	if err != nil {
		return membership.RoleUnknown, fmt.Errorf("failed to find org %s, caused by:%w", orgName, err)
	}
	user, err = c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return membership.RoleUnknown, fmt.Errorf("failed to find user %s, caused by:%w", userName, err)
	}
	m, err := c.memberStore.Find(ctx, org.ID, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return membership.RoleUnknown, fmt.Errorf("failed to check memberhsip existence, caused by:%w", err)
	}
	if m == nil {
		return membership.RoleUnknown, nil
	}
	return c.toGitRole(m.Role), nil
}

func (c *memberComponentImpl) AddMembers(ctx context.Context, orgName string, users []string, operatorName string, role string) error {
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
			return fmt.Errorf("failed to check memberhsip existence, user:%s,caused by:%w", userName, err)
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

		if c.sysMQ != nil {
			userUUIDs, err := c.memberStore.UserUUIDsByOrganizationID(ctx, org.ID)
			if err != nil {
				return fmt.Errorf("failed to get uuids by orgid,error:%w", err)
			}
			title := "Organization member change"
			content := fmt.Sprintf("New member %s joined organization %s.", userName, orgName)
			go func(userUUIDs []string, orgName, title, content string) {
				err = c.sendMemberMsg(ctx, userUUIDs, orgName, title, content)
				if err != nil {
					slog.Error("failed to send organization member join message", slog.String("orgName", orgName), slog.String("content", content), slog.Any("err", err))
				}
			}(userUUIDs, orgName, title, content)
		}

	}

	return nil
}

func (c *memberComponentImpl) AddMember(ctx context.Context, orgName, userName, operatorName string, role string) error {
	return c.AddMembers(ctx, orgName, []string{userName}, operatorName, role)
}

func (c *memberComponentImpl) Delete(ctx context.Context, orgName, userName, operatorName string, role string) error {
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
		return fmt.Errorf("failed to check memberhsip existence,caused by:%w", err)
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

	if c.sysMQ != nil {
		userUUIDs, err := c.memberStore.UserUUIDsByOrganizationID(ctx, org.ID)
		if err != nil {
			return fmt.Errorf("failed to get uuids by orgid,error:%w", err)
		}
		title := "Organization member change"
		content := fmt.Sprintf("%s left the organization %s.", userName, orgName)
		go func(userUUIDs []string, orgName, title, content string) {
			err = c.sendMemberMsg(ctx, userUUIDs, orgName, title, content)
			if err != nil {
				slog.Error("failed to send organization member leave message", slog.String("orgName", orgName), slog.String("userName", userName), slog.Any("err", err))
			}
		}(userUUIDs, orgName, title, content)
	}

	if c.config.GitServer.Type == types.GitServerTypeGitea {
		return c.gitMemberShip.RemoveMember(ctx, orgName, userName, c.toGitRole(role))
	} else {
		return nil
	}
}

func (c *memberComponentImpl) allowAddMember(u *database.Member) bool {
	//TODO: check more roles
	return u != nil && u.Role == string(membership.RoleAdmin)
}

func (c *memberComponentImpl) toGitRole(role string) membership.Role {
	switch role {
	case "admin":
		return membership.RoleAdmin
	case "write":
		return membership.RoleWrite
	case "read":
		return membership.RoleRead
	default:
		return membership.RoleUnknown
	}
}

func (c *memberComponentImpl) sendMemberMsg(ctx context.Context, userUUIDs []string, orgName, title, content string) error {
	url := fmt.Sprintf("/organizations/%s", orgName)
	msg := types.NotificationMessage{
		MsgUUID:          uuid.New().String(),
		UserUUIDs:        userUUIDs,
		NotificationType: types.NotificationOrganization,
		Title:            title,
		Content:          content,
		CreateAt:         time.Now(),
		ClickActionURL:   url,
	}
	err := c.sysMQ.PublishSiteInternalMsg(msg)
	if err != nil {
		return fmt.Errorf("failed to publish site msg, msg: %+v, err: %w", msg, err)
	}
	return nil
}
