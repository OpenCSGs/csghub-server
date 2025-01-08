//go:build !ee && !saas

package component

import (
	"strings"

	"opencsg.com/csghub-server/builder/store/database"
)

func matchRuntimeFrameworkWithEngineEE(rf *database.RuntimeFramework, engine string) bool {
	return false
}

func checkTagName(rf *database.RuntimeFramework, tag string) bool {
	return strings.Contains(rf.FrameImage, tag)
}
