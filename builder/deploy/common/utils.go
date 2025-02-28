package common

import (
	"fmt"
	"strings"
)

func GetNamespaceAndNameFromGitPath(gitpath string) (string, string, error) {
	if gitpath == "" {
		return "", "", fmt.Errorf("empty git path %s", gitpath)
	}
	var fields []string
	idx := strings.Index(gitpath, "_")
	if idx > -1 && idx+1 < len(gitpath) {
		fields = strings.Split(gitpath[idx+1:], "/")
		if len(fields) != 2 {
			return "", "", fmt.Errorf("empty git path %s", gitpath)
		}
	} else {
		return "", "", fmt.Errorf("empty git path %s", gitpath)
	}
	return fields[0], fields[1], nil
}
