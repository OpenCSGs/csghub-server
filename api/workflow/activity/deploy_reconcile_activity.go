package activity

import (
	"context"
	"fmt"
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
	hardTimeoutMultiplier = 4
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
	reconcileByStatus(ctx, a, common.ResourceUnhealthy, a.deployConfig.UnhealthyReconcileMin)

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
	logger := log.With(a.getLogger(ctx),
		"cluster_id", cid,
		"current_status", statusName(currentStatus),
		"deploy_count", len(deploys),
		"hard_timeout", hardTimeout.String(),
	)

	// 1. Heartbeat check — if the cluster is down, apply timeout fallback.
	// The cluster is unreachable, so we cannot verify any deploy's real
	// status — mark all timed-out deploys as ResourceUnhealthy regardless
	// of currentStatus (Running, Deploying, or Startup).
	// Skip for ResourceUnhealthy recovery: the deploy is already unhealthy,
	// re-marking it is a no-op.
	if currentStatus != common.ResourceUnhealthy && checkHeartbeatForCluster(ctx, a, cid) {
		logger.Warn("reconcile(deploy): cluster heartbeat timed out, applying fallback")
		marked := 0
		for i := range deploys {
			elapsed := time.Since(deploys[i].StatusUpdateAt)
			if elapsed > hardTimeout {
				logger.Warn("reconcile(deploy): deploy exceeded hard timeout, falling back",
					"deploy_id", deploys[i].ID, "svc_name", deploys[i].SvcName,
					"last_status_update", deploys[i].StatusUpdateAt.Format(time.RFC3339),
					"elapsed", elapsed.Round(time.Second).String(),
					"target_status", statusName(common.ResourceUnhealthy))
				applyFallback(ctx, a, &deploys[i], currentStatus, common.ResourceUnhealthy)
				marked++
			} else {
				logger.Info("reconcile(deploy): deploy within hard timeout, skip",
					"deploy_id", deploys[i].ID, "svc_name", deploys[i].SvcName,
					"last_status_update", deploys[i].StatusUpdateAt.Format(time.RFC3339),
					"elapsed", elapsed.Round(time.Second).String())
			}
		}
		logger.Warn("reconcile(deploy): heartbeat timeout fallback done",
			"marked", marked, "skipped", len(deploys)-marked)
		return
	}

	// 2. Query runner and process results.
	logger.Info("reconcile(deploy): cluster healthy, querying runner")
	clusterBatchDo(ctx, a, cid, deploys,
		buildDeployBatchItem,
		func(d *database.Deploy, r *runnerTypes.BatchStatusItemResult, err error) {
			// ResourceUnhealthy recovery: the deploy is already at the worst
			// fallback status, so there is no hardTimeout fallback. Distinguish
			// a real RPC failure (r == nil → runner unreachable, stay unhealthy)
			// from a per-item Error (r != nil → runner was reachable; for ksvc
			// the result still carries a valid Code, e.g. Stopped for a deleted
			// service, which is a definitive recovery status).
			if currentStatus == common.ResourceUnhealthy {
				if r == nil {
					logger.Info("reconcile(deploy): unhealthy deploy runner unreachable, skip",
						"deploy_id", d.ID, "svc_name", d.SvcName, "error", err)
					return
				}
				processUnhealthyResult(ctx, a, d, r)
				return
			}
			if err != nil {
				// BatchStatus failed or item error — apply fallback only if
				// the deploy has been stuck beyond hardTimeout. We cannot
				// verify the deploy's real status, so mark it ResourceUnhealthy
				// regardless of currentStatus.
				elapsed := time.Since(d.StatusUpdateAt)
				if elapsed > hardTimeout {
					logger.Warn("reconcile(deploy): batch error + hard timeout, falling back",
						"deploy_id", d.ID, "svc_name", d.SvcName,
						"last_status_update", d.StatusUpdateAt.Format(time.RFC3339),
						"elapsed", elapsed.Round(time.Second).String(),
						"target_status", statusName(common.ResourceUnhealthy),
						"error", err)
					applyFallback(ctx, a, d, currentStatus, common.ResourceUnhealthy)
				} else {
					logger.Info("reconcile(deploy): batch error but within hard timeout, skip",
						"deploy_id", d.ID, "svc_name", d.SvcName,
						"last_status_update", d.StatusUpdateAt.Format(time.RFC3339),
						"elapsed", elapsed.Round(time.Second).String(),
						"error", err)
				}
				return
			}
			processBatchResult(ctx, a, d, r, currentStatus)
		},
	)
}

