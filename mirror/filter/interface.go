package filter

import "context"

type Filter interface {
	ShouldSync(ctx context.Context, repoID int64) (bool, string, error)
}
