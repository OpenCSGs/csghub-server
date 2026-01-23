package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
)

type LfsComponent interface {
	DispatchLfsXnetProgress() error
	DispatchLfsXnetResult() error
	PublishLfsMigrationMessage(ctx context.Context, repo *database.Repository, oid string) error
}
