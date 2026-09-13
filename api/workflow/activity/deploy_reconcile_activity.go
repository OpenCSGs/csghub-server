package activity

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/log"
	v1 "k8s.io/api/core/v1"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	runnerTypes "opencsg.com/csghub-server/runner/types"
)

const (
	maxReconcilePerRun    = 100
	hardTimeoutMultiplier = 3
)

var (
	workflowInitialPhases = []v1alpha1.WorkflowPhase{"", v1alpha1.WorkflowUnknown, v1alpha1.WorkflowPending}
	workflowRunningPhases = []v1alpha1.WorkflowPhase{v1alpha1.WorkflowRunning}
)

func (a *Activities) getLogger(ctx context.Context) log.Logger {
	if ctx.Value("test") == "test" {
		return slog.Default()
	}
	return activity.GetLogger(ctx)
}

// ==================== Entry Point ====================

func (a *Activities) ReconcileAllStatus(ctx context.Context) error {
	if a.deployer == nil {
		a.getLogger(ctx).Error("reconcile: deployer is nil, skip whole reconcile round; " +
			"check worker startup log for NewDeployerForReconcile failure")
		return nil
	}
	if err := a.ReconcileDeployStatus(ctx); err != nil {
		a.getLogger(ctx).Error("reconcile: deploy status reconcile failed", "error", err)
	}
	if err := a.ReconcileWorkflowStatus(ctx); err != nil {
		a.getLogger(ctx).Error("reconcile: workflow status reconcile failed", "error", err)
	}
	return nil
}

// ==================== Deploy Reconcile ====================

func (a *Activities) ReconcileDeployStatus(ctx context.Context) error {
	startTime := time.Now()
	a.getLogger(ctx).Info("reconcile(deploy): started")

	reconcileByStatus(ctx, a, common.Deploying, a.deployConfig.StuckTimeoutMin)
	reconcileByStatus(ctx, a, common.Startup, a.deployConfig.StuckTimeoutMin)
	reconcileByStatus(ctx, a, common.Running, a.deployConfig.RunningReconcileHour*60)

	a.getLogger(ctx).Info("reconcile(deploy): completed", "elapsed_seconds", time.Since(startTime).Seconds())
	return nil
}

func reconcileByStatus(ctx context.Context, a *Activities, status int, timeoutMin int) {
	logger := a.getLogger(ctx)
	batchStart := time.Now()
	deploys, err := a.stores.deployTask.ListDeploysNeedingReconcile(
		ctx, []int{status}, timeoutMin, maxReconcilePerRun,
	)
	if err != nil {
		logger.Error("reconcile(deploy): list deploys failed",
			"status", statusName(status), "error", err)
		return
	}
	if len(deploys) == 0 {
		logger.Debug("reconcile(deploy): no stuck deploys",
			"status", statusName(status), "timeout_min", timeoutMin)
		return
	}

	logger.Info("reconcile(deploy): batch start",
		"status", statusName(status), "count", len(deploys), "timeout_min", timeoutMin)

	hardTimeout := time.Duration(timeoutMin*hardTimeoutMultiplier) * time.Minute
	clusters := groupByCluster(deploys)
	runClusterParallel(ctx, clusters, func(cid string, cds []database.Deploy) {
		reconcileDeployCluster(ctx, a, cid, cds, status, hardTimeout)
	})

	logger.Info("reconcile(deploy): batch done",
		"status", statusName(status), "clusters", len(clusters),
		"elapsed_seconds", time.Since(batchStart).Seconds())
}

