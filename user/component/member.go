package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/common/errorx"

	"github.com/google/uuid"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type memberComponentImpl struct {
	memberStore           database.MemberStore
	orgStore              database.OrgStore
	userStore             database.UserStore
	gitServer             gitserver.GitServer
	config                *config.Config
	notificationSvcClient rpc.NotificationSvcClient
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
	GetMember(ctx context.Context, orgName, userName string) (*database.Member, error)
}

func NewMemberComponent(config *config.Config) (MemberComponent, error) {
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server:%w", err)
	}
	notificationSvcClient := rpc.NewNotificationSvcHttpClient(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
		rpc.AuthWithApiKey(config.APIToken))
	return &memberComponentImpl{
		memberStore:           database.NewMemberStore(),
		orgStore:              database.NewOrgStore(),
		userStore:             database.NewUserStore(),
		gitServer:             gs,
		config:                config,
		notificationSvcClient: notificationSvcClient,
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

	dbmembers, total, err := c.memberStore.OrganizationMembers(ctx, org.ID, "", pageSize, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find org members,caused by:%w", err)
	}
	var members []types.Member
	for _, dbmember := range dbmembers {
		if dbmember.User == nil {
			slog.Warn("member user is nil, skip", "member", dbmember)
			continue
		}
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
	return nil
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
	return nil
}

func (c *memberComponentImpl) ChangeMemberRole(ctx context.Context, orgName, userName, operatorName, oldRole, newRole string) error {
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
	user, err = c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return fmt.Errorf("failed to find user,user:%s,caused by:%w", userName, err)
	}

	if op.ID == user.ID {
		_, adminCount, err := c.memberStore.OrganizationMembers(ctx, org.ID, string(membership.RoleAdmin), 1, 1)
		if err != nil {
			return fmt.Errorf("failed to count admins in org, caused by: %w", err)
		}
		if adminCount <= 1 && newRole != string(membership.RoleAdmin) {
			err := errors.New("cannot revoke the last admin role from organization")
			return errorx.LastOrgAdmin(err, errorx.Ctx().Set("username", userName))
		}
		if newRole == string(membership.RoleAdmin) && oldRole != string(membership.RoleAdmin) {
			err := errors.New("cannot promote yourself to admin")
			return errorx.CannotPromoteSelfToAdmin(err, errorx.Ctx().Set("username", userName))
		}
	} else {
		if !c.allowMagnageMember(opMember) {
			return errorx.ErrForbiddenMsg("operation not allowed, you do not have permission to change the role of other members")
		}
	}

	m, err := c.memberStore.Find(ctx, org.ID, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check membership existence,caused by:%w", err)
	}
	if m == nil {
		return fmt.Errorf("user %s is not a member of organization %s", userName, orgName)
	}

	err = c.memberStore.Update(ctx, org.ID, user.ID, newRole)
	if err != nil {
		return fmt.Errorf("failed to update member role,caused by:%w", err)
	}

	userUUIDs, err := c.memberStore.UserUUIDsByOrganizationID(ctx, org.ID)
	if err != nil {
		return fmt.Errorf("failed to get uuids by orgid,error:%w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = c.sendMemberMsg(notificationCtx, types.OrgMemberReq{
			UserUUIDs: userUUIDs,
			OrgName:   orgName,
			Operation: types.OrgMemberOperationUpdate,
			UserName:  userName,
			NewRole:   newRole,
		})
		if err != nil {
			slog.Error("failed to send organization permission change.", slog.String("orgName", orgName), slog.Any("err", err))
		}
	}()

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
		return membership.RoleUnknown, fmt.Errorf("failed to check membership existence, caused by:%w", err)
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
	if !c.allowMagnageMember(opMember) {
		return errorx.ErrForbiddenMsg(fmt.Sprintf("add member operation not allowed, user:%s", operatorName))
	}

	for _, userName := range users {
		user, err = c.userStore.FindByUsername(ctx, userName)
		if err != nil {
			return fmt.Errorf("failed to find user, user:%s,caused by:%w", userName, err)
		}
		m, err := c.memberStore.Find(ctx, org.ID, user.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to check membership existence, user:%s,caused by:%w", userName, err)
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

		userUUIDs, err := c.memberStore.UserUUIDsByOrganizationID(ctx, org.ID)
		if err != nil {
			return fmt.Errorf("failed to get uuids by orgid,error:%w", err)
		}

		if len(userUUIDs) > 0 {
			go func(orgName, userName, role string, userUUIDs []string) {
				notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				err = c.sendMemberMsg(notificationCtx, types.OrgMemberReq{
					UserUUIDs: userUUIDs,
					OrgName:   orgName,
					Operation: types.OrgMemberOperationAdd,
					UserName:  userName,
				})
				if err != nil {
					slog.Error("failed to send organization member join message", slog.String("orgName", orgName), slog.Any("err", err))
				}
			}(orgName, userName, role, userUUIDs)
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
	user, err = c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return fmt.Errorf("failed to find user,caused by:%w", err)
	}

	// can't remove the last member of this organization
	_, total, err := c.memberStore.OrganizationMembers(ctx, org.ID, "", 1, 1)
	if err != nil {
		return fmt.Errorf("failed to find org members, caused by:%w", err)
	}

	if total == 0 {
		err := fmt.Errorf("no member in organization %s", org.Name)
		return errorx.ReqParamInvalid(err,
			errorx.Ctx().
				Set("namespace", org.Namespace),
		)
	}

	if op.ID == user.ID {
		// admin delete itself
		if opMember.Role == string(membership.RoleAdmin) {
			_, adminCount, err := c.memberStore.OrganizationMembers(ctx, org.ID, "admin", 1, 1)
			if err != nil {
				return fmt.Errorf("failed to count admins in org, caused by: %w", err)
			}
			if adminCount <= 1 {
				// only one admin, refused delete
				err := errors.New("cannot remove the last admin from organization")
				return errorx.LastOrgAdmin(err, errorx.Ctx().Set("username", userName))
			}
		}
	} else {
		// only admin can delete others
		if !c.allowMagnageMember(opMember) {
			return errorx.ErrForbiddenMsg("operation not allowed, you do not have permission to remove other members")
		}
	}

	m, err := c.memberStore.Find(ctx, org.ID, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check membership existence,caused by:%w", err)
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
	userUUIDs, err := c.memberStore.UserUUIDsByOrganizationID(ctx, org.ID)
	if err != nil {
		return fmt.Errorf("failed to get uuids by orgid,error:%w", err)
	}

	if len(userUUIDs) > 0 {
		go func() {
			notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err = c.sendMemberMsg(notificationCtx, types.OrgMemberReq{
				UserUUIDs: userUUIDs,
				OrgName:   orgName,
				Operation: types.OrgMemberOperationRemove,
				UserName:  userName,
			})
			if err != nil {
				slog.Error("failed to send organization member leave message", slog.String("orgName", orgName), slog.String("userName", userName), slog.Any("err", err))
			}
		}()
	}

	return nil
}

func (c *memberComponentImpl) allowMagnageMember(u *database.Member) bool {
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

func (c *memberComponentImpl) GetMember(ctx context.Context, orgName, userName string) (*database.Member, error) {
	org, err := c.orgStore.FindByPath(ctx, orgName)
	if err != nil {
		return nil, fmt.Errorf("failed to find org,caused by:%w", err)
	}
	user, err := c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to find user,caused by:%w", err)
	}
	m, err := c.memberStore.Find(ctx, org.ID, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find member:%w", err)
	}
	return m, err
}

func (c *memberComponentImpl) sendMemberMsg(ctx context.Context, req types.OrgMemberReq) error {
	msg := types.NotificationMessage{
		MsgUUID:          uuid.New().String(),
		UserUUIDs:        req.UserUUIDs,
		NotificationType: types.NotificationOrganization,
		CreateAt:         time.Now(),
		ClickActionURL:   fmt.Sprintf("/organizations/%s", req.OrgName),
		Template:         string(types.MessageScenarioOrgMember),
		Payload: map[string]any{
			"operation": req.Operation,
			"user_name": req.UserName,
			"new_role":  req.NewRole,
			"org_name":  req.OrgName,
		},
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message, err: %w", err)
	}
	notificationMsg := types.MessageRequest{
		Scenario:   types.MessageScenarioOrgMember,
		Parameters: string(msgBytes),
		Priority:   types.MessagePriorityHigh,
	}

	var sendErr error
	retryCount := c.config.Notification.NotificationRetryCount
	for i := range retryCount {
		if sendErr = c.notificationSvcClient.Send(ctx, &notificationMsg); sendErr == nil {
			break
		}
		if i < retryCount-1 {
			slog.Warn("failed to send notification, retrying", "notification_msg", notificationMsg, "attempt", i+1, "error", sendErr.Error())
		}
	}
	if sendErr != nil {
		return fmt.Errorf("failed to send notification after %d attempts, err: %w", retryCount, sendErr)
	}

	return nil
}
