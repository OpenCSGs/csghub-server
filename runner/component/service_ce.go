//go:build !ee && !saas

package component

import (
	"context"
	"fmt"

	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/types"
)

func (s *serviceComponentImpl) runServiceMultiHost(ctx context.Context, req types.SVCRequest) error {
	return fmt.Errorf("multi-host inference is not supported")
}

func (s *serviceComponentImpl) RemoveWorkset(ctx context.Context, cluster cluster.Cluster, ksvc *v1.Service) error {
	return nil
}
