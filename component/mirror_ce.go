//go:build !ee && !saas

package component

import (
	"context"
)

func (c *mirrorComponentImpl) Schedule(ctx context.Context) error {
	return nil
}

func (c *mirrorComponentImpl) PublicModelRepo(ctx context.Context) error {
	return nil
}
