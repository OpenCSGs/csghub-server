package upstream

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	deploybuilder "opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

// AIGatewayUpstreamSyncComponent handles syncing deploy lifecycle events
// to the unified ai_gateway_upstreams table. It is the write-side counterpart
// to the AIGateway routing read path — internal deploys are persisted as
// upstreams with source=csghub so they can be served uniformly alongside
// external upstreams.
type AIGatewayUpstreamSyncComponent interface {
	// SyncRunningDeploy creates or updates an upstream for a running deploy.
	// The deploy fields are carried in info, built on the trigger side, so
	// this method does not need to query the server's deploy tables.
	SyncRunningDeploy(ctx context.Context, info *commontypes.DeployUpstreamInfo) error
	// DisableDeployTarget marks the upstream for a stopped deploy as disabled.
	DisableDeployTarget(ctx context.Context, deployID int64) error
	// DeleteDeployTarget removes the upstream for a deleted deploy.
	DeleteDeployTarget(ctx context.Context, deployID int64) error
	// ReconcileUpstreams performs a full reconciliation pass: it syncs all
	// running deploys and disables upstreams whose deploys are no longer running.
	ReconcileUpstreams(ctx context.Context) error
}

type aiGatewayUpstreamSyncComponentImpl struct {
	deployStore    database.DeployTaskStore
	upstreamStore  database.UpstreamStore
	llmConfigStore database.LLMConfigStore
	clusterStore   database.ClusterInfoStore
	namespaceStore database.NamespaceStore
	modelIDBuilder ModelIDBuilder
}

type AIGatewayUpstreamSyncComponentConfig struct {
	DeployStore    database.DeployTaskStore
	UpstreamStore  database.UpstreamStore
	LLMConfigStore database.LLMConfigStore
	ClusterStore   database.ClusterInfoStore
	NamespaceStore database.NamespaceStore
	ModelIDBuilder ModelIDBuilder
}

func NewAIGatewayUpstreamSyncComponent(cfg AIGatewayUpstreamSyncComponentConfig) AIGatewayUpstreamSyncComponent {
	return &aiGatewayUpstreamSyncComponentImpl{
		deployStore:    cfg.DeployStore,
		upstreamStore:  cfg.UpstreamStore,
		llmConfigStore: cfg.LLMConfigStore,
		clusterStore:   cfg.ClusterStore,
		namespaceStore: cfg.NamespaceStore,
		modelIDBuilder: cfg.ModelIDBuilder,
	}
}

// NewAIGatewayUpstreamSyncComponentDefault creates a sync component with all
// dependencies initialized from the given config. This is the convenience
// constructor for the aigateway binary where the stores are not shared with
// other components.
func NewAIGatewayUpstreamSyncComponentDefault(cfg *config.Config) AIGatewayUpstreamSyncComponent {
	return NewAIGatewayUpstreamSyncComponent(AIGatewayUpstreamSyncComponentConfig{
		DeployStore:    database.NewDeployTaskStore(),
		UpstreamStore:  database.NewUpstreamStore(cfg),
		LLMConfigStore: database.NewLLMConfigStore(cfg),
		ClusterStore:   database.NewClusterInfoStore(),
		NamespaceStore: database.NewNamespaceStore(),
		ModelIDBuilder: NewModelIDBuilder(),
	})
}

