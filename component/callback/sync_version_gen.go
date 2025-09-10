package callback

import (
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/filter"
	"opencsg.com/csghub-server/common/types"
)

type SyncVersionGenerator interface {
	GenSyncVersion(req *types.GiteaCallbackPushReq) error
}

type syncVersionGeneratorImpl struct {
	multiSyncStore database.MultiSyncStore
	ruleStore      database.RuleStore
	repoStore      database.RepoStore
	repoFilter     filter.RepoFilter
}

func NewSyncVersionGenerator() *syncVersionGeneratorImpl {
	return &syncVersionGeneratorImpl{
		multiSyncStore: database.NewMultiSyncStore(),
		ruleStore:      database.NewRuleStore(),
		repoStore:      database.NewRepoStore(),
		repoFilter:     filter.NewRepoFilter(),
	}
}
