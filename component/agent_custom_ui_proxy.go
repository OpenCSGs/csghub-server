package component

import (
	"context"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AgentCustomUIProxyComponent interface {
	Resolve(ctx context.Context, agentContentID string) (*types.AgentCustomUIProxyInfo, bool, error)
}

type agentCustomUIProxyComponent struct {
	agentStore  database.AgentInstanceStore
	spaceStore  database.SpaceStore
	deployStore database.DeployTaskStore
	shareStore  database.AgentShareStore
	userStore   database.UserStore
	repoComp    RepoComponent
}

func NewAgentCustomUIProxyComponent(repoComp RepoComponent) AgentCustomUIProxyComponent {
	return &agentCustomUIProxyComponent{
		agentStore:  database.NewAgentInstanceStore(),
		spaceStore:  database.NewSpaceStore(),
		deployStore: database.NewDeployTaskStore(),
		shareStore:  database.NewAgentShareStore(),
		userStore:   database.NewUserStore(),
		repoComp:    repoComp,
	}
}

func (c *agentCustomUIProxyComponent) Resolve(ctx context.Context, agentContentID string) (*types.AgentCustomUIProxyInfo, bool, error) {
	agent, err := c.agentStore.FindByContentID(ctx, "csgclaw", agentContentID)
	if err != nil {
		if errors.Is(err, errorx.ErrDatabaseNoRows) {
			return nil, false, nil
		}
		return nil, true, fmt.Errorf("failed to resolve CSGClaw agent hostname: %w", err)
	}
	if agent == nil {
		return nil, false, nil
	}
	enabled, err := types.CSGClawHasCustomUI(&agent.Metadata)
	if err != nil {
		return nil, true, err
	}
	if !enabled {
		return nil, false, nil
	}
	if !agent.Public {
		return nil, true, errorx.ErrForbidden
	}
	agentName, err := types.CSGClawAgentName(&agent.Metadata)
	if err != nil {
		if errors.Is(err, types.ErrCSGClawAgentNameInvalid) {
			return nil, true, errorx.ErrCSGClawAgentNameInvalid
		}
		return nil, true, fmt.Errorf("invalid CSGClaw agent name: %w", err)
	}
	spaceID, err := types.CSGClawCustomUISpaceID(&agent.Metadata)
	if err != nil {
		return nil, true, fmt.Errorf("invalid custom UI binding: %w", err)
	}
	share, err := c.shareStore.FindByShareName(ctx, types.CSGClawAutomaticShareName(agent.ID))
	if err != nil || share == nil || share.InstanceID != agent.ID {
		if err == nil {
			return nil, true, fmt.Errorf("automatic agent share unavailable")
		}
		return nil, true, fmt.Errorf("automatic agent share unavailable: %w", err)
	}
	space, err := c.spaceStore.ByID(ctx, spaceID)
	if err != nil {
		return nil, true, fmt.Errorf("failed to resolve custom UI Space: %w", err)
	}
	if space == nil || space.Repository == nil {
		return nil, true, errorx.ErrNotFound
	}
	// A private bound space is allowed: the public agent's binding authorizes
	// anonymous access to the space's deployment through the agent's custom
	// domain. The space's own endpoint stays access-controlled via the normal
	// RProxy path. The binding is resolved on every request, so unbinding or
	// making the agent private (rejected above) cuts access immediately.
	if space.Repository.Private {
		// Fail closed when the agent owner no longer has write access to the
		// private bound space (e.g. org membership revoked after binding):
		// binding is a publish action, and losing the permission that
		// authorized the publish must cut the anonymous exposure.
		owner, err := c.userStore.FindByUUID(ctx, agent.UserUUID)
		if err != nil {
			return nil, true, fmt.Errorf("failed to resolve agent owner for custom UI permission check: %w", err)
		}
		canWrite, err := c.repoComp.CheckUserRepoPermission(ctx, owner.Username, space.Repository, rebac.RepositoryCanWrite)
		if err != nil {
			return nil, true, fmt.Errorf("failed to recheck custom UI Space write access: %w", err)
		}
		if !canWrite {
			return nil, true, errorx.ErrForbidden
		}
	}
	info := &types.AgentCustomUIProxyInfo{
		AgentContentID: agent.ContentID,
		AgentName:      agentName,
		ShareName:      share.ShareName,
		Kind:           types.AgentInstanceKind(agent.Kind),
		SpaceID:        spaceID,
	}
	deploy, err := c.deployStore.GetLatestDeployBySpaceID(ctx, spaceID)
	if err != nil {
		return nil, true, fmt.Errorf("failed to resolve custom UI Space deployment: %w", err)
	}
	if deploy == nil || deploy.Status != common.Running || deploy.SvcName == "" {
		return info, true, nil
	}
	info.SpaceSvcName = deploy.SvcName
	info.SpaceEndpoint = deploy.Endpoint
	info.SpaceClusterID = deploy.ClusterID
	info.Available = true
	return info, true, nil
}
