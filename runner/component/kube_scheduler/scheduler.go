package kube_scheduler

import (
	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"opencsg.com/csghub-server/common/types"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"
)

// Applier defines the interface for applying scheduler configurations
type Applier interface {
	ApplyToKnative(spec *v1.RevisionTemplateSpec, request types.SVCRequest)
	ApplyToLWS(spec *lwsv1.LeaderWorkerSet)
	// ApplyToArgoWithVcjob applies Volcano Job CRD to the template
	ApplyToArgoWithVcjob(template *v1alpha1.Template) error
	// ApplyToArgo applies generic scheduler configuration (schedulerName, queue) to the template
	ApplyToArgo(template *v1alpha1.Template) error
}

// NewApplier creates a new scheduler applier based on the configuration
// implementation is in scheduler_ce.go and scheduler_ee.go

// DefaultOpApplier applies standard Kubernetes configuration
type DefaultOpApplier struct{}

func (a *DefaultOpApplier) ApplyToKnative(spec *v1.RevisionTemplateSpec, request types.SVCRequest) {}
func (a *DefaultOpApplier) ApplyToLWS(spec *lwsv1.LeaderWorkerSet)                                 {}
func (a *DefaultOpApplier) ApplyToArgo(template *v1alpha1.Template) error {
	return nil
}
func (a *DefaultOpApplier) ApplyToArgoWithVcjob(template *v1alpha1.Template) error {
	return nil
}