// SyncRunningDeploy handles a "deploy is running" event.
//
// Flow:
//  1. Look up the existing upstream by source=csghub + source_id=deploy.ID.
//     - If found AND its llm_config still exists: update the upstream in place,
//     reusing the existing llm_config_id. This prevents duplicate llm_configs
//     when an admin has manually moved the upstream to a different llm_config.
//     - If found but its llm_config was deleted: delete the orphaned upstream
//     and fall through to step 2.
//     - If not found: fall through to step 2.
//  2. Find or create an llm_config by model_name (legacyModelID), then upsert
//     the upstream.
func (c *aiGatewayUpstreamSyncComponentImpl) SyncRunningDeploy(ctx context.Context, info *commontypes.DeployUpstreamInfo) error {
	if info == nil || info.DeployID == 0 {
		return fmt.Errorf("sync running deploy: nil info or zero deploy_id")
	}

	// Step 1: check if an upstream already exists for this deploy.
	existing, err := c.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, info.DeployID)
	if err != nil {
		return fmt.Errorf("lookup existing upstream for deploy %d: %w", info.DeployID, err)
	}
	if existing != nil {
		// Check whether the upstream's llm_config still exists.
		llmCfg, err := c.llmConfigStore.GetByID(ctx, existing.LLMConfigID)
		if err != nil {
			return fmt.Errorf("lookup llm_config %d for existing upstream: %w", existing.LLMConfigID, err)
		}
		if llmCfg != nil {
			// Both upstream and llm_config exist — update the upstream in place,
			// reusing the existing llm_config_id.
			upstreamURL, hostOverride := c.computeURLAndHost(ctx, info)
			existing.URL = upstreamURL
			existing.ModelName = info.RepoPath
			existing.Provider = info.Provider
			existing.Enabled = true
			// Rebuild only the sync-owned metadata fields (InternalModelInfo,
			// including Host). UI-configured fields (ResponsesChatAdapter,
			// Protocol) set by an admin via the admin API are preserved so they
			// survive re-syncs and the periodic reconcile pass. This mirrors the
			// preservation logic in component.UpdateUpstream.
			existing.Metadata = c.mergeSyncOwnedMetadata(existing.Metadata, info, hostOverride)
			if err := c.upstreamStore.Update(ctx, existing); err != nil {
				return fmt.Errorf("update existing upstream for deploy %d: %w", info.DeployID, err)
			}
			slog.InfoContext(ctx, "upstream sync: updated existing upstream", "deploy_id", info.DeployID, "upstream_id", existing.ID, "llm_config_id", existing.LLMConfigID)
			return nil
		}
		// llm_config was deleted but upstream still exists — delete the orphan.
		slog.WarnContext(ctx, "upstream sync: llm_config deleted, removing orphaned upstream", "deploy_id", info.DeployID, "upstream_id", existing.ID, "llm_config_id", existing.LLMConfigID)
		if err := c.upstreamStore.Delete(ctx, existing.ID); err != nil {
			return fmt.Errorf("delete orphaned upstream for deploy %d: %w", info.DeployID, err)
		}
		// Fall through to step 2.
	}

	// Step 2: upstream doesn't exist (or was just deleted). Find or create llm_config.
	llmCfg, err := c.llmConfigStore.GetByModelName(ctx, info.LegacyModelID)
	if err != nil {
		return fmt.Errorf("lookup llm_config by model_name %q: %w", info.LegacyModelID, err)
	}

	var llmConfigID int64
	if llmCfg != nil {
		llmConfigID = llmCfg.ID
	} else {
		// Always enable the llm_config at creation: the deploy is a user-owned
		// resource, and a disabled llm_config would make the user's own
		// inference endpoint unreachable until an admin intervenes. Admins can
		// still disable individual llm_configs later via the admin API.
		newCfg, err := c.llmConfigStore.Create(ctx, database.LLMConfig{
			ModelName:          info.LegacyModelID,
			Type:               database.LLMTypeAigatewayExternal,
			Enabled:            true,
			NeedSensitiveCheck: false,
		})
		if err != nil {
			return fmt.Errorf("create llm_config for deploy %d: %w", info.DeployID, err)
		}
		llmConfigID = newCfg.ID
	}

	upstreamURL, hostOverride := c.computeURLAndHost(ctx, info)

	upstream := &database.Upstream{
		LLMConfigID: llmConfigID,
		URL:         upstreamURL,
		Weight:      1,
		Enabled:     true,
		ModelName:   info.RepoPath,
		Provider:    info.Provider,
		Source:      commontypes.UpstreamSourceCSGHubDeploy,
		SourceID:    info.DeployID,
		Metadata: &commontypes.UpstreamMetadata{
			InternalModelInfo: c.buildInternalModelInfo(info, hostOverride),
		},
	}

	if err := c.upstreamStore.UpsertInternalDeployTarget(ctx, upstream); err != nil {
		return fmt.Errorf("upsert upstream for deploy %d: %w", info.DeployID, err)
	}

	slog.InfoContext(ctx, "upstream sync: synced running deploy", "deploy_id", info.DeployID, "upstream_id", upstream.ID)
	return nil
}

