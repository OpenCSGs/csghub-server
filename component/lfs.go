package component

import "context"

type LfsComponent interface {
	DispatchLfsXnetProgress() error
	DispatchLfsXnetResult() error
	PublishLfsMigrationMessage(ctx context.Context, repoID int64, oid string) error
}
