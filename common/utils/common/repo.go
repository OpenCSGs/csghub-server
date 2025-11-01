package common

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/enum"
)

const (
	ModelOrgPrefix       = "models_"
	DatasetOrgPrefix     = "datasets_"
	SpaceOrgPrefix       = "spaces_"
	CodeOrgPrefix        = "codes_"
	HashedRepoPathPrefix = "@hashed_repos"
)

const (
	CSGSourceType = "csghub"
	HFSourceType  = "huggingface"
	MSSourceType  = "modelscope"
)

var MirrorTaskStatusToRepoStatusMap = map[types.MirrorTaskStatus]types.RepositorySyncStatus{
	types.MirrorQueued:           types.SyncStatusPending,
	types.MirrorRepoSyncStart:    types.SyncStatusInProgress,
	types.MirrorRepoSyncFailed:   types.SyncStatusFailed,
	types.MirrorRepoSyncFinished: types.SyncStatusInProgress,
	types.MirrorRepoSyncFatal:    types.SyncStatusFailed,
	types.MirrorLfsSyncStart:     types.SyncStatusInProgress,
	types.MirrorLfsSyncFailed:    types.SyncStatusFailed,
	types.MirrorLfsSyncFinished:  types.SyncStatusCompleted,
	types.MirrorLfsSyncFatal:     types.SyncStatusFailed,
	types.MirrorLfsIncomplete:    types.SyncStatusFailed,
	types.MirrorCanceled:         types.SyncStatusCanceled,

	types.MirrorRepoTooLarge: types.SyncStatusFailed,
}

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

func BuildCloneInfoByDomain(publicDomain, sshDomain string, repository *database.Repository) types.Repository {
	return types.Repository{
		HTTPCloneURL: buildHTTPCloneURL(publicDomain, repository.RepositoryType, repository.Path),
		SSHCloneURL:  buildSSHCloneURL(sshDomain, repository.RepositoryType, repository.Path),
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
		err := errors.New("name contains consecutive special characters which is not allowed")
		return false, errorx.BadRequest(err,
			errorx.Ctx().
				Set("name", name).
				Set("detail", err.Error()),
		)
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
			return errorx.BadRequest(errors.New(rule.message),
				errorx.Ctx().
					Set("name", name).
					Set("detail", rule.message),
			)
		}
	}

	return nil
}

func GetSourceTypeAndPathFromURL(url string) (string, string, error) {
	if url == "" {
		return "", "", errors.New("url is empty")
	}
	var sourceType, path string
	url = strings.TrimSuffix(url, ".git")
	strs := strings.Split(url, "/")
	if len(strs) < 2 {
		return "", "", errors.New("invalid url")
	}
	path = strings.Join(strs[len(strs)-2:], "/")
	if strings.Contains(url, "huggingface.co/") {
		sourceType = enum.HFSource
	} else if strings.Contains(url, "www.modelscope.cn/") {
		sourceType = enum.MSSource
	} else if strings.Contains(url, "opencsg.com/") {
		sourceType = enum.CSGSource
	} else if strings.Contains(url, "github.com/") {
		sourceType = enum.GitHubSource
	} else {
		return "", "", fmt.Errorf("unsupported source type: %s", url)
	}
	return sourceType, path, nil
}

func BuildRelativePath(repoType, namespace, name string) string {
	return strings.ToLower(repoType + "_" + namespace + "/" + name)
}

func BuildLfsPath(repoID int64, oid string, migrated bool) string {
	var lfsPath string
	if migrated {
		sha256Path := SHA256(strconv.FormatInt(repoID, 10))
		lfsPath = fmt.Sprintf("repos/%s/%s/%s/%s", sha256Path[0:2], sha256Path[2:4], sha256Path, oid)
	} else {
		lfsPath = path.Join("lfs", path.Join(oid[0:2], oid[2:4], oid[4:]))
	}
	return lfsPath
}

func buildHashedRelativePath(repoID int64) string {
	sha256Path := SHA256(strconv.FormatInt(repoID, 10))
	return fmt.Sprintf("%s/%s/%s/%s", HashedRepoPathPrefix, sha256Path[0:2], sha256Path[2:4], sha256Path)
}

func BuildHashedRelativePath(repoID int64) string {
	return buildHashedRelativePath(repoID) + ".git"
}

func SafeBuildLfsPath(repoID int64, oid, lfsRelativePath string, migrated bool) string {
	if oid != "" {
		return BuildLfsPath(repoID, oid, migrated)
	}
	return path.Join("lfs", lfsRelativePath)
}

func MirrorTaskStatusToRepoStatus(mirrorTaskSatus types.MirrorTaskStatus) types.RepositorySyncStatus {
	return MirrorTaskStatusToRepoStatusMap[mirrorTaskSatus]
}
