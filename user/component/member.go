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
	"opencsg.com/csghub-server/builder/rebac"
	rebacfactory "opencsg.com/csghub-server/builder/rebac/factory"
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
	rebac                 rebac.Authorizer
}

type MemberComponent interface {
	OrgMembers(ctx context.Context, orgName, currentUser string, pageSize, page int) ([]types.Member, int, error)
	ChangeMemberRole(ctx context.Context, orgName, userName, operatorName, oldRole, newRole string) error
	GetMemberRole(ctx context.Context, orgName, userName string) (types.UserRole, error)
	AddMembers(ctx context.Context, orgName string, users []string, operatorName string, role string) error
	// Delete removes the organization membership identified by the organization and user names.
	Delete(ctx context.Context, orgName, userName, operatorName string) error
	GetMemberRoleByUUID(ctx context.Context, orgUUID, userName string) (types.UserRole, error)
}

func NewMemberComponent(config *config.Config) (MemberComponent, error) {
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server:%w", err)
	}
	notificationSvcClient := rpc.NewNotificationSvcHttpClient(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
		rpc.AuthWithApiKey(config.APIToken))
	authorizer, err := rebacfactory.NewAuthorizer()
	if err != nil {
		return nil, fmt.Errorf("create ReBAC authorizer: %w", err)
	}
	return &memberComponentImpl{
		memberStore:           database.NewMemberStore(),
		orgStore:              database.NewOrgStore(config),
		userStore:             database.NewUserStore(),
		gitServer:             gs,
		config:                config,
		notificationSvcClient: notificationSvcClient,
		rebac:                 authorizer,
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
			slog.ErrorContext(ctx, "failed to find member", "error", err)
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
			slog.WarnContext(ctx, "member user is nil, skip", "member", dbmember)
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
		_, adminCount, err := c.memberStore.OrganizationMembers(ctx, org.ID, string(types.UserAdmin), 1, 1)
		if err != nil {
			return fmt.Errorf("failed to count admins in org, caused by: %w", err)
		}
		if adminCount <= 1 && newRole != string(types.UserAdmin) {
			err := errors.New("cannot revoke the last admin role from organization")
			return errorx.LastOrgAdmin(err, errorx.Ctx().Set("username", userName))
		}
		if newRole == string(types.UserAdmin) && oldRole != string(types.UserAdmin) {
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

	memberRole := types.UserRole(newRole)
	if _, ok := memberRole.ReBACRelation(); !ok {
		return errorx.ReqParamInvalid(fmt.Errorf("unsupported organization role %q", memberRole), nil)
	}
	err = c.memberStore.Update(ctx, org.ID, user.ID, newRole)
	if err != nil {
		return fmt.Errorf("failed to update member role,caused by:%w", err)
	}
	if err := reconcileOrganizationMemberRelationships(ctx, c.rebac, org.UUID.String(), []string{user.UUID},
		desiredOrganizationMemberRoles([]string{user.UUID}, memberRole)); err != nil {
		return fmt.Errorf("sync updated organization member role to ReBAC: %w", err)
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
			slog.ErrorContext(notificationCtx, "failed to send organization permission change.", slog.String("orgName", orgName), slog.Any("err", err))
		}
	}()

	return nil
}

func (c *memberComponentImpl) GetMemberRole(ctx context.Context, orgName, userName string) (types.UserRole, error) {
	var (
		org  database.Organization
		user database.User
		err  error
	)
	org, err = c.orgStore.FindByPath(ctx, orgName)
	if err != nil {
		return "", fmt.Errorf("failed to find org %s, caused by:%w", orgName, err)
	}
	user, err = c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return "", fmt.Errorf("failed to find user %s, caused by:%w", userName, err)
	}
	m, err := c.memberStore.Find(ctx, org.ID, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to check membership existence, caused by:%w", err)
	}
	if errors.Is(err, sql.ErrNoRows) || m == nil {
		return "", nil
	}
	return types.UserRole(m.Role), nil
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
	memberRole := types.UserRole(role)
	if _, ok := memberRole.ReBACRelation(); !ok {
		return errorx.ReqParamInvalid(fmt.Errorf("unsupported organization role %q", memberRole), nil)
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
		// Reconcile existing memberships as well so retries can repair missing or stale tuples.
		if m != nil {
			existingRole := types.UserRole(m.Role)
			if err := reconcileOrganizationMemberRelationships(ctx, c.rebac, org.UUID.String(), []string{user.UUID},
				desiredOrganizationMemberRoles([]string{user.UUID}, existingRole)); err != nil {
				return fmt.Errorf("sync existing organization member to ReBAC: %w", err)
			}
			continue
		}
		err = c.memberStore.Add(ctx, org.ID, user.ID, role)
		if err != nil {
			err = fmt.Errorf("failed to create db member, org:%s, user:%s,caused by:%w", orgName, userName, err)
			return err
		}
		if err := reconcileOrganizationMemberRelationships(ctx, c.rebac, org.UUID.String(), []string{user.UUID},
			desiredOrganizationMemberRoles([]string{user.UUID}, memberRole)); err != nil {
			return fmt.Errorf("sync added organization member to ReBAC: %w", err)
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
					slog.ErrorContext(notificationCtx, "failed to send organization member join message", slog.String("orgName", orgName), slog.Any("err", err))
				}
			}(orgName, userName, role, userUUIDs)
		}
	}

	return nil
}

// Delete removes a user's organization membership after validating the operator's access.
func (c *memberComponentImpl) Delete(ctx context.Context, orgName, userName, operatorName string) error {
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
		if opMember.Role == string(types.UserAdmin) {
			_, adminCount, err := c.memberStore.OrganizationMembers(ctx, org.ID, string(types.UserAdmin), 1, 1)
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
	// Reconcile absent memberships as well so retries can remove stale tuples.
	if m == nil {
		if err := reconcileOrganizationMemberRelationships(ctx, c.rebac, org.UUID.String(), []string{user.UUID}, nil); err != nil {
			return fmt.Errorf("sync absent organization member to ReBAC: %w", err)
		}
		return nil
	}
	err = c.memberStore.Delete(ctx, org.ID, user.ID)
	if err != nil {
		err = fmt.Errorf("failed to delete member,caused by:%w", err)
		return err
	}
	if err := reconcileOrganizationMemberRelationships(ctx, c.rebac, org.UUID.String(), []string{user.UUID}, nil); err != nil {
		return fmt.Errorf("sync removed organization member to ReBAC: %w", err)
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
				slog.ErrorContext(notificationCtx, "failed to send organization member leave message", slog.String("orgName", orgName), slog.String("userName", userName), slog.Any("err", err))
			}
		}()
	}

	return nil
}

func (c *memberComponentImpl) allowMagnageMember(u *database.Member) bool {
	//TODO: check more roles
	return u != nil && u.Role == string(types.UserAdmin)
}

func (c *memberComponentImpl) GetMemberRoleByUUID(ctx context.Context, orgUUID, userName string) (types.UserRole, error) {
	org, err := c.orgStore.FindByUUID(ctx, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to find org by uuid,caused by:%w", err)
	}
	user, err := c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return "", fmt.Errorf("failed to find user,caused by:%w", err)
	}
	m, err := c.memberStore.Find(ctx, org.ID, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to find member:%w", err)
	}
	if errors.Is(err, sql.ErrNoRows) || m == nil {
		return "", nil
	}
	return types.UserRole(m.Role), nil
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
