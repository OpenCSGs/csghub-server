package component

import (
	"errors"
	"fmt"
	"strings"

	commontypes "opencsg.com/csghub-server/common/types"
)

// ModelIDBuilder abstracts model-id composition/parsing logic so callers can inject mocks in tests.
type ModelIDBuilder interface {
	To(modelName, svcName string) string
	From(modelID string) (modelName, svcName string, err error)
	GetModelOwner(deployType int, username string) string
	BuildCompositeModelID(baseModelID, provider, format string) string
	ParseCompositeModelID(modelID, format string) (baseModelID string)
}

type defaultModelIDBuilder struct{}

// NewModelIDBuilder creates the default model-id builder implementation.
func NewModelIDBuilder() ModelIDBuilder {
	return defaultModelIDBuilder{}
}

func (b defaultModelIDBuilder) To(modelName, svcName string) string {
	return fmt.Sprintf("%s:%s", modelName, svcName)
}

func (b defaultModelIDBuilder) From(modelID string) (modelName, svcName string, err error) {
	strs := strings.Split(modelID, ":")
	if len(strs) > 2 {
		return "", "", errors.New("invalid model id format, should be in format 'model_name:svc_name' or 'model_name'")
	} else if len(strs) < 2 {
		return strs[0], "", nil
	}
	return strs[0], strs[1], nil
}

func (b defaultModelIDBuilder) GetModelOwner(deployType int, username string) string {
	if deployType == commontypes.ServerlessType {
		return "OpenCSG"
	}
	return username
}

func (b defaultModelIDBuilder) BuildCompositeModelID(baseModelID, provider, format string) string {
	return fmt.Sprintf(format, baseModelID, provider)
}

func (b defaultModelIDBuilder) ParseCompositeModelID(modelID, format string) (baseModelID string) {
	// Assuming format is like "%s(%s)"
	// Find prefix and suffix around the two %s
	parts := strings.Split(format, "%s")
	if len(parts) == 3 {
		prefix := parts[0]
		mid := parts[1]
		suffix := parts[2]

		if strings.HasPrefix(modelID, prefix) && strings.HasSuffix(modelID, suffix) {
			inner := modelID[len(prefix) : len(modelID)-len(suffix)]
			if mid != "" {
				idx := strings.LastIndex(inner, mid)
				if idx != -1 {
					return inner[:idx]
				}
			}
		}
	}

	// Fallback to simple extraction for default format "%s(%s)"
	if idx := strings.LastIndex(modelID, "("); idx != -1 && strings.HasSuffix(modelID, ")") {
		return modelID[:idx]
	}
	return modelID
}
