//go:build !ee && !saas

package component

import (
	"context"
	"errors"

	"opencsg.com/csghub-server/common/types"
)

func (m *mirrorComponentImpl) Schedule(ctx context.Context) error {
	return nil
}

func (m *mirrorComponentImpl) PublicModelRepo(ctx context.Context) error {
	return nil
}

func (m *mirrorComponentImpl) ResolveNamespace(ctx context.Context, req types.ResolveNamespaceReq) (*types.ResolveNamespaceResp, error) {
	return nil, errors.New("not implemented in CE version")
}
