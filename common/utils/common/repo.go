package common

import (
	"fmt"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

func WithPrefix(name string, prefix string) string {
	return prefix + name
}

func WithoutPrefix(name string, prefix string) string {
	return strings.Replace(name, prefix, "", 1)
}

func BuildCloneURL(url, repoType, namespace, name string) string {
	return fmt.Sprintf("%s/%ss/%s/%s.git", url, repoType, namespace, name)
}

func RemoveOpencsgPrefix(name string) string {
	str, _ := strings.CutPrefix(name, types.OpenCSGPrefix)
	return str
}
