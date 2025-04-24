package gitaly

import (
	"opencsg.com/csghub-server/common/utils/common"
)

func BuildRelativePath(repoType, namespace, name string) string {
	return common.BuildRelativePath(repoType, namespace, name) + ".git"
}
