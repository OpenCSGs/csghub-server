package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

// BuildModelID computes the public model ID for a deploy, matching the
// format used by the AIGateway model list (ModelIDBuilder.To).
//   - Serverless: {HFPath} if set, else {Path}
//   - Inference: {repoName}:{base36(deployID)}
//   - Other: "" (falls back to LegacyModelID on the consumer side)
func BuildModelID(deploy *database.Deploy) string {
	if deploy == nil || deploy.Repository == nil {
		return ""
	}
	switch deploy.Type {
	case commontypes.ServerlessType:
		if deploy.Repository.HFPath != "" {
			return deploy.Repository.HFPath
		}
		return deploy.Repository.Path
	case commontypes.InferenceType:
		return fmt.Sprintf("%s:%s", deploy.Repository.Name, strconv.FormatInt(deploy.ID, 36))
	default:
		return ""
	}
}

// BuildDeployUpstreamInfoWithDeploy maps a deploy (with Repository and User
// relations loaded) into a DeployUpstreamInfo that carries only the fields
// the upstream sync consumer needs. This is called on the trigger side so
// the consumer does not need to query the server's deploy tables.
//
// If nsStore is non-nil and the deploy has an OwnerNamespace, the namespace's
// type ("user" or "organization") is resolved and set on info.OwnerType. This
// lookup is best-effort: errors are logged but do not fail the build.
//
// Returns nil if the deploy or its repository is nil.
func BuildDeployUpstreamInfoWithDeploy(ctx context.Context, deploy *database.Deploy, nsStore database.NamespaceStore) *commontypes.DeployUpstreamInfo {
	if deploy == nil || deploy.Repository == nil {
		return nil
	}

	repo := deploy.Repository
	legacyModelID := BuildModelID(deploy)
	// For unknown deploy types, fall back to the legacy {modelName}:{svcName}
	// format so the model is still addressable.
	if legacyModelID == "" {
		modelName := repo.Path
		if repo.HFPath != "" {
			modelName = repo.HFPath
		}
		legacyModelID = fmt.Sprintf("%s:%s", modelName, deploy.SvcName)
	}

	var userUUID, username string
	if deploy.User != nil {
		userUUID = deploy.User.UUID
		username = deploy.User.Username
	}

	info := &commontypes.DeployUpstreamInfo{
		DeployID:         deploy.ID,
		RepoPath:         repo.Path,
		RepoName:         repo.Name,
		HFPath:           repo.HFPath,
		DeployType:       deploy.Type,
		Provider:         commontypes.ProviderTypeFromDeployType(deploy.Type),
		Endpoint:         deploy.Endpoint,
		ClusterID:        deploy.ClusterID,
		SvcName:          deploy.SvcName,
		ImageID:          deploy.ImageID,
		RuntimeFramework: deploy.RuntimeFramework,
		EngineArgs:       deploy.EngineArgs,
		Task:             string(deploy.Task),
		UserUUID:         userUUID,
		OwnerUsername:    username,
		OwnerNamespace:   deploy.OwnerNamespace,
		SecureLevel:      deploy.SecureLevel,
		CreatedAt:        deploy.CreatedAt.Unix(),
		LegacyModelID:    legacyModelID,
	}

	if nsStore != nil && deploy.OwnerNamespace != "" {
		ns, nsErr := nsStore.FindByPath(ctx, deploy.OwnerNamespace)
		if nsErr != nil {
			slog.WarnContext(ctx, "upstream event: failed to resolve namespace type",
				slog.String("namespace", deploy.OwnerNamespace), slog.Any("error", nsErr))
		} else {
			info.OwnerType = string(ns.NamespaceType)
		}
	}

	return info
}

// PublishDeployUpstreamSyncEvent marshals a DeployUpstreamSyncEvent and
// publishes it via DefaultEventPublisher. It is fire-and-forget — errors
// are logged but never returned, since the Temporal cron reconciliation
// will catch up on any missed events.
//
// For running events, pass a non-nil info so the consumer does not need to
// query the server's deploy tables. For stop/delete events, pass nil — the
// consumer only needs deployID.
func PublishDeployUpstreamSyncEvent(ctx context.Context, subject string, deployID int64, info *commontypes.DeployUpstreamInfo) {
	if event.DefaultEventPublisher.MQ == nil {
		return
	}
	raw, err := json.Marshal(commontypes.DeployUpstreamSyncEvent{
		DeployID:  deployID,
		EventTime: time.Now().UnixNano(),
		Deploy:    info,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal deploy upstream sync event",
			slog.Any("error", err), slog.Int64("deploy_id", deployID), slog.String("subject", subject))
		return
	}
	_ = event.DefaultEventPublisher.PublishDeployUpstreamSyncEvent(subject, raw)
}
