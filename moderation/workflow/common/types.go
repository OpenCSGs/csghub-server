package common

import "opencsg.com/csghub-server/common/types"

type Repo struct {
	Namespace string
	Name      string
	RepoType  types.RepositoryType
	Branch    string
}