// computeURLAndHost resolves the upstream URL and HostOverride from cluster
// info. If the cluster store is unavailable or the cluster lookup fails, it
// falls back to using the deploy's endpoint directly.
func (c *aiGatewayUpstreamSyncComponentImpl) computeURLAndHost(ctx context.Context, info *commontypes.DeployUpstreamInfo) (string, string) {
	upstreamURL := info.Endpoint
	hostOverride := ""
	if c.clusterStore != nil && info.ClusterID != "" {
		cluster, err := c.clusterStore.ByClusterID(ctx, info.ClusterID)
		if err != nil {
			slog.WarnContext(ctx, "upstream sync: failed to get cluster, using deploy endpoint as-is",
				"deploy_id", info.DeployID, "cluster_id", info.ClusterID, "error", err)
			return upstreamURL, hostOverride
		}
		target, host, extractErr := commonutils.ExtractDeployTargetAndHost(ctx, &cluster, commontypes.EndpointReq{
			ClusterID: info.ClusterID,
			Target:    info.Endpoint,
			Endpoint:  info.Endpoint,
			SvcName:   info.SvcName,
		})
		if extractErr != nil {
			slog.WarnContext(ctx, "upstream sync: failed to extract target/host, using deploy endpoint as-is",
				"deploy_id", info.DeployID, "error", extractErr)
			return upstreamURL, hostOverride
		}
		// Preserve the deploy endpoint's path when the cluster AppEndpoint
		// replaces the host. Some deploys serve APIs under a path prefix
		// (e.g. /v1 for OpenAI-compatible inference), while others (e.g.
		// Stable Diffusion) have no path. Only the path from the deploy
		// endpoint is carried over — no path is injected.
		endpointPath := commonutils.ExtractURLPath(info.Endpoint)
		if endpointPath != "" {
			if parsed, pErr := url.Parse(target); pErr == nil {
				parsed.Path = strings.TrimRight(parsed.Path, "/") + endpointPath
				target = parsed.String()
			}
		}
		upstreamURL = target
		hostOverride = host
	}
	return upstreamURL, hostOverride
}

// mergeSyncOwnedMetadata rebuilds the sync-owned metadata fields
// (InternalModelInfo, including Host) from the current deploy info while
// preserving UI-configured fields (ResponsesChatAdapter, Protocol) that an
// admin may have set on the existing upstream via the admin API. If the
// existing metadata is nil, a fresh metadata object is created.
func (c *aiGatewayUpstreamSyncComponentImpl) mergeSyncOwnedMetadata(existing *commontypes.UpstreamMetadata, info *commontypes.DeployUpstreamInfo, hostOverride string) *commontypes.UpstreamMetadata {
	var responsesChatAdapter *commontypes.ResponsesChatAdapter
	var protocol string
	if existing != nil {
		responsesChatAdapter = existing.ResponsesChatAdapter
		protocol = existing.Protocol
	}
	return &commontypes.UpstreamMetadata{
		InternalModelInfo:    c.buildInternalModelInfo(info, hostOverride),
		ResponsesChatAdapter: responsesChatAdapter,
		Protocol:             protocol,
	}
}

// DisableDeployTarget handles a "deploy stopped" event.
// It finds the upstream by source+source_id and sets enabled=false.
func (c *aiGatewayUpstreamSyncComponentImpl) DisableDeployTarget(ctx context.Context, deployID int64) error {
	upstream, err := c.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deployID)
	if err != nil {
		return fmt.Errorf("find upstream for deploy %d to disable: %w", deployID, err)
	}
	if upstream == nil {
		slog.WarnContext(ctx, "upstream sync: deploy upstream not found for disable, skipping", "deploy_id", deployID)
		return nil
	}
	upstream.Enabled = false
	if err := c.upstreamStore.Update(ctx, upstream); err != nil {
		return fmt.Errorf("disable upstream for deploy %d: %w", deployID, err)
	}
	slog.InfoContext(ctx, "upstream sync: disabled deploy upstream", "deploy_id", deployID, "upstream_id", upstream.ID)
	return nil
}

// DeleteDeployTarget handles a "deploy deleted" event.
// It finds and deletes the upstream by source+source_id. When the upstream's
// llm_config is left with no upstreams after the deletion, the llm_config is
// deleted as well so no unroutable orphan configs accumulate.
func (c *aiGatewayUpstreamSyncComponentImpl) DeleteDeployTarget(ctx context.Context, deployID int64) error {
	upstream, err := c.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deployID)
	if err != nil {
		return fmt.Errorf("find upstream for deploy %d to delete: %w", deployID, err)
	}
	if upstream == nil {
		slog.WarnContext(ctx, "upstream sync: deploy upstream not found for delete, skipping", "deploy_id", deployID)
		return nil
	}
	if err := c.upstreamStore.Delete(ctx, upstream.ID); err != nil {
		return fmt.Errorf("delete upstream for deploy %d: %w", deployID, err)
	}
	slog.InfoContext(ctx, "upstream sync: deleted deploy upstream", "deploy_id", deployID, "upstream_id", upstream.ID)

	// Cascade: delete the llm_config when its last upstream is gone.
	if upstream.LLMConfigID != 0 {
		remaining, err := c.upstreamStore.ListByLLMConfigID(ctx, upstream.LLMConfigID)
		if err != nil {
			return fmt.Errorf("list remaining upstreams for llm_config %d after deploy %d delete: %w", upstream.LLMConfigID, deployID, err)
		}
		if len(remaining) == 0 {
			if err := c.llmConfigStore.Delete(ctx, upstream.LLMConfigID); err != nil {
				return fmt.Errorf("delete empty llm_config %d after deploy %d delete: %w", upstream.LLMConfigID, deployID, err)
			}
			slog.InfoContext(ctx, "upstream sync: deleted empty llm_config after upstream delete",
				"deploy_id", deployID, "llm_config_id", upstream.LLMConfigID)
		}
	}
	return nil
}