func reconcileDeployCluster(ctx context.Context, a *Activities, cid string, deploys []database.Deploy, currentStatus int, hardTimeout time.Duration) {
	clusterBatchDo(ctx, a, cid, deploys, hardTimeout,
		// getLastUpdate
		func(d *database.Deploy) time.Time { return d.StatusUpdateAt },
		// markFailed
		func(d *database.Deploy) {
			if hasPendingInstance(d) {
				a.getLogger(ctx).Info("reconcile: skip fallback, pod still pending",
					"deploy_id", d.ID, "svc_name", d.SvcName)
				return
			}
			applyStatusUpdate(ctx, a, d, currentStatus, fallbackTargetStatus(currentStatus),
				nil, "runner_unreachable")
		},
		// buildBatchItem
		func(d *database.Deploy) (key string, item runnerTypes.BatchStatusItem) {
			switch d.Type {
			case types.SandboxType:
				return d.SvcName, runnerTypes.BatchStatusItem{Type: runnerTypes.ResourceTypeSandbox, Name: d.SvcName}
			case types.SpaceType, types.InferenceType, types.FinetuneType,
				types.ServerlessType, types.NotebookType:
				return d.SvcName, runnerTypes.BatchStatusItem{Type: runnerTypes.ResourceTypeKsvc, Name: d.SvcName}
			default:
				return d.SvcName, runnerTypes.BatchStatusItem{Type: runnerTypes.ResourceTypeKsvc, Name: d.SvcName}
			}
		},
		// onBatchError
		func(d *database.Deploy) {
			if hasPendingInstance(d) {
				a.getLogger(ctx).Info("reconcile: skip fallback, pod still pending",
					"deploy_id", d.ID, "svc_name", d.SvcName)
				return
			}
			if time.Since(d.StatusUpdateAt) > hardTimeout {
				applyStatusUpdate(ctx, a, d, currentStatus, fallbackTargetStatus(currentStatus),
					nil, "runner_unreachable")
			}
		},
		// processResult
		func(d *database.Deploy, r *runnerTypes.BatchStatusItemResult) {
			processBatchResult(ctx, a, d, r, currentStatus)
		},
	)
}

func processBatchResult(ctx context.Context, a *Activities, deploy *database.Deploy, r *runnerTypes.BatchStatusItemResult, currentStatus int) {
	logger := a.getLogger(ctx)
	logger.Debug("reconcile(deploy): process batch item result",
		"deploy_id", deploy.ID, "svc_name", deploy.SvcName, "type", deploy.Type,
		"current_status", statusName(currentStatus),
		"result_code", r.Code, "result_status", r.Status,
		"actual_replica", r.ActualReplica, "desired_replica", r.DesiredReplica,
		"instances", len(r.Instances))

	var newStatus int
	var instances []types.Instance
	switch deploy.Type {
	case types.SandboxType:
		newStatus = mapSandboxStatusToDeployStatus(r.Status, currentStatus)
	case types.SpaceType, types.InferenceType, types.FinetuneType,
		types.ServerlessType, types.NotebookType:
		switch r.Code {
		case common.Stopped:
			if currentStatus == common.Deploying || currentStatus == common.Startup {
				newStatus = common.DeployFailed
			} else if currentStatus == common.Running {
				newStatus = common.Stopped
			} else {
				logger.Warn("reconcile(deploy): stopped result under unexpected status, skip",
					"deploy_id", deploy.ID, "svc_name", deploy.SvcName,
					"current_status", statusName(currentStatus))
				return
			}
		case common.Running:
			// For Deploying/Startup, the informer handles normal flow
			// with more accurate status mapping (Deploying/Sleeping/Running).
			// Reconcile should only fix anomalies, not interfere.
			if currentStatus == common.Deploying || currentStatus == common.Startup {
				logger.Info("reconcile(deploy): runner already running, leave transition to informer",
					"deploy_id", deploy.ID, "svc_name", deploy.SvcName,
					"current_status", statusName(currentStatus))
				return
			}
			newStatus = common.Running
			instances = r.Instances
		default:
			// Service exists but not fully ready (Startup from batch API).
			// Deploying/Startup: let informer handle the normal transition.
			// Running: don't downgrade (scale-to-zero is normal).
			if currentStatus == common.Deploying || currentStatus == common.Startup {
				logger.Info("reconcile(deploy): runner not fully ready, leave transition to informer",
					"deploy_id", deploy.ID, "svc_name", deploy.SvcName,
					"current_status", statusName(currentStatus), "result_code", r.Code)
				return
			}
			if currentStatus == common.Running {
				// scale-to-zero is a normal Running state, do not downgrade
				logger.Info("reconcile(deploy): runner not fully ready under Running, no downgrade (scale-to-zero)",
					"deploy_id", deploy.ID, "svc_name", deploy.SvcName, "result_code", r.Code)
				return
			}
			newStatus = common.Startup
			instances = r.Instances
		}
	default:
		logger.Warn("reconcile: unknown deploy type, skip", "deploy_id", deploy.ID, "type", deploy.Type)
		return
	}
	if newStatus != 0 {
		if newStatus == currentStatus {
			// Running→Running (or other no-op transitions): only refresh
			// StatusUpdateAt to reset the fallback timer. Do NOT touch
			// Reason/Instances or overwrite fields that the informer may
			// have updated since the scan snapshot.
			logger.Info("reconcile(deploy): status unchanged, refresh timestamp only",
				"deploy_id", deploy.ID, "svc_name", deploy.SvcName,
				"status", statusName(newStatus))
			refreshStatusTimestamp(ctx, a, deploy, currentStatus)
		} else {
			applyStatusUpdate(ctx, a, deploy, currentStatus, newStatus, instances, "runner_status_sync")
		}
	}
}

