package component

import (
	"errors"
	"fmt"
	"strings"
)

type ModelIDBuilder struct {
}

func (b ModelIDBuilder) To(modelName, svcName string) string {
	return fmt.Sprintf("%s:%s", modelName, svcName)
}

func (b ModelIDBuilder) From(modelID string) (modelName, svcName string, err error) {
	strs := strings.Split(modelID, ":")
	if len(strs) > 2 {
		return "", "", errors.New("invalid model id format, should be in format 'model_name:svc_name' or 'model_name'")
	} else if len(strs) < 2 {
		return strs[0], "", nil
	}
	return strs[0], strs[1], nil
}