func processBatchResult(ctx context.Context, a *Activities, deploy *database.Deploy, r *runnerTypes.BatchStatusItemResult, currentStatus int) {
	logger := log.With(a.getLogger(ctx),
		"deploy_id", deploy.ID, "svc_name", deploy.SvcName,
		"current_status", statusName(currentStatus),
		"type", deploy.Type,
	)
	logger.Debug("reconcile(deploy): process batch item result",
		"result_code", r.Code, "result_status", r.Status,
		"actual_replica", r.ActualReplica, "desired_replica", r.DesiredReplica,
		"instances", len(r.Instances))

	// Step 1: Map the runner result to the deploy status the runner observes.
	runnerStatus, instances, ok := mapRunnerResultToStatus(deploy, r, currentStatus)
	if !ok {
		logger.Warn("reconcile(deploy): unknown deploy type, skip", "type", deploy.Type)
		return
	}

	// Step 2: If the runner agrees with the current DB status, the deploy is
	// healthy — just refresh StatusUpdateAt to reset the fallback timer.
	if runnerStatus == currentStatus {
		logger.Info("reconcile(deploy): status unchanged, refresh timestamp",
			"runner_status", statusName(runnerStatus))
		refreshStatusTimestamp(ctx, a, deploy, currentStatus)
		return
	}

	// Step 3: The runner disagrees. For ksvc deploys, reconcile only fixes
	// anomalies (a Running deploy whose service vanished, or a deploying
	// deploy that failed); normal transitions are left to the informer.
	// Sandbox deploys have no separate informer, so the mapped status is
	// applied directly.
	if deploy.Type == types.SandboxType {
		applyStatusUpdate(ctx, a, deploy, currentStatus, runnerStatus, instances, "runner_sandbox_sync")
		return
	}

	// Ksvc: only apply the anomaly transitions, skip the rest.
	applyStatus, targetStatus := shouldApplyKsvcTransition(currentStatus, runnerStatus)
	if !applyStatus {
		logger.Info("reconcile(deploy): runner status differs, leave to informer",
			"runner_status", statusName(runnerStatus))
		return
	}

	applyStatusUpdate(ctx, a, deploy, currentStatus, targetStatus, instances, "runner_status_sync")
}

// processUnhealthyResult handles the recovery path for deploys stuck in
// ResourceUnhealthy. Unlike processBatchResult, it accepts any definitive
// status the runner reports — the deploy is already abnormal, so any clear
// runner status is a recovery. If the runner has no definitive status (e.g.
// sandbox default case returns currentStatus), the deploy stays unhealthy.
func processUnhealthyResult(ctx context.Context, a *Activities, deploy *database.Deploy, r *runnerTypes.BatchStatusItemResult) {
	logger := log.With(a.getLogger(ctx),
		"deploy_id", deploy.ID, "svc_name", deploy.SvcName,
		"current_status", statusName(common.ResourceUnhealthy),
	)
	logger.Debug("reconcile(deploy): process unhealthy result",
		"result_code", r.Code, "result_status", r.Status,
		"instances", len(r.Instances))

	runnerStatus, instances, ok := mapRunnerResultToStatus(deploy, r, common.ResourceUnhealthy)
	if !ok {
		logger.Warn("reconcile(deploy): unknown deploy type, skip", "type", deploy.Type)
		return
	}
	// mapRunnerResultToStatus returns currentStatus (ResourceUnhealthy) when
	// the runner has no definitive status (e.g. sandbox default case) — skip.
	if runnerStatus == common.ResourceUnhealthy {
		logger.Info("reconcile(deploy): runner has no definitive status, stay unhealthy",
			"runner_status", statusName(runnerStatus))
		return
	}
	applyStatusUpdate(ctx, a, deploy, common.ResourceUnhealthy, runnerStatus, instances, "runner_status_sync")
}

// mapRunnerResultToStatus converts a runner BatchStatusItemResult into the
// deploy status the runner observes, independent of the DB's currentStatus
// (except for sandbox, whose mapping itself depends on currentStatus).
//
// Ksvc (Space/Inference/Finetune/Serverless/Notebook): the runner returns
// Code ∈ {Running, Stopped, Startup}. Stopped means the Knative service is
// gone from the cluster; the caller decides whether that maps to Stopped or
// DeployFailed based on currentStatus.
//
// Sandbox: the runner returns Status via mapSandboxStatusToDeployStatus,
// which already encodes the full sandbox condition mapping including the
// Stopped→DeployFailed decision under Deploying/Startup.
func mapRunnerResultToStatus(deploy *database.Deploy, r *runnerTypes.BatchStatusItemResult, currentStatus int) (status int, instances []types.Instance, ok bool) {
	switch deploy.Type {
	case types.SandboxType:
		return mapSandboxStatusToDeployStatus(r.Status, currentStatus), nil, true
	case types.SpaceType, types.InferenceType, types.FinetuneType,
		types.ServerlessType, types.NotebookType:
		switch r.Code {
		case common.Running:
			return common.Running, r.Instances, true
		case common.Stopped:
			return common.Stopped, nil, true
		default: // Startup or any other code
			return common.Startup, r.Instances, true
		}
	default:
		return 0, nil, false
	}
}