// refreshStatusTimestamp only bumps StatusUpdateAt, leaving all other
// fields untouched. Used when reconcile confirms the deploy is still
// in the same status (e.g. Running→Running) so that the fallback timer
// does not accumulate stale time across successful BatchStatus calls.
func refreshStatusTimestamp(ctx context.Context, a *Activities, deploy *database.Deploy, expectedStatus int) {
	logger := log.With(a.getLogger(ctx),
		"deploy_id", deploy.ID, "svc_name", deploy.SvcName,
		"expected_status", statusName(expectedStatus),
	)
	current, err := a.stores.deployTask.GetDeployByID(ctx, deploy.ID)
	if err != nil {
		logger.Error("reconcile: re-read deploy failed, skip refresh", "error", err)
		return
	}
	if current == nil {
		logger.Warn("reconcile: deploy not found, skip refresh, may be deleted")
		return
	}
	if current.Status != expectedStatus {
		logger.Info("reconcile: status changed since scan, skip refresh",
			"db_status", statusName(current.Status))
		return
	}
	current.StatusUpdateAt = time.Now()
	if err := a.stores.deployTask.UpdateDeploy(ctx, current); err != nil {
		logger.Error("reconcile: refresh status timestamp failed", "error", err)
		return
	}
	logger.Debug("reconcile: status timestamp refreshed")
}

// ==================== Workflow Reconcile ====================

func (a *Activities) ReconcileWorkflowStatus(ctx context.Context) error {
	startTime := time.Now()
	a.getLogger(ctx).Info("reconcile(wf): started")

	reconcileWorkflowByPhase(ctx, a, workflowInitialPhases, a.deployConfig.StuckTimeoutMin)
	reconcileWorkflowByPhase(ctx, a, workflowRunningPhases, a.deployConfig.RunningReconcileHour*60)

	a.getLogger(ctx).Info("reconcile(wf): completed", "elapsed_seconds", time.Since(startTime).Seconds())
	return nil
}

func reconcileWorkflowByPhase(ctx context.Context, a *Activities, phases []v1alpha1.WorkflowPhase, timeoutMin int) {
	logger := a.getLogger(ctx)
	batchStart := time.Now()
	wfs, err := a.stores.argoWorkFlow.ListWorkflowsNeedingReconcile(
		ctx, phases, timeoutMin, maxReconcilePerRun,
	)
	if err != nil {
		logger.Error("reconcile(wf): list workflows failed", "timeout_min", timeoutMin, "error", err)
		return
	}
	if len(wfs) == 0 {
		logger.Debug("reconcile(wf): no stuck workflows",
			"timeout_min", timeoutMin)
		return
	}

	logger.Info("reconcile(wf): batch start", "count", len(wfs), "timeout_min", timeoutMin)
	hardTimeout := time.Duration(timeoutMin*hardTimeoutMultiplier) * time.Minute
	clusters := groupWorkflowsByCluster(wfs)

	runClusterParallel(ctx, clusters, func(cid string, cwfs []database.ArgoWorkflow) {
		reconcileWorkflowCluster(ctx, a, cid, cwfs, hardTimeout)
	})

	logger.Info("reconcile(wf): batch done", "count", len(wfs), "clusters", len(clusters),
		"elapsed_seconds", time.Since(batchStart).Seconds())
}

