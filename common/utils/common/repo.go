package common

import (
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
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

func BuildCloneInfo(config *config.Config, repository *database.Repository) types.Repository {
	return types.Repository{
		HTTPCloneURL: buildCloneURL(config.APIServer.PublicDomain, repository.RepositoryType, repository.Path),
		SSHCloneURL:  buildCloneURL(config.APIServer.SSHDomain, repository.RepositoryType, repository.Path),
	}
}

func buildCloneURL(domain string, repoType types.RepositoryType, path string) string {
	return fmt.Sprintf("%s/%ss/%s.git", strings.TrimSuffix(domain, "/"), repoType, path)
}
