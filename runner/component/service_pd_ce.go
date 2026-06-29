//go:build !ee && !saas

package component

import (
	"context"
	"fmt"

	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/types"
)

func (s *serviceComponentImpl) runServicePD(ctx context.Context, req types.SVCRequest) error {
	return fmt.Errorf("PD disaggregation is not supported")
}

// removePDWorkset is unused in CE build but required for compilation.
// It's called from service_ee.go RemoveWorkset in EE build.
//
//nolint:unused
func (s *serviceComponentImpl) removePDWorkset(ctx context.Context, cluster cluster.Cluster, ksvc *v1.Service) error {
	return nil
}
