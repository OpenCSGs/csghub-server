package common

import (
	"fmt"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

const (
	ModelOrgPrefix   = "models_"
	DatasetOrgPrefix = "datasets_"
	SpaceOrgPrefix   = "spaces_"
	CodeOrgPrefix    = "codes_"
)

func WithPrefix(name string, prefix string) string {
	return prefix + name
}

func WithoutPrefix(name string, prefix string) string {
	return strings.Replace(name, prefix, "", 1)
}

func ConvertDotToSlash(d string) string {
	if d == "." {
		return "/"
	} else {
		return d
	}
}

func PortalCloneUrl(url string, repoType types.RepositoryType, gitDomain, portalDomain string) string {
	prefix := repoPrefixByType(repoType)
	url = strings.Replace(url, prefix, fmt.Sprintf("%s/", prefix[:len(prefix)-1]), 1)
	url = strings.Replace(url, gitDomain, portalDomain, 1)
	return url
}

func repoPrefixByType(repoType types.RepositoryType) string {
	var prefix string
	switch repoType {
	case types.ModelRepo:
		prefix = ModelOrgPrefix
	case types.DatasetRepo:
		prefix = DatasetOrgPrefix
	case types.SpaceRepo:
		prefix = SpaceOrgPrefix
	case types.CodeRepo:
		prefix = CodeOrgPrefix
	}

	return prefix
}
