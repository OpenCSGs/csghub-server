//go:build !ee && !saas

package activity

import (
	"context"
)

func (a *Activities) BatchMigrateToXnet(ctx context.Context) error {
	return nil
}
