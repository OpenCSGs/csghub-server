package activity

import "context"

func (a *Activities) DeletePendingDeletion(ctx context.Context) error {
	return a.repoComponent.DeletePendingDeletion(ctx)
}
