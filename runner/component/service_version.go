package component

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"knative.dev/serving/pkg/client/informers/externalversions"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	utils "opencsg.com/csghub-server/common/utils/common"
)

func (s *serviceComponentImpl) addOrUpdateRevisionInDB(ctx context.Context, rev *v1.Revision, cluster *cluster.Cluster) error {
	commitID := rev.Labels[CommitId]
	if commitID == "" {
		return nil
	}
	svcName := rev.Labels[KeyServiceLabel]
	services, err := getServices(ctx, cluster, rev.Namespace, svcName)
	if err != nil {
		slog.Error("failed to get services", slog.Any("error", err))
		return err
	}

	trafficPercent := int64(0)
	for _, traffic := range services.Status.Traffic {
		if traffic.RevisionName == rev.Name {
			trafficPercent += int64(*traffic.Percent)
		}
	}

	readyCond := rev.Status.GetCondition(v1.RevisionConditionReady)

	var message, reason string
	if readyCond != nil {
		message = readyCond.Message
		reason = readyCond.Reason
	} else {
		message = "Revision condition not yet reported by controller"
		reason = "ConditionNotFound"
	}
	knativeRevision := &database.KnativeServiceRevision{
		SvcName:        svcName,
		RevisionName:   rev.Name,
		CommitID:       commitID,
		TrafficPercent: trafficPercent,
		IsReady:        rev.IsReady(),
		Message:        message,
		Reason:         reason,
		CreateTime:     rev.CreationTimestamp.Time,
	}

	err = s.revisionStore.AddRevision(ctx, *knativeRevision)
	if err != nil {
		slog.Error("failed to add revision to db", slog.Any("error", err))
		return err
	}
	return nil
}

func (s *serviceComponentImpl) deleteRevisionInDB(ctx context.Context, rev *v1.Revision) error {
	commitID := rev.Labels[CommitId]
	if commitID == "" {
		return nil
	}
	svcName := rev.Labels[KeyServiceLabel]

	revision, err := s.revisionStore.QueryRevision(ctx, svcName, commitID)
	if err != nil {
		slog.Error("failed to delete revision from db", slog.Any("error", err))
		return err
	}

	if revision == nil {
		return nil
	}

	err = s.revisionStore.DeleteRevision(ctx, svcName, commitID)
	if err != nil {
		slog.Error("failed to delete revision from db", slog.Any("error", err))
		return err
	}

	return nil
}

func (s *serviceComponentImpl) runRevisionInformer(stopCh <-chan struct{}, cluster *cluster.Cluster) {
	informerFactory := externalversions.NewSharedInformerFactoryWithOptions(
		cluster.KnativeClient,
		time.Duration(s.informerSyncPeriodInMin)*time.Minute, //sync every 2 minutes, if network unavailable, it will trigger watcher to reconnect
		externalversions.WithNamespace(s.k8sNameSpace),
	)
	informer := informerFactory.Serving().V1().Revisions().Informer()
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			rev := obj.(*v1.Revision)
			slog.Debug("add knative revision by informer", slog.Any("clusterID", cluster.ID), slog.Any("revision", rev.Name))
			ctx, scancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer scancel()
			err := s.addOrUpdateRevisionInDB(ctx, rev, cluster)
			if err != nil {
				slog.Error("failed to add revision by informer add callback", slog.Any("revision", rev.Name), slog.Any("error", err))
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			new := newObj.(*v1.Revision)
			ctx, scancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer scancel()
			err := s.addOrUpdateRevisionInDB(ctx, new, cluster)
			if err != nil {
				slog.Error("failed to update revision status by informer update callback", slog.Any("revision", new.Name), slog.Any("error", err))
			}
		},
		DeleteFunc: func(obj any) {
			rev := obj.(*v1.Revision)
			slog.Debug("delete knative revision by informer", slog.Any("clusterID", cluster.ID), slog.Any("revision", rev.Name))
			ctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer scancel()
			err := s.deleteRevisionInDB(ctx, rev)
			if err != nil {
				slog.Error("failed to delete revision by informer delete callback", slog.Any("revision", rev.Name), slog.Any("error", err))
			}
		},
	})
	if err != nil {
		slog.Error("failed to add revision informer event handler", slog.Any("error", err))
	}

	// Start informer
	informerFactory.Start(stopCh)

	// Wait for cache sync
	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
	}
	<-stopCh
}