// shouldApplyKsvcTransition decides whether reconcile should write a status
// transition when the runner disagrees with the DB for a ksvc deploy. Reconcile
// only fixes anomalies; normal transitions are left to the informer.
//
//	Running:   accept Stopped (service genuinely gone); skip Startup
//	           (scale-to-zero is a normal Running state).
//	Deploying: accept DeployFailed (service vanished before ready);
//	           skip Running/Startup (informer owns the transition).
//	Startup:  accept DeployFailed (service vanished before ready);
//	           skip Running/Startup (informer owns the transition).
func shouldApplyKsvcTransition(currentStatus, runnerStatus int) (apply bool, target int) {
	switch currentStatus {
	case common.Running:
		if runnerStatus == common.Stopped {
			return true, common.Stopped
		}
		return false, 0
	case common.Deploying, common.Startup:
		if runnerStatus == common.Stopped {
			return true, common.DeployFailed
		}
		return false, 0
	default:
		// Reconcile only scans Deploying/Startup/Running; any other
		// currentStatus is unexpected — skip to be safe.
		return false, 0
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
	logger := log.With(a.getLogger(ctx),
		"cluster_id", cid,
		"wf_count", len(wfs),
		"hard_timeout", hardTimeout.String(),
	)

	// 1. Heartbeat check — if the cluster is down, apply timeout fallback.
	if checkHeartbeatForCluster(ctx, a, cid) {
		logger.Warn("reconcile(wf): cluster heartbeat timed out, applying fallback")
		marked := 0
		for i := range wfs {
			lastUpdate := wfs[i].StatusUpdateAt
			if lastUpdate.IsZero() {
				lastUpdate = wfs[i].SubmitTime
			}
			elapsed := time.Since(lastUpdate)
			if elapsed > hardTimeout {
				logger.Warn("reconcile(wf): workflow exceeded hard timeout, marking failed",
					"wf_id", wfs[i].ID,
					"last_status_update", lastUpdate.Format(time.RFC3339),
					"elapsed", elapsed.Round(time.Second).String())
				markWorkflowFailed(ctx, a, &wfs[i], cid)
				marked++
			} else {
				logger.Info("reconcile(wf): workflow within hard timeout, skip",
					"wf_id", wfs[i].ID,
					"last_status_update", lastUpdate.Format(time.RFC3339),
					"elapsed", elapsed.Round(time.Second).String())
			}
		}
		logger.Warn("reconcile(wf): heartbeat timeout fallback done",
			"marked", marked, "skipped", len(wfs)-marked)
		return
	}

	// 2. Query runner and process results.
	logger.Info("reconcile(wf): cluster healthy, querying runner")
	clusterBatchDo(ctx, a, cid, wfs,
		func(wf *database.ArgoWorkflow) (string, runnerTypes.BatchStatusItem) {
			return wf.TaskId, runnerTypes.BatchStatusItem{Type: runnerTypes.ResourceTypeWorkflow, Name: wf.TaskId}
		},
		func(wf *database.ArgoWorkflow, r *runnerTypes.BatchStatusItemResult, err error) {
			if err != nil {
				// BatchStatus failed or item error — apply fallback only if
				// the workflow has been stuck beyond hardTimeout.
				lastUpdate := wf.StatusUpdateAt
				if lastUpdate.IsZero() {
					lastUpdate = wf.SubmitTime
				}
				elapsed := time.Since(lastUpdate)
				if elapsed > hardTimeout {
					logger.Warn("reconcile(wf): batch error + hard timeout, marking failed",
						"wf_id", wf.ID,
						"last_status_update", lastUpdate.Format(time.RFC3339),
						"elapsed", elapsed.Round(time.Second).String(),
						"error", err)
					markWorkflowFailed(ctx, a, wf, cid)
				} else {
					logger.Info("reconcile(wf): batch error but within hard timeout, skip",
						"wf_id", wf.ID,
						"last_status_update", lastUpdate.Format(time.RFC3339),
						"elapsed", elapsed.Round(time.Second).String(),
						"error", err)
				}
				return
			}
			if string(wf.Status) != r.Phase && len(r.Phase) > 0 {
				wf.Status = v1alpha1.WorkflowPhase(r.Phase)
				wf.StatusUpdateAt = time.Now()
				if _, err := a.stores.argoWorkFlow.UpdateWorkFlow(ctx, *wf); err != nil {
					logger.Error("reconcile(wf): update failed", "wf_id", wf.ID, "error", err)
				} else {
					logger.Info("reconcile(wf): status synced", "wf_id", wf.ID, "phase", r.Phase)
				}
			}
		},
	)
}

// markWorkflowFailed sets a workflow to WorkflowFailed and persists it.
func markWorkflowFailed(ctx context.Context, a *Activities, wf *database.ArgoWorkflow, cid string) {
	wf.Status = v1alpha1.WorkflowFailed
	wf.StatusUpdateAt = time.Now()
	if _, err := a.stores.argoWorkFlow.UpdateWorkFlow(ctx, *wf); err != nil {
		a.getLogger(ctx).Error("reconcile(wf): mark failed error", "wf_id", wf.ID, "error", err)
	} else {
		a.getLogger(ctx).Info("reconcile(wf): workflow marked failed",
			"wf_id", wf.ID, "cluster_id", cid)
	}
}

// ==================== Shared Cluster Batch Pattern ====================

// clusterBatchDo calls BatchStatus for the given items and dispatches each
// result back to processResult. It does NOT handle heartbeat or timeout
// fallback — the caller owns that, so the flow stays linear and readable.
//
// processResult receives (item, result, err):
//   - err != nil && r == nil → the whole BatchStatus call failed; the runner
//     is unreachable. The caller decides whether to apply timeout fallback.
//   - err != nil && r != nil → this item had an Error field, but the runner
//     was reachable and r may still carry a valid status (e.g. ksvc
//     "service not found" carries Code: Stopped). The caller decides whether
//     the Error is a definitive status or a fallback signal.
//   - err == nil → a normal status result to map and apply.
func clusterBatchDo[T any](
	ctx context.Context, a *Activities, cid string, items []T,
	buildItem func(*T) (key string, item runnerTypes.BatchStatusItem),
	processResult func(*T, *runnerTypes.BatchStatusItemResult, error),
) {
	if a.deployer == nil {
		return
	}

	logger := a.getLogger(ctx)

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
		for i := range items {
			processResult(&items[i], nil, err)
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
			processResult(&items[idx], &r, fmt.Errorf("%s", r.Error))
			continue
		}
		processResult(&items[idx], &r, nil)
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

// checkHeartbeatForCluster reports whether the cluster heartbeat has timed out.
// On check error it returns false (proceed to BatchStatus) and logs a warning.
func checkHeartbeatForCluster(ctx context.Context, a *Activities, cid string) bool {
	timedOut, err := a.deployer.CheckHeartbeatTimeout(ctx, cid)
	if err != nil {
		a.getLogger(ctx).Warn("reconcile: heartbeat check failed, proceed batch",
			"cluster_id", cid, "error", err)
		return false
	}
	return timedOut
}

// applyFallback marks a deploy as ResourceUnhealthy when the runner is
// unreachable and the deploy has been stuck beyond hardTimeout. It skips
// deploys that still have a Pending pod, since "service not found" during
// scheduling is transient. Both fallback paths (heartbeat timeout and
// batch error) use ResourceUnhealthy — we cannot verify the deploy's real
// status, so we do not assert DeployFailed.
func applyFallback(ctx context.Context, a *Activities, d *database.Deploy, expectedStatus, targetStatus int) {
	if hasPendingInstance(d) {
		a.getLogger(ctx).Info("reconcile: skip fallback, pod still pending",
			"deploy_id", d.ID, "svc_name", d.SvcName)
		return
	}
	applyStatusUpdate(ctx, a, d, expectedStatus, targetStatus, nil, "runner_unreachable")
}

// buildDeployBatchItem converts a deploy to a runner BatchStatusItem.
func buildDeployBatchItem(d *database.Deploy) (key string, item runnerTypes.BatchStatusItem) {
	switch d.Type {
	case types.SandboxType:
		return d.SvcName, runnerTypes.BatchStatusItem{Type: runnerTypes.ResourceTypeSandbox, Name: d.SvcName}
	case types.SpaceType, types.InferenceType, types.FinetuneType,
		types.ServerlessType, types.NotebookType:
		return d.SvcName, runnerTypes.BatchStatusItem{Type: runnerTypes.ResourceTypeKsvc, Name: d.SvcName}
	default:
		return d.SvcName, runnerTypes.BatchStatusItem{Type: runnerTypes.ResourceTypeKsvc, Name: d.SvcName}
	}
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
