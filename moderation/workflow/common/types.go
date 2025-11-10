package common

import "opencsg.com/csghub-server/common/types"

const (
	RepoFullCheckQueue = "moderation_repo_full_check_queue"
)

type Repo struct {
	Namespace string
	Name      string
	RepoType  types.RepositoryType
	Branch    string
}
