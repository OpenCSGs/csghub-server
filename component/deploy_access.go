package component

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// deploy access control for dedicated instances (issue csghub-portal#3416):
//   - personal instance: only the owner may view or operate it
//   - organization instance: every member may view it; only members with the
//     write permission (write or admin) may start/stop/delete or switch its
//     public visibility
//   - system administrators have no implicit access to other users' instances

// checkDeployReadAccess returns the deploy when currentUser may view it.
// Legacy deploys without owner_namespace keep the creator/same-org fallback:
// space deploys derive visibility from the repo, other legacy rows stay
// creator-only.
func (c *repoComponentImpl) checkDeployReadAccess(ctx context.Context, currentUser string, deploy *database.Deploy) (*database.User, *database.Deploy, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, nil, fmt.Errorf("deploy access check user failed, %w", err)
	}
	if deploy.UserID == user.ID {
		return &user, deploy, nil
	}
	if deploy.OwnerNamespace == "" {
		if deploy.SpaceID > 0 && c.IsInSameOrg(ctx, user.ID, deploy.UserID) {
			return &user, deploy, nil
		}
		return nil, nil, errorx.ErrForbiddenMsg("deploy was not created by user")
	}
	allowed, err := c.checkDeployNamespacePermission(ctx, currentUser, user, deploy, rebac.NamespaceCanRead)
	slog.InfoContext(ctx, "check deploy read access",
		slog.String("namespace", deploy.OwnerNamespace),
		slog.Any("currentUser", currentUser),
		slog.Bool("allowed", allowed), slog.Any("err", err))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check namespace read permission, %w", err)
	}
	if !allowed {
		return nil, nil, errorx.ErrForbiddenMsg("deploy was not created by user")
	}
	return &user, deploy, nil
}

// checkDeployOperateAccess returns the deploy when currentUser may operate it
// (start/stop/delete/public switch). Organization instances require the write
// permission; personal instances stay owner-only.
func (c *repoComponentImpl) checkDeployOperateAccess(ctx context.Context, currentUser string, deploy *database.Deploy) (*database.User, *database.Deploy, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, nil, fmt.Errorf("deploy access check user failed, %w", err)
	}
	if deploy.UserID == user.ID {
		return &user, deploy, nil
	}
	if deploy.OwnerNamespace == "" {
		return nil, nil, errorx.ErrForbiddenMsg("deploy was not created by user")
	}
	allowed, err := c.checkDeployNamespacePermission(ctx, currentUser, user, deploy, rebac.NamespaceCanWrite)
	slog.InfoContext(ctx, "check deploy can write access",
		slog.String("namespace", deploy.OwnerNamespace),
		slog.Any("currentUser", currentUser),
		slog.Bool("allowed", allowed), slog.Any("err", err))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check namespace write permission, %w", err)
	}
	if !allowed {
		return nil, nil, errorx.ErrForbiddenMsg("deploy was not created by user")
	}
	return &user, deploy, nil
}

// checkDeployNamespacePermission resolves deploy access against ReBAC directly.
// Private inference instances must not inherit the platform administrator
// bypass in CheckCurrentUserPermission.
func (c *repoComponentImpl) checkDeployNamespacePermission(
	ctx context.Context,
	currentUser string,
	user database.User,
	deploy *database.Deploy,
	permission rebac.Permission,
) (bool, error) {
	if deploy.Type != types.InferenceType || deploy.SecureLevel != types.EndpointPrivate {
		return c.CheckCurrentUserPermission(ctx, currentUser, deploy.OwnerNamespace, permission)
	}

	ns, err := c.namespaceStore.FindByPath(ctx, deploy.OwnerNamespace)
	if err != nil {
		return false, fmt.Errorf("find namespace %q for deploy permission: %w", deploy.OwnerNamespace, err)
	}
	decision, err := c.rebac.Check(ctx, rebac.CheckRequest{
		Subject:  rebac.UserSubject(user.UUID),
		Relation: permission,
		Object:   rebac.NamespaceObject(ns.UUID),
	})
	if err != nil {
		return false, fmt.Errorf("check namespace permission for deploy: %w", err)
	}
	return decision.Allowed, nil
}

// getDeployForAccess loads the deploy and verifies the requested access mode.
func (c *repoComponentImpl) getDeployForAccess(ctx context.Context, currentUser string, deployID int64, mode types.DeployAccessMode) (*database.User, *database.Deploy, error) {
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user deploy %v, %w", deployID, err)
	}
	if deploy == nil {
		return nil, nil, fmt.Errorf("do not found user deploy %v", deployID)
	}
	switch mode {
	case types.DeployAccessOperate:
		return c.checkDeployOperateAccess(ctx, currentUser, deploy)
	default:
		return c.checkDeployReadAccess(ctx, currentUser, deploy)
	}
}

// CheckDeployReadAccess verifies that currentUser may view the deploy.
func (c *repoComponentImpl) CheckDeployReadAccess(ctx context.Context, deployReq types.DeployActReq) (*database.User, *database.Deploy, error) {
	return c.getDeployForAccess(ctx, deployReq.CurrentUser, deployReq.DeployID, types.DeployAccessRead)
}

// CheckDeployOperateAccess verifies that currentUser may operate the deploy
// (start/stop/delete/public switch).
func (c *repoComponentImpl) CheckDeployOperateAccess(ctx context.Context, deployReq types.DeployActReq) (*database.User, *database.Deploy, error) {
	return c.getDeployForAccess(ctx, deployReq.CurrentUser, deployReq.DeployID, types.DeployAccessOperate)
}
