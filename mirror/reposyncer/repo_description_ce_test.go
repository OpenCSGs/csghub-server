//go:build !saas

package reposyncer

import (
	"testing"

	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func expectDescriptionGeneration(
	t *testing.T,
	mockGit *mockgit.MockGitServer,
	mockRepoStore *mockdb.MockRepoStore,
	mockPromptPrefixStore *mockdb.MockPromptPrefixStore,
	mockLLMConfigStore *mockdb.MockLLMConfigStore,
	repoType types.RepositoryType,
	namespace, name, ref string,
	descriptionErr error,
) {
}
