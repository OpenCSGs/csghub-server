//go:build !ee && !saas

package activity

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/types"
)

// BatchMigrateToXnet preserves the no-op activity used by the community edition.
func (a *Activities) BatchMigrateToXnet(ctx context.Context) error {
	return nil
}

// BatchMigrateToXnetForType validates the requested kind before the community edition no-op.
func (a *Activities) BatchMigrateToXnetForType(ctx context.Context, repoType types.RepositoryType) error {
	if !repoType.IsValid() && repoType != types.SkillRepo {
		return fmt.Errorf("unsupported repository type for xnet migration: %q", repoType)
	}
	return nil
}
