//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
)

func SyncAgentTemplateSensitiveCheckResult(context.Context, *database.Repository, database.AgentTemplateStore, database.RepoFileCheckStore) error {
	return nil
}
