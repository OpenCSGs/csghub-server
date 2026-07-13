package component

import (
	"fmt"
	"strconv"

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

func (b defaultModelIDBuilder) To(deploy database.Deploy) string {
	if deploy.Repository == nil {
		return ""
	}

	switch deploy.Type {
	case commontypes.ServerlessType:
		if deploy.Repository.HFPath != "" {
			return deploy.Repository.HFPath
		}
		return deploy.Repository.Path
	case commontypes.InferenceType:
		return fmt.Sprintf("%s:%s", deploy.Repository.Name, strconv.FormatInt(deploy.ID, 36))
	default:
		return ""
	}
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
