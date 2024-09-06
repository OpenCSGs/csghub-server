package gitaly

import "strings"

func BuildRelativePath(repoType, namespace, name string) string {
	return strings.ToLower(repoType + "_" + namespace + "/" + name + ".git")
}
