package upstream

import (
	"fmt"

	deploybuilder "opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

// ModelIDBuilder abstracts model-id composition/parsing logic so callers can inject mocks in tests.
type ModelIDBuilder interface {
	To(deploy database.Deploy) string
	ToLegacyCSGHubModelID(repo *database.Repository, svcName string) string
	GetModelOwner(deployType int, username string) string
}

type defaultModelIDBuilder struct{}

// NewModelIDBuilder creates the default model-id builder implementation.
func NewModelIDBuilder() ModelIDBuilder {
	return defaultModelIDBuilder{}
}

// To delegates to the shared BuildModelID so the trigger side
// (BuildDeployUpstreamInfoWithDeploy) and the consumer side
// (buildInternalModel) produce identical model IDs. For unknown deploy
// types, BuildModelID returns "" and the consumer's buildInternalModel
// falls back to LegacyModelID (pre-computed on the trigger side).
func (b defaultModelIDBuilder) To(deploy database.Deploy) string {
	return deploybuilder.BuildModelID(&deploy)
}

func (b defaultModelIDBuilder) ToLegacyCSGHubModelID(repo *database.Repository, svcName string) string {
	modelName := ""
	if repo != nil {
		if repo.HFPath != "" {
			modelName = repo.HFPath
		} else {
			modelName = repo.Path
		}
	}
	return fmt.Sprintf("%s:%s", modelName, svcName)
}

func (b defaultModelIDBuilder) GetModelOwner(deployType int, username string) string {
	if deployType == commontypes.ServerlessType {
		return "OpenCSG"
	}
	return username
}
