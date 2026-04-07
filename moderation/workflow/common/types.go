package common

import (
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

const (
	RepoFullCheckQueue        = "moderation_repo_full_check_queue"
	RepoFullCheckWorkflowName = "RepoFullCheckWorkflow"
)

type Repo struct {
	Namespace string
	Name      string
	RepoType  types.RepositoryType
	Branch    string
}

// RepoFullCheckWorkflowFn is a type definition for the workflow function to allow IDE navigation
// while avoiding import cycles.
type RepoFullCheckWorkflowFn func(ctx workflow.Context, repo Repo, cfg *config.Config) error

// Pointer to the actual workflow function to allow IDE navigation
var RepoFullCheckWorkflow RepoFullCheckWorkflowFn
