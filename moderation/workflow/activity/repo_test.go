package activity

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestCheckRepoFiles_PrivateRepo(t *testing.T) {
	// Private repos must also be checked per policy compliance.
	// The activity should NOT skip private repos — it proceeds to create
	// a repo component just like public repos. With an empty config this
	// fails, confirming the activity does not early-return for private repos.
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(CheckRepoFiles)

	privateRepo := &database.Repository{
		ID:             1,
		Path:           "user/private-repo",
		DefaultBranch:  "main",
		Name:           "private-repo",
		RepositoryType: types.ModelRepo,
		Private:        true,
	}

	cfg := &config.Config{}

	_, err := env.ExecuteActivity(CheckRepoFiles, privateRepo, cfg)
	// Expect an error because NewRepoComponent will fail without proper
	// config. The key assertion is that we get an error (not nil) — the
	// activity did NOT skip the private repo.
	require.Error(t, err)
}

func TestCheckRepoFiles_PublicRepo(t *testing.T) {
	// For a public repo, the activity will attempt to create a real repo
	// component (NewRepoComponent), which requires database/git server
	// connections. This path is already covered by the workflow-level tests
	// in repo_full_check_test.go where the activity is mocked via
	// env.OnActivity. Here we only verify that a public repo proceeds past
	// component creation, which fails without a real config.
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(CheckRepoFiles)

	publicRepo := &database.Repository{
		ID:             2,
		Path:           "user/public-repo",
		DefaultBranch:  "main",
		Name:           "public-repo",
		RepositoryType: types.ModelRepo,
		Private:        false,
	}

	cfg := &config.Config{}

	_, err := env.ExecuteActivity(CheckRepoFiles, publicRepo, cfg)
	// Expect an error because NewRepoComponent will fail without proper
	// config (no git server, no database).
	require.Error(t, err)
}
