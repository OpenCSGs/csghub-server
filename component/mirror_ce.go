//go:build !ee && !saas

package component

import (
	"context"
)

func (m *mirrorComponentImpl) Schedule(ctx context.Context) error {
	return nil
}

func (m *mirrorComponentImpl) PublicModelRepo(ctx context.Context) error {
	return nil
}
