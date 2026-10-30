//go:build !ee && !saas

package activity

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func TestActivities_BatchMigrateToXnet_CE(t *testing.T) {
	activities := &Activities{}

	err := activities.BatchMigrateToXnet(context.Background())
	require.NoError(t, err)
}

// TestActivities_BatchMigrateToXnetForType_CE checks the no-op rejects invalid scopes.
func TestActivities_BatchMigrateToXnetForType_CE(t *testing.T) {
	for _, scenario := range []struct {
		repoType types.RepositoryType
		allowed  bool
	}{
		{repoType: types.ModelRepo, allowed: true},
		{repoType: types.DatasetRepo, allowed: true},
		{repoType: types.CodeRepo, allowed: true},
		{repoType: types.SpaceRepo, allowed: true},
		{repoType: types.MCPServerRepo, allowed: true},
		{repoType: types.SkillRepo, allowed: true},
		{repoType: types.PromptRepo, allowed: true},
		{repoType: types.TemplateRepo, allowed: true},
		{repoType: types.UnknownRepo},
		{repoType: "invalid"},
	} {
		t.Run(fmt.Sprintf("type_%q", scenario.repoType), func(t *testing.T) {
			activities := &Activities{}
			err := activities.BatchMigrateToXnetForType(context.Background(), scenario.repoType)
			if scenario.allowed {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "unsupported repository type")
			}
		})
	}
}