// GetServiceExternalStatus determines if a service is externally healthy in a multi-revision deployment
// (as long as at least one revision is ready, the service is considered healthy)
// Returns: external status (True/False/Unknown), list of ready revisions, error
func GetServiceExternalStatus(ctx context.Context, cluster *cluster.Cluster, service *v1.Service, namespace string) (corev1.ConditionStatus, []string, error) {
	if service == nil {
		return corev1.ConditionUnknown, nil, fmt.Errorf("service object is nil")
	}

	// Step 1: Extract all revision names that need to be checked from traffic rules
	revisionNames, err := getTrafficRevisionNames(ctx, cluster, service, namespace)
	if err != nil {
		return corev1.ConditionUnknown, nil, fmt.Errorf("get traffic revision names failed: %w", err)
	}
	if len(revisionNames) <= 1 {
		return getReadyCondition(service), nil, nil
	}

	// Check readiness status of each revision
	readyRevisions := make([]string, 0)
	for _, revName := range revisionNames {
		rev, err := cluster.KnativeClient.ServingV1().Revisions(namespace).Get(ctx, revName, metav1.GetOptions{})
		if err != nil {
			// If querying a single revision fails, skip it (does not affect overall determination)
			continue
		}
		// Check revision readiness status (the core condition for a revision is "Ready")
		revReady := getRevisionReadyCondition(rev)
		if revReady == corev1.ConditionTrue {
			readyRevisions = append(readyRevisions, revName)
		}
	}

	switch {
	case len(readyRevisions) > 0:
		// As long as one revision is ready, the external service is healthy
		return corev1.ConditionTrue, readyRevisions, nil
	case len(revisionNames) == len(readyRevisions):
		// All revisions have been queried and none are ready
		return corev1.ConditionFalse, nil, nil
	default:
		// Some revision queries failed/status unknown
		return corev1.ConditionUnknown, nil, nil
	}
}

