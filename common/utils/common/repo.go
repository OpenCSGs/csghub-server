package common

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
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
		HTTPCloneURL: buildHTTPCloneURL(config.APIServer.PublicDomain, repository.RepositoryType, repository.Path),
		SSHCloneURL:  buildSSHCloneURL(config.APIServer.SSHDomain, repository.RepositoryType, repository.Path),
	}
}

func buildHTTPCloneURL(domain string, repoType types.RepositoryType, path string) string {
	return fmt.Sprintf("%s/%ss/%s.git", strings.TrimSuffix(domain, "/"), repoType, path)
}

func buildSSHCloneURL(domain string, repoType types.RepositoryType, path string) string {
	parsedURL, err := url.Parse(domain)
	if err != nil {
		return ""
	}
	sshDomainWithoutPrefix := strings.TrimPrefix(domain, "ssh://")

	if parsedURL.Port() == "" {
		return fmt.Sprintf("%s:%ss/%s.git", strings.TrimSuffix(sshDomainWithoutPrefix, "/"), repoType, path)
	} else {
		return fmt.Sprintf("ssh://%s/%ss/%s.git", strings.TrimSuffix(sshDomainWithoutPrefix, "/"), repoType, path)
	}
}

func IsValidName(name string) (bool, error) {
	// validate name
	if err := validate(name); err != nil {
		return false, err
	}
	// repeat special character check
	if hasRepeatSpecialCharacter(name) {
		return false, errors.New("Name contains consecutive special characters which is not allowed.")
	}
	return true, nil
}

func hasRepeatSpecialCharacter(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if strings.Contains("-_.", string(s[i])) && s[i] == s[i+1] {
			return true
		}
	}
	return false
}

func validate(name string) error {
	rules := []struct {
		pattern *regexp.Regexp
		message string
	}{
		// Length mast between 2 and 64
		{
			pattern: regexp.MustCompile(`^.{2,64}$`),
			message: "Length must be between 2 and 64 characters.",
		},
		// Must start with a letter
		{
			pattern: regexp.MustCompile(`^[a-zA-Z]`),
			message: "Must start with a letter.",
		},
		// Must end with a letter or number
		{
			pattern: regexp.MustCompile(`[a-zA-Z0-9]$`),
			message: "Must end with a letter or number.",
		},
		// Only letters, numbers, and -_. are allowed
		{
			pattern: regexp.MustCompile(`^[a-zA-Z0-9-_\.]+$`),
			message: "Only letters, numbers, and -_. are allowed.",
		},
		// Final regex check
		{
			pattern: regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-_\.]*[a-zA-Z0-9]$`),
			message: "Name does not match the required format.",
		},
	}

	// Validate name
	for _, rule := range rules {
		if !rule.pattern.MatchString(name) {
			return errors.New(rule.message)
		}
	}

	return nil
}
