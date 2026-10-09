package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	deploycommon "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestAgentCustomUIProxyComponent_Resolve(t *testing.T) {
	ctx := context.Background()
	agents := mockdatabase.NewMockAgentInstanceStore(t)
	spaces := mockdatabase.NewMockSpaceStore(t)
	deploys := mockdatabase.NewMockDeployTaskStore(t)
	shares := mockdatabase.NewMockAgentShareStore(t)
	comp := &agentCustomUIProxyComponent{agentStore: agents, spaceStore: spaces, deployStore: deploys, shareStore: shares}
	agent := &database.AgentInstance{
		ID: 12, Type: "csgclaw", ContentID: "agent-host", Public: true,
		Metadata: map[string]any{
			"provision_request": map[string]any{"custom_ui_space_id": float64(7)},
			"template_metadata": map[string]any{"agent_file": map[string]any{"name": "agent-name"}},
		},
	}
	agents.EXPECT().FindByContentID(ctx, "csgclaw", "agent-host").Return(agent, nil)
	shares.EXPECT().FindByShareName(ctx, types.CSGClawAutomaticShareName(12)).Return(&database.AgentShare{InstanceID: 12, ShareName: "s-a-12"}, nil)
	spaces.EXPECT().ByID(ctx, int64(7)).Return(&database.Space{Repository: &database.Repository{Private: false}}, nil)
	deploys.EXPECT().GetLatestDeployBySpaceID(ctx, int64(7)).Return(&database.Deploy{Status: deploycommon.Running, SvcName: "ui-svc", Endpoint: "https://ui.example", ClusterID: "cluster-1"}, nil)

	info, matched, err := comp.Resolve(ctx, "agent-host")
	require.NoError(t, err)
	require.True(t, matched)
	require.Equal(t, &types.AgentCustomUIProxyInfo{
		AgentContentID: "agent-host", AgentName: "agent-name", ShareName: "s-a-12", SpaceID: 7,
		SpaceSvcName: "ui-svc", SpaceEndpoint: "https://ui.example", SpaceClusterID: "cluster-1", Available: true,
	}, info)
}

func TestAgentCustomUIProxyComponent_ResolvePrivateAgent(t *testing.T) {
	ctx := context.Background()
	agents := mockdatabase.NewMockAgentInstanceStore(t)
	agent := &database.AgentInstance{ID: 12, Type: "csgclaw", ContentID: "agent-host", Public: false, Metadata: map[string]any{"provision_request": map[string]any{"custom_ui_space_id": float64(7)}}}
	agents.EXPECT().FindByContentID(ctx, "csgclaw", "agent-host").Return(agent, nil)
	comp := &agentCustomUIProxyComponent{agentStore: agents}

	info, matched, err := comp.Resolve(ctx, "agent-host")
	require.ErrorIs(t, err, errorx.ErrForbidden)
	require.True(t, matched)
	require.Nil(t, info)
}

func TestAgentCustomUIProxyComponent_ResolveInvalidAgentName(t *testing.T) {
	ctx := context.Background()
	agents := mockdatabase.NewMockAgentInstanceStore(t)
	agent := &database.AgentInstance{
		ID: 12, Type: "csgclaw", ContentID: "agent-host", Public: true,
		Metadata: map[string]any{
			"provision_request": map[string]any{"custom_ui_space_id": float64(7)},
			"template_metadata": map[string]any{"agent_file": map[string]any{"name": "bad\nname"}},
		},
	}
	agents.EXPECT().FindByContentID(ctx, "csgclaw", "agent-host").Return(agent, nil)
	comp := &agentCustomUIProxyComponent{agentStore: agents}

	info, matched, err := comp.Resolve(ctx, "agent-host")
	require.ErrorIs(t, err, errorx.ErrCSGClawAgentNameInvalid)
	require.True(t, matched)
	require.Nil(t, info)
}