func reconcileWorkflowCluster(ctx context.Context, a *Activities, cid string, wfs []database.ArgoWorkflow, hardTimeout time.Duration) {
	clusterBatchDo(ctx, a, cid, wfs, hardTimeout,
		func(wf *database.ArgoWorkflow) time.Time {
			if wf.StatusUpdateAt.IsZero() {
				return wf.SubmitTime
			}
			return wf.StatusUpdateAt
		},
		func(wf *database.ArgoWorkflow) {
			wf.Status = v1alpha1.WorkflowFailed
			wf.StatusUpdateAt = time.Now()
			if _, err := a.stores.argoWorkFlow.UpdateWorkFlow(ctx, *wf); err != nil {
				a.getLogger(ctx).Error("reconcile(wf): mark failed error", "wf_id", wf.ID, "error", err)
			} else {
				a.getLogger(ctx).Info("reconcile(wf): workflow marked failed",
					"wf_id", wf.ID, "cluster_id", cid)
			}
		},
		func(wf *database.ArgoWorkflow) (string, runnerTypes.BatchStatusItem) {
			return wf.TaskId, runnerTypes.BatchStatusItem{Type: runnerTypes.ResourceTypeWorkflow, Name: wf.TaskId}
		},
		func(wf *database.ArgoWorkflow) {
			lastUpdate := wf.StatusUpdateAt
			if lastUpdate.IsZero() {
				lastUpdate = wf.SubmitTime
			}
			elapsed := time.Since(lastUpdate)
			if elapsed > hardTimeout {
				a.getLogger(ctx).Warn("reconcile(wf): batch error timeout, marking failed",
					"wf_id", wf.ID, "hard_timeout", hardTimeout,
					"elapsed", elapsed.Round(time.Second).String())
				wf.Status = v1alpha1.WorkflowFailed
				wf.StatusUpdateAt = time.Now()
				if _, err := a.stores.argoWorkFlow.UpdateWorkFlow(ctx, *wf); err != nil {
					a.getLogger(ctx).Error("reconcile(wf): mark failed error", "wf_id", wf.ID, "error", err)
				}
			} else {
				a.getLogger(ctx).Info("reconcile(wf): batch error but within hard timeout, skip",
					"wf_id", wf.ID, "elapsed", elapsed.Round(time.Second).String(),
					"hard_timeout", hardTimeout)
			}
		},
		func(wf *database.ArgoWorkflow, r *runnerTypes.BatchStatusItemResult) {
			if string(wf.Status) != r.Phase && len(r.Phase) > 0 {
				wf.Status = v1alpha1.WorkflowPhase(r.Phase)
				wf.StatusUpdateAt = time.Now()
				if _, err := a.stores.argoWorkFlow.UpdateWorkFlow(ctx, *wf); err != nil {
					a.getLogger(ctx).Error("reconcile(wf): update failed", "wf_id", wf.ID, "error", err)
				} else {
					a.getLogger(ctx).Info("reconcile(wf): status synced", "wf_id", wf.ID, "phase", r.Phase)
				}
			}
		},
	)
}

// ==================== Shared Cluster Batch Pattern ====================

