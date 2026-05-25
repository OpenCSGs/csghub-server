package component

import (
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type UpdateRepoDescriptionFromReadmeReq struct {
	RepoStore         database.RepoStore
	GitServer         gitserver.GitServer
	PromptPrefixStore database.PromptPrefixStore
	LLMConfigStore    database.LLMConfigStore
	RepoType          types.RepositoryType
	Namespace         string
	Name              string
	Ref               string
}
