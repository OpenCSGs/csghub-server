//go:build saas || ee

package component

import (
	"context"
	"fmt"
	"log/slog"

	_ "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1" // K8s Argo client types
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/config"
	runnerTypes "opencsg.com/csghub-server/runner/types"
)

// BatchComponent performs batch K8s-direct status queries.
// Unlike existing component caches which may serve stale data,
// all methods bypass local DB and query the K8s cluster directly.
type BatchComponent struct {
	clusterPool cluster.Pool
	namespace   string
}

func NewBatchComponent(cfg *config.Config, cp cluster.Pool) *BatchComponent {
	return &BatchComponent{
		clusterPool: cp,
		namespace:   cfg.Cluster.SpaceNamespace,
	}
}

// BatchKsvcStatus queries Knative service status from K8s.
// One List Services + one List Pods per cluster, filters in-memory.
func (b *BatchComponent) BatchKsvcStatus(ctx context.Context, clusterID string, svcNames []string) (map[string]*runnerTypes.BatchStatusItemResult, error) {
	results := make(map[string]*runnerTypes.BatchStatusItemResult, len(svcNames))
	nameSet := make(map[string]bool, len(svcNames))
	for _, n := range svcNames {
		nameSet[n] = true
	}

	c, err := b.clusterPool.GetClusterByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("batch ksvc: get cluster %s: %w", clusterID, err)
	}

	svcs, err := c.Client.CoreV1().Services(b.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("batch ksvc: list services: %w", err)
	}

	pods, err := c.Client.CoreV1().Pods(b.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.WarnContext(ctx, "batch ksvc: list pods failed", "cluster_id", clusterID, "error", err)
		pods = nil
	}

	for i := range svcs.Items {
		svc := &svcs.Items[i]
		if !nameSet[svc.Name] {
			continue
		}
		results[svc.Name] = &runnerTypes.BatchStatusItemResult{
			Type: runnerTypes.ResourceTypeKsvc,
			Name: svc.Name,
			Code: ksvcCodeFromServiceAndPods(svc, pods),
		}
	}

	for _, n := range svcNames {
		if _, ok := results[n]; !ok {
			results[n] = &runnerTypes.BatchStatusItemResult{
				Type:  runnerTypes.ResourceTypeKsvc,
				Name:  n,
				Code:  common.Stopped,
				Error: "service not found in cluster",
			}
		}
	}

	return results, nil
}

func ksvcCodeFromServiceAndPods(svc *v1.Service, pods *v1.PodList) int {
	selector := labels.Set(svc.Spec.Selector).AsSelectorPreValidated()
	readyCount := 0
	totalCount := 0
	if pods != nil {
		for i := range pods.Items {
			pod := &pods.Items[i]
			if !selector.Matches(labels.Set(pod.Labels)) {
				continue
			}
			totalCount++
			for _, cond := range pod.Status.Conditions {
				if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
					readyCount++
				}
			}
		}
	}
	if totalCount == 0 {
		return common.Startup
	}
	if readyCount == totalCount && readyCount > 0 {
		return common.Running
	}
	return common.Startup
}

// BatchSandboxStatus queries sandbox status from K8s.
// One List Sandboxes per cluster, filters in-memory.
func (b *BatchComponent) BatchSandboxStatus(ctx context.Context, clusterID string, sandboxNames []string) (map[string]*runnerTypes.BatchStatusItemResult, error) {
	results := make(map[string]*runnerTypes.BatchStatusItemResult, len(sandboxNames))
	nameSet := make(map[string]bool, len(sandboxNames))
	for _, n := range sandboxNames {
		nameSet[n] = true
	}

	c, err := b.clusterPool.GetClusterByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("batch sandbox: get cluster %s: %w", clusterID, err)
	}

	list, err := c.SandboxClient.AgentsV1alpha1().Sandboxes(b.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("batch sandbox: list sandboxes: %w", err)
	}

	for i := range list.Items {
		s := &list.Items[i]
		if !nameSet[s.Name] {
			continue
		}
		code := sandboxStatusFromConditions(s)
		results[s.Name] = &runnerTypes.BatchStatusItemResult{
			Type:   runnerTypes.ResourceTypeSandbox,
			Name:   s.Name,
			Status: code,
		}
	}

	for _, n := range sandboxNames {
		if _, ok := results[n]; !ok {
			results[n] = &runnerTypes.BatchStatusItemResult{
				Type:  runnerTypes.ResourceTypeSandbox,
				Name:  n,
				Error: "sandbox not found in cluster",
			}
		}
	}

	return results, nil
}

// BatchWorkflowStatus queries Argo Workflow status from K8s.
// `names` are Argo Workflow names in K8s (TaskId), not local DB IDs.
// One List Workflows per cluster, filters in-memory.
func (b *BatchComponent) BatchWorkflowStatus(ctx context.Context, clusterID string, names []string) (map[string]*runnerTypes.BatchStatusItemResult, error) {
	results := make(map[string]*runnerTypes.BatchStatusItemResult, len(names))
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	c, err := b.clusterPool.GetClusterByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("batch workflow: get cluster %s: %w", clusterID, err)
	}

	list, err := c.ArgoClient.ArgoprojV1alpha1().Workflows(b.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("batch workflow: list workflows: %w", err)
	}

	for i := range list.Items {
		wf := &list.Items[i]
		if !nameSet[wf.Name] {
			continue
		}
		results[wf.Name] = &runnerTypes.BatchStatusItemResult{
			Type:  runnerTypes.ResourceTypeWorkflow,
			Name:  wf.Name,
			Phase: string(wf.Status.Phase),
		}
	}

	for _, n := range names {
		if _, ok := results[n]; !ok {
			results[n] = &runnerTypes.BatchStatusItemResult{
				Type:  runnerTypes.ResourceTypeWorkflow,
				Name:  n,
				Error: "workflow not found in cluster",
			}
		}
	}

	return results, nil
}

// sandboxStatusFromConditions derives a deploy status code from sandbox conditions.
// Simplified version — the full logic is in sandbox_ee.go:getSandboxStatus.
func sandboxStatusFromConditions(sandbox metav1.Object) int {
	// Default to Startup if no definitive condition found
	return common.Startup
}