// ReconcileUpstreams performs a full reconciliation pass.
//  1. Lists all running serverless and inference deploys and syncs each one.
//  2. Lists all enabled csghub upstreams and disables any whose deploy
//     is no longer in the running set (ghost upstreams).
func (c *aiGatewayUpstreamSyncComponentImpl) ReconcileUpstreams(ctx context.Context) error {
	// Sync all running deploys — only serverless and inference types are
	// relevant to the AIGateway. Spaces, finetunes, evaluations, and notebooks
	// must not be pushed as upstreams.
	runningDeploys, err := c.deployStore.ListRunningDeploysByTypes(ctx, []int{
		commontypes.ServerlessType,
		commontypes.InferenceType,
	})
	if err != nil {
		return fmt.Errorf("reconcile: list running deploys: %w", err)
	}

	runningDeployIDs := make(map[int64]struct{}, len(runningDeploys))
	for _, deploy := range runningDeploys {
		runningDeployIDs[deploy.ID] = struct{}{}
		// Load with relations to build the upstream info.
		fullDeploy, err := c.deployStore.GetDeployByIDWithRelations(ctx, deploy.ID)
		if err != nil {
			slog.ErrorContext(ctx, "reconcile: failed to load deploy with relations", "deploy_id", deploy.ID, "error", err)
			continue
		}
		if fullDeploy == nil {
			continue
		}
		info := deploybuilder.BuildDeployUpstreamInfoWithDeploy(ctx, fullDeploy, c.namespaceStore)
		if info == nil {
			continue
		}
		if err := c.SyncRunningDeploy(ctx, info); err != nil {
			slog.ErrorContext(ctx, "reconcile: failed to sync running deploy", "deploy_id", deploy.ID, "error", err)
		}
	}

	// Check for ghost upstreams — csghub upstreams whose deploys are no longer running.
	csghubUpstreams, err := c.upstreamStore.ListEnabledBySource(ctx, commontypes.UpstreamSourceCSGHubDeploy)
	if err != nil {
		return fmt.Errorf("reconcile: list csghub upstreams: %w", err)
	}

	for _, upstream := range csghubUpstreams {
		if _, ok := runningDeployIDs[upstream.SourceID]; !ok {
			slog.InfoContext(ctx, "reconcile: disabling ghost upstream", "upstream_id", upstream.ID, "source_id", upstream.SourceID)
			upstream.Enabled = false
			if err := c.upstreamStore.Update(ctx, upstream); err != nil {
				slog.ErrorContext(ctx, "reconcile: failed to disable ghost upstream", "upstream_id", upstream.ID, "error", err)
			}
		}
	}

	return nil
}

func (c *aiGatewayUpstreamSyncComponentImpl) buildInternalModelInfo(info *commontypes.DeployUpstreamInfo, hostOverride string) *commontypes.InternalModelInfo {
	return &commontypes.InternalModelInfo{
		CSGHubModelID:    info.RepoPath,
		RepoName:         info.RepoName,
		HFPath:           info.HFPath,
		LegacyModelID:    info.LegacyModelID,
		OwnerUUID:        info.UserUUID,
		OwnerUsername:    info.OwnerUsername,
		OwnerNamespace:   info.OwnerNamespace,
		OwnerType:        info.OwnerType,
		ClusterID:        info.ClusterID,
		SvcName:          info.SvcName,
		SvcType:          info.DeployType,
		ImageID:          info.ImageID,
		RuntimeFramework: info.RuntimeFramework,
		EngineArgs:       info.EngineArgs,
		Task:             info.Task,
		SourceDeployID:   info.DeployID,
		CreatedAt:        info.CreatedAt,
		Host:             hostOverride,
		SecureLevel:      info.SecureLevel,
	}
}