// clusterBatchDo is the shared pattern for per-cluster reconciliation.
// It handles: nil deployer check → cluster health check → hardTimeout →
// batch API call → result processing.
func clusterBatchDo[T any](
	ctx context.Context, a *Activities, cid string, items []T, hardTimeout time.Duration,
	getLastUpdate func(*T) time.Time,
	markFailed func(*T),
	buildItem func(*T) (key string, item runnerTypes.BatchStatusItem),
	onBatchError func(*T),
	processResult func(*T, *runnerTypes.BatchStatusItemResult),
) {
	if a.deployer == nil {
		return
	}

	logger := a.getLogger(ctx)

	// Cluster health check
	timedOut, err := a.deployer.CheckHeartbeatTimeout(ctx, cid)
	if err != nil {
		logger.Warn("reconcile: heartbeat check failed, skip timeout fallback and proceed batch",
			"cluster_id", cid, "item_count", len(items), "error", err)
	} else if timedOut {
		marked := 0
		for i := range items {
			item := &items[i]
			if time.Since(getLastUpdate(item)) > hardTimeout {
				logger.Warn("reconcile: cluster unhealthy + hard timeout, marking failed",
					"cluster_id", cid)
				markFailed(item)
				marked++
			}
		}
		logger.Warn("reconcile: cluster heartbeat timeout, applied fallback",
			"cluster_id", cid, "item_count", len(items),
			"marked_failed", marked, "hard_timeout_min", int(hardTimeout.Minutes()))
		return
	}

	// Build batch request
	batchItems := make([]runnerTypes.BatchStatusItem, 0, len(items))
	idxMap := make(map[string]int, len(items))
	for i := range items {
		key, bitem := buildItem(&items[i])
		batchItems = append(batchItems, bitem)
		idxMap[key] = i
	}

	logger.Info("reconcile: calling BatchStatus",
		"cluster_id", cid, "item_count", len(batchItems))

	resp, err := a.deployer.BatchStatus(ctx, &runnerTypes.BatchStatusRequest{ClusterID: cid, Items: batchItems})
	if err != nil {
		logger.Warn("reconcile: BatchStatus failed",
			"cluster_id", cid, "count", len(items), "error", err)
		if onBatchError != nil {
			for i := range items {
				onBatchError(&items[i])
			}
		}
		return
	}

	logger.Info("reconcile: BatchStatus success",
		"cluster_id", cid, "request_count", len(batchItems), "result_count", len(resp.Items))

	for _, r := range resp.Items {
		idx, ok := idxMap[r.Name]
		if !ok {
			logger.Warn("reconcile: batch response item not in request, ignore",
				"cluster_id", cid, "name", r.Name)
			continue
		}
		if r.Error != "" {
			logger.Warn("reconcile: batch item error",
				"name", r.Name, "error", r.Error)
			// Apply timeout fallback for individual errors,
			// same as the whole-batch-failure path.
			if onBatchError != nil {
				onBatchError(&items[idx])
			}
			continue
		}
		processResult(&items[idx], &r)
	}
}

// runClusterParallel runs a per-cluster function in parallel across clusters.
func runClusterParallel[T any](ctx context.Context, clusters map[string][]T, fn func(string, []T)) {
	var wg sync.WaitGroup
	for cid, citems := range clusters {
		wg.Add(1)
		go func(id string, items []T) {
			defer wg.Done()
			fn(id, items)
		}(cid, citems)
	}
	wg.Wait()
}

// ==================== Helpers ====================

// hasPendingInstance reports whether the deploy has any Pod still in Pending phase.
// Reconcile must not fall back to DeployFailed while a Pod is still scheduling,
// since "service not found" during Pending is a transient state, not a permanent
// runner-unreachable condition. The webhook writes Pending instances into
// deploy.Instances on runner.service.create/change events.
func hasPendingInstance(d *database.Deploy) bool {
	for _, ins := range d.Instances {
		if ins.Status == string(v1.PodPending) {
			return true
		}
	}
	return false
}

func groupByCluster(deploys []database.Deploy) map[string][]database.Deploy {
	clusters := make(map[string][]database.Deploy)
	for _, d := range deploys {
		key := d.ClusterID
		if key == "" {
			key = "_default"
		}
		clusters[key] = append(clusters[key], d)
	}
	return clusters
}

