package component

import (
	"context"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/builder/deploy/common"
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
}

func NewAgentCustomUIProxyComponent() AgentCustomUIProxyComponent {
	return &agentCustomUIProxyComponent{
		agentStore:  database.NewAgentInstanceStore(),
		spaceStore:  database.NewSpaceStore(),
		deployStore: database.NewDeployTaskStore(),
		shareStore:  database.NewAgentShareStore(),
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
	if space.Repository.Private {
		return nil, true, errorx.ErrForbidden
	}
	info := &types.AgentCustomUIProxyInfo{
		AgentContentID: agent.ContentID,
		AgentName:      agentName,
		ShareName:      share.ShareName,
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
