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

var RevisionState string = "serving.knative.dev/revision-state"

func isRevisionDeleted(rev *v1.Revision) bool {
	if rev.Annotations == nil || rev.Annotations[RevisionState] == "deleted" {
		return true
	}
	return false
}

func (s *serviceComponentImpl) addOrUpdateRevisionInDB(ctx context.Context, rev *v1.Revision, cluster *cluster.Cluster) error {
	commitID := rev.Labels[CommitId]
	if commitID == "" {
		return nil
	}
	svcName := rev.Labels[KeyServiceLabel]
	services, err := getServices(ctx, cluster, rev.Namespace, svcName)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get services", slog.Any("error", err), slog.String("svcName", svcName), slog.String("namespace", rev.Namespace), slog.String("clusterID", cluster.ID))
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
		slog.ErrorContext(ctx, "failed to add revision to db", slog.Any("error", err), slog.String("svcName", svcName), slog.String("revisionName", rev.Name), slog.String("commitID", commitID), slog.String("clusterID", cluster.ID))
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
		slog.ErrorContext(ctx, "failed to delete revision from db", slog.Any("error", err), slog.String("svcName", svcName), slog.String("commitID", commitID))
		return err
	}

	if revision == nil {
		return nil
	}

	err = s.revisionStore.DeleteRevision(ctx, svcName, commitID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete revision from db", slog.Any("error", err), slog.String("svcName", svcName), slog.String("commitID", commitID))
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
			ctx, scancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer scancel()
			if isRevisionDeleted(rev) {
				err := s.deleteRevisionInDB(ctx, rev)
				if err != nil {
					slog.ErrorContext(ctx, "failed to delete revision by informer add callback", slog.String("revision", rev.Name), slog.Any("error", err), slog.String("clusterID", cluster.ID), slog.String("svcName", rev.Labels[KeyServiceLabel]))
				}
				return
			} else {
				err := s.addOrUpdateRevisionInDB(ctx, rev, cluster)
				if err != nil {
					slog.ErrorContext(ctx, "failed to add revision by informer add callback", slog.String("revision", rev.Name), slog.Any("error", err), slog.String("clusterID", cluster.ID), slog.String("svcName", rev.Labels[KeyServiceLabel]))
				}
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			new := newObj.(*v1.Revision)
			ctx, scancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer scancel()
			if isRevisionDeleted(new) {
				err := s.deleteRevisionInDB(ctx, new)
				if err != nil {
					slog.ErrorContext(ctx, "failed to delete revision by informer update callback", slog.String("revision", new.Name), slog.Any("error", err), slog.String("clusterID", cluster.ID), slog.String("svcName", new.Labels[KeyServiceLabel]))
				}
			} else {
				err := s.addOrUpdateRevisionInDB(ctx, new, cluster)
				if err != nil {
					slog.ErrorContext(ctx, "failed to update revision status by informer update callback", slog.String("revision", new.Name), slog.Any("error", err), slog.String("clusterID", cluster.ID), slog.String("svcName", new.Labels[KeyServiceLabel]))
				}
			}
		},
		DeleteFunc: func(obj any) {
			rev := obj.(*v1.Revision)
			ctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer scancel()
			err := s.deleteRevisionInDB(ctx, rev)
			if err != nil {
				slog.ErrorContext(ctx, "failed to delete revision by informer delete callback", slog.String("revision", rev.Name), slog.Any("error", err), slog.String("clusterID", cluster.ID), slog.String("svcName", rev.Labels[KeyServiceLabel]))
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
	revisionList, err := getRevisionList(ctx, cluster, namespace, service.Name)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get revision list", slog.String("ksvc_name", service.Name), slog.Any("error", err), slog.String("namespace", namespace))
		return corev1.ConditionUnknown, nil, fmt.Errorf("get revision list failed: %w", err)
	}

	slog.InfoContext(ctx, "get traffic revision names", slog.String("ksvc_name", service.Name), slog.Int("revisionList_len", len(revisionList.Items)), slog.String("namespace", namespace))
	if len(revisionList.Items) <= 1 {
		return getReadyCondition(service), nil, nil
	}

	// Check readiness status of each revision
	readyRevisions := make([]string, 0)
	for _, rev := range revisionList.Items {
		if isRevisionDeleted(&rev) {
			continue
		}
		revName := rev.Name
		// Check revision readiness status (the core condition for a revision is "Ready")
		revReady := getRevisionReadyCondition(&rev)
		if revReady == corev1.ConditionTrue {
			readyRevisions = append(readyRevisions, revName)
		}
	}

	switch {
	case len(readyRevisions) > 0:
		// As long as one revision is ready, the external service is healthy
		return corev1.ConditionTrue, readyRevisions, nil
	case len(revisionList.Items) == len(readyRevisions):
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
		slog.ErrorContext(ctx, "fail to get service", slog.String("svcName", svcName), slog.String("namespace", namespace), slog.Any("error", err), slog.String("clusterID", cluster.ID))
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
		slog.ErrorContext(ctx, "fail to list revisions for service", slog.String("svcName", svcName), slog.String("namespace", namespace), slog.Any("error", err), slog.String("clusterID", cluster.ID))
		return nil, fmt.Errorf("fail to list revisions for service %s, error %w ", svcName, err)
	}

	var filteredItems []v1.Revision
	for _, rev := range revisionList.Items {
		if isRevisionDeleted(&rev) {
			continue
		}
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

// TrafficFixAlgorithm Core algorithm for traffic correction
// Parameters:
//
//	oldTraffic: Original KSVC Traffic configuration
//	delRevisionName: Name of the Revision to be deleted
//
// Returns:
//
//	fixedTraffic: Corrected compliant Traffic configuration (sum 100%)
//	isValid: Whether the correction was successful
func TrafficFixAlgorithm(oldTraffic []v1.TrafficTarget, delRevisionName string) (fixedTraffic []v1.TrafficTarget, isValid bool) {
	// Step 1: Filter out the version to be deleted to get valid Traffic list
	var validTraffic []v1.TrafficTarget
	for _, item := range oldTraffic {
		// Skip the Revision to be deleted, keep other valid configurations
		if item.RevisionName == delRevisionName || item.RevisionName == "" {
			continue
		}
		// Handle nil pointer (avoid null pointer exception)
		if item.Percent == nil {
			item.Percent = new(int64)
			*item.Percent = 0
		}
		validTraffic = append(validTraffic, item)
	}

	// Step 2: Handle boundary case - no valid Traffic
	if len(validTraffic) == 0 {
		// Fallback configuration: Point to the latest Revision, allocate 100% traffic (ensure service availability)
		fixedItem := v1.TrafficTarget{
			LatestRevision: new(bool),
			Percent:        new(int64),
		}
		*fixedItem.LatestRevision = true
		*fixedItem.Percent = 100
		return []v1.TrafficTarget{fixedItem}, true
	}

	// Step 3: Calculate the total traffic of valid Traffic
	var trafficSum int64
	for _, item := range validTraffic {
		trafficSum += *item.Percent
	}

	// Step 4: Correct traffic based on scenarios, ensure sum is 100%
	var resultTraffic []v1.TrafficTarget
	switch len(validTraffic) {
	case 1:
		// Scenario 1: Only 1 valid version remains → directly set to 100%
		singleItem := validTraffic[0]
		*singleItem.Percent = 100
		resultTraffic = append(resultTraffic, singleItem)
	default:
		// Scenario 2: Multiple valid versions remain → normalize proportionally, last version takes the rest
		remaining := int64(100)
		// Process the first N-1 versions first, distribute traffic proportionally to original ratios
		for i := 0; i < len(validTraffic)-1; i++ {
			item := validTraffic[i]
			if trafficSum == 0 {
				*item.Percent = 0
			} else {
				// Calculate proportionally (take integer, avoid decimals)
				percent := (*item.Percent * 100) / trafficSum
				*item.Percent = percent
				remaining -= percent
			}
			resultTraffic = append(resultTraffic, item)
		}
		// Last version takes all remaining traffic to ensure sum is 100%
		lastItem := validTraffic[len(validTraffic)-1]
		*lastItem.Percent = remaining
		resultTraffic = append(resultTraffic, lastItem)
	}

	// Step 5: Double check - ensure the corrected sum is 100%
	var finalSum int64
	for _, item := range resultTraffic {
		finalSum += *item.Percent
	}
	if finalSum != 100 {
		return nil, false
	}

	return resultTraffic, true
}