func groupWorkflowsByCluster(wfs []database.ArgoWorkflow) map[string][]database.ArgoWorkflow {
	clusters := make(map[string][]database.ArgoWorkflow)
	for _, wf := range wfs {
		key := wf.ClusterID
		if key == "" {
			key = "_default"
		}
		clusters[key] = append(clusters[key], wf)
	}
	return clusters
}

func mapSandboxStatusToDeployStatus(status, currentStatus int) int {
	switch status {
	case common.Running:
		return common.Running
	case common.Startup, common.Deploying:
		return common.Startup
	case common.DeployFailed:
		return common.DeployFailed
	case common.RunTimeError:
		return common.RunTimeError
	case common.Stopped:
		if currentStatus == common.Deploying || currentStatus == common.Startup {
			return common.DeployFailed
		}
		return common.Stopped
	default:
		return currentStatus
	}
}

func applyStatusUpdate(ctx context.Context, a *Activities, deploy *database.Deploy, expectedStatus, newStatus int, instances []types.Instance, source string) {
	logger := log.With(a.getLogger(ctx),
		"deploy_id", deploy.ID, "svc_name", deploy.SvcName,
		"expected_status", statusName(expectedStatus),
		"new_status", statusName(newStatus), "source", source,
	)
	current, err := a.stores.deployTask.GetDeployByID(ctx, deploy.ID)
	if err != nil {
		logger.Error("reconcile: re-read deploy failed, skip", "error", err)
		return
	}
	if current == nil {
		logger.Warn("reconcile: deploy not found, may be deleted")
		return
	}
	if current.Status != expectedStatus {
		logger.Info("reconcile: status changed since scan, skip update",
			"db_status", statusName(current.Status))
		return
	}
	deploy.Status = newStatus
	deploy.StatusUpdateAt = time.Now()
	deploy.Reason = reconcileReason(source)
	if len(instances) > 0 {
		deploy.Instances = instances
	}
	if err := a.stores.deployTask.UpdateDeploy(ctx, deploy); err != nil {
		logger.Error("reconcile: update deploy failed", "error", err)
		return
	}
	logger.Info("reconcile: deploy status updated")
}

// fallbackTargetStatus decides the target status when the runner is
// unreachable and the deploy has been stuck beyond hardTimeout. For
// Running deploys, scale-to-zero or transient runner unavailability
// should surface as Stopped (recoverable) rather than DeployFailed
// (terminal), matching the normal-mapping semantics in processBatchResult.
// Deploying/Startup remain DeployFailed since a stuck deployment is
// genuinely failed.
func fallbackTargetStatus(currentStatus int) int {
	if currentStatus == common.Running {
		return common.Stopped
	}
	return common.DeployFailed
}

func reconcileReason(source string) string {
	switch source {
	case "runner_unreachable":
		return "reconcile: runner API unreachable, status updated by fallback mechanism after timeout"
	case "runner_status_sync":
		return "reconcile: status synced from runner Knative service API"
	case "runner_workflow_sync":
		return "reconcile: status synced from runner workflow API"
	case "runner_sandbox_sync":
		return "reconcile: status synced from runner sandbox API"
	default:
		return "reconcile: status updated by fallback mechanism (" + source + ")"
	}
}

func statusName(status int) string {
	switch status {
	case common.Pending:
		return "Pending"
	case common.BuildInQueue:
		return "BuildInQueue"
	case common.Building:
		return "Building"
	case common.BuildFailed:
		return "BuildFailed"
	case common.BuildSuccess:
		return "BuildSuccess"
	case common.BuildSkip:
		return "BuildSkip"
	case common.Deploying:
		return "Deploying"
	case common.DeployFailed:
		return "DeployFailed"
	case common.Startup:
		return "Startup"
	case common.Running:
		return "Running"
	case common.RunTimeError:
		return "RunTimeError"
	case common.Sleeping:
		return "Sleeping"
	case common.Stopped:
		return "Stopped"
	case common.Deleted:
		return "Deleted"
	case common.ResourceUnhealthy:
		return "ResourceUnhealthy"
	default:
		return "Unknown"
	}
}