// corev1.ConditionTrue
func getReadyCondition(service *v1.Service) corev1.ConditionStatus {
	for _, condition := range service.Status.Conditions {
		if condition.Type == v1.ServiceConditionReady {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}

// getTrafficRevisionNames extracts all revision names with assigned traffic from KSVC.Spec.Traffic
func getTrafficRevisionNames(ctx context.Context, cluster *cluster.Cluster, service *v1.Service, namespace string) ([]string, error) {
	revisionNames := make([]string, 0)
	for _, traffic := range service.Spec.Traffic {
		if traffic.RevisionName != "" {
			// Scenario 1: Traffic points to a specific revision (e.g., old version)
			revisionNames = append(revisionNames, traffic.RevisionName)
		} else if traffic.LatestRevision != nil && *traffic.LatestRevision {
			// Scenario 2: Traffic points to the latest revision → query the latest revision name for KSVC
			latestRevName, err := getLatestRevisionName(ctx, cluster, service, namespace)
			if err != nil {
				continue
			}
			if latestRevName != "" {
				revisionNames = append(revisionNames, latestRevName)
			}
		}
	}
	return revisionNames, nil
}

// getLatestRevisionName gets the latest revision name associated with KSVC
func getLatestRevisionName(ctx context.Context, cluster *cluster.Cluster, service *v1.Service, namespace string) (string, error) {
	// Extract the latest revision name from KSVC.Status (Knative updates it automatically)
	if service.Status.LatestCreatedRevisionName != "" {
		return service.Status.LatestCreatedRevisionName, nil
	}
	// Fallback: query all revisions associated with KSVC and pick the latest created
	revisionList, err := cluster.KnativeClient.ServingV1().Revisions(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("serving.knative.dev/service=%s", service.Name),
	})
	if err != nil {
		return "", err
	}
	if len(revisionList.Items) == 0 {
		return "", fmt.Errorf("no revision found for service %s", service.Name)
	}
	// Sort by creation timestamp in descending order, take the first (latest)
	latestRev := revisionList.Items[0]
	for _, rev := range revisionList.Items {
		if rev.CreationTimestamp.After(latestRev.CreationTimestamp.Time) {
			latestRev = rev
		}
	}
	return latestRev.Name, nil
}

// getRevisionReadyCondition extracts the readiness status of a single revision
func getRevisionReadyCondition(rev *v1.Revision) corev1.ConditionStatus {
	if rev == nil {
		return corev1.ConditionUnknown
	}
	for _, condition := range rev.Status.Conditions {
		if condition.Type == v1.RevisionConditionReady {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}

func getServices(ctx context.Context, cluster *cluster.Cluster, namespace, svcName string) (*v1.Service, error) {
	ksvc, err := cluster.KnativeClient.ServingV1().Services(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		slog.ErrorContext(ctx, "fail to get service", slog.String("svcName", svcName), slog.String("namespace", namespace), slog.Any("error", err))
		return nil, fmt.Errorf("fail to get service %s, error %v ", svcName, err)
	}

	return ksvc, nil
}

func getRevisionList(ctx context.Context, cluster *cluster.Cluster, namespace, svcName string) (*v1.RevisionList, error) {
	labelSelector := fmt.Sprintf("serving.knative.dev/service=%s", svcName)
	revisionList, err := cluster.KnativeClient.ServingV1().Revisions(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		slog.ErrorContext(ctx, "fail to list revisions for service", slog.String("svcName", svcName), slog.String("nameSpace", namespace), slog.Any("error", err))
		return nil, fmt.Errorf("fail to list revisions for service %s, error %w ", svcName, err)
	}

	var filteredItems []v1.Revision
	for _, rev := range revisionList.Items {
		if rev.DeletionTimestamp == nil {
			filteredItems = append(filteredItems, rev)
		}
	}

	revisionList.Items = filteredItems
	return revisionList, nil
}

// validateTrafficReqByCommit
func validateTrafficReqByCommit(ctx context.Context, req []types.TrafficReq, commitToRevision map[string]v1.Revision) error {
	totalPercent := int64(0)
	for _, r := range req {
		totalPercent += r.TrafficPercent

		if r.TrafficPercent < 0 || r.TrafficPercent > 100 {
			slog.WarnContext(ctx, "t'ratraffic percent out of range", slog.String("commit", r.Commit), slog.Int64("percent", r.TrafficPercent))
			return errorx.ErrInvalidPercent
		}

		if rev, exists := commitToRevision[r.Commit]; !exists {
			slog.WarnContext(ctx, "commit not found in revision map", slog.String("commit", r.Commit))
			return errorx.ErrRevisionNotFound
		} else {
			if !rev.IsReady() {
				slog.WarnContext(ctx, "revision not ready", slog.String("commit", r.Commit), slog.String("revision", rev.Name))
				return errorx.ErrRevisionNotReady
			}
		}

	}

	if totalPercent != 100 {
		slog.WarnContext(ctx, "traffic percent sum not equal 100", slog.Int64("sum", totalPercent))
		return errorx.ErrInvalidPercent
	}

	return nil
}

func buildTrafficTargetsByCommit(ctx context.Context, req []types.TrafficReq, commitToRevision map[string]v1.Revision) ([]v1.TrafficTarget, error) {
	trafficTargets := make([]v1.TrafficTarget, 0, len(req))
	for _, r := range req {
		rev, exists := commitToRevision[r.Commit]
		if !exists {
			slog.WarnContext(ctx, "commit not found in revision map", slog.String("commit", r.Commit))
			return nil, fmt.Errorf("commit %s not found in revision map", r.Commit)
		}

		target := v1.TrafficTarget{
			RevisionName:   rev.Name,
			LatestRevision: utils.BoolPtr(false),
			Percent:        utils.Int64Ptr(r.TrafficPercent),
		}
		trafficTargets = append(trafficTargets, target)
	}
	return trafficTargets, nil
}

// buildCommitRevisionMap
func buildCommitRevisionMap(revisionList *v1.RevisionList) (map[string]v1.Revision, error) {
	commitToRevision := make(map[string]v1.Revision)
	for _, rev := range revisionList.Items {
		if rev.Labels == nil {
			continue
		}
		commit := rev.Labels[CommitId]
		if commit == "" {
			continue
		}
		commitToRevision[commit] = rev
	}

	if len(commitToRevision) == 0 {
		return nil, fmt.Errorf("fail to build commit revision map, no valid revision found")
	}
	return commitToRevision, nil
}
