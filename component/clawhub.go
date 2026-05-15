package component

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

const (
	globalNamespace = "global"
	separator       = "--"
)

func parseCanonicalSlug(canonicalSlug string) (namespace, slug string) {
	sepIndex := strings.Index(canonicalSlug, separator)
	if sepIndex > 0 {
		return canonicalSlug[:sepIndex], canonicalSlug[sepIndex+len(separator):]
	}
	return globalNamespace, canonicalSlug
}

func parseNormalizedCanonicalSlug(canonicalSlug string) (namespace, slug string) {
	namespace, rawSlug := parseCanonicalSlug(canonicalSlug)
	slug, _ = NormalizeClawHubSkillIdentity(rawSlug, "")
	return namespace, slug
}

func toCanonicalSlug(namespace, slug string) string {
	if namespace == globalNamespace {
		return slug
	}
	return namespace + separator + slug
}

type ClawHubComponent interface {
	Search(ctx context.Context, query string, limit int, username string) (*types.ClawHubSearchResponse, error)
	GetSkill(ctx context.Context, canonicalSlug string, username string) (*types.ClawHubSkillResponse, error)
	GetSkillVersion(ctx context.Context, canonicalSlug string, version string, username string) (*types.ClawHubSkillVersionResponse, error)
	PublishSkill(ctx context.Context, req *types.ClawHubPublishRequest, files map[string][]byte, username string) (*types.ClawHubPublishSkillResponse, error)
	ResolveSkill(ctx context.Context, canonicalSlug string, username string) (*types.ClawHubResolveResponse, error)
	DownloadSkill(ctx context.Context, canonicalSlug string, version string, username string) ([]byte, string, error)
	Whoami(ctx context.Context, username string) (*types.ClawHubUserResponse, error)
}

func NewClawHubComponent(config *config.Config) (ClawHubComponent, error) {
	skillComponent, err := NewSkillComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create skill component: %w", err)
	}
	gitServer, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server: %w", err)
	}
	repoComponent, err := NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component: %w", err)
	}

	return &clawHubComponent{
		skill:             skillComponent,
		gitServer:         gitServer,
		repoComponent:     repoComponent,
		skillStore:        database.NewSkillStore(),
		skillVersionStore: database.NewSkillVersionStore(),
		userStore:         database.NewUserStore(),
		config:            config,
	}, nil
}

type clawHubComponent struct {
	skill             SkillComponent
	gitServer         gitserver.GitServer
	repoComponent     RepoComponent
	skillStore        database.SkillStore
	skillVersionStore database.SkillVersionStore
	userStore         database.UserStore
	config            *config.Config
}

func (c *clawHubComponent) Search(ctx context.Context, query string, limit int, username string) (*types.ClawHubSearchResponse, error) {
	filter := &types.RepoFilter{
		Search:   query,
		Username: username,
	}

	skills, _, err := c.skill.Index(ctx, filter, limit, 1, false, true)
	if err != nil {
		return nil, err
	}

	skillIDs := make([]int64, 0, len(skills))
	for _, skill := range skills {
		skillIDs = append(skillIDs, skill.ID)
	}
	latestVersions, err := c.skillVersionStore.LatestBySkillIDs(ctx, skillIDs)
	if err != nil {
		return nil, err
	}

	results := make([]types.ClawHubSearchResult, 0, len(skills))
	for _, skill := range skills {
		namespace := globalNamespace
		if skill.Path != "" {
			parts := strings.Split(skill.Path, "/")
			if len(parts) > 0 && parts[0] != "" {
				namespace = parts[0]
			}
		}

		latestVersion := latestVersions[skill.ID]
		if latestVersion == nil || latestVersion.Version == "" {
			continue
		}

		slug, displayName := NormalizeClawHubSkillIdentity(skill.Name, skill.Nickname)
		results = append(results, types.ClawHubSearchResult{
			Slug:        toCanonicalSlug(namespace, slug),
			DisplayName: displayName,
			Summary:     skill.Description,
			Version:     clawHubResponseVersion(latestVersion.Version),
			Score:       1.0,
			UpdatedAt:   skill.UpdatedAt.Unix(),
		})
	}

	return &types.ClawHubSearchResponse{Results: results}, nil
}

func (c *clawHubComponent) GetSkill(ctx context.Context, canonicalSlug string, username string) (*types.ClawHubSkillResponse, error) {
	namespace, rawSlug := parseCanonicalSlug(canonicalSlug)
	slug, displayName := NormalizeClawHubSkillIdentity(rawSlug, "")

	skill, err := c.skill.Show(ctx, namespace, slug, username, false, false)
	if err != nil {
		return nil, errorx.SkillNotFound(err, errorx.Ctx().Set("slug", canonicalSlug))
	}
	if skill.Nickname != "" {
		_, displayName = NormalizeClawHubSkillIdentity(skillNameForDisplay(skill, rawSlug), skill.Nickname)
	}

	return buildClawHubSkillResponse(skill, namespace, slug, displayName, clawHubVersionInfosFromSkill(skill.Versions)), nil
}

func (c *clawHubComponent) GetSkillVersion(ctx context.Context, canonicalSlug string, version string, username string) (*types.ClawHubSkillVersionResponse, error) {
	namespace, rawSlug := parseCanonicalSlug(canonicalSlug)
	slug, displayName := NormalizeClawHubSkillIdentity(rawSlug, "")

	skill, err := c.skill.Show(ctx, namespace, slug, username, false, false)
	if err != nil {
		return nil, errorx.SkillNotFound(err, errorx.Ctx().Set("slug", canonicalSlug))
	}
	if skill.Nickname != "" {
		_, displayName = NormalizeClawHubSkillIdentity(skillNameForDisplay(skill, rawSlug), skill.Nickname)
	}

	skillVersion, err := c.findSkillVersion(ctx, skill.ID, version)
	if err != nil {
		return nil, err
	}

	return &types.ClawHubSkillVersionResponse{
		Version: clawHubSkillVersionInfoFromDB(skillVersion),
		Skill: &types.ClawHubVersionSkillInfo{
			Slug:        toCanonicalSlug(namespace, slug),
			DisplayName: displayName,
		},
	}, nil
}

func skillNameForDisplay(skill *types.Skill, fallback string) string {
	if skill.Name != "" {
		return skill.Name
	}
	return fallback
}

func buildClawHubSkillResponse(
	skill *types.Skill,
	namespace string,
	slug string,
	displayName string,
	versions []*types.ClawHubVersionInfo,
) *types.ClawHubSkillResponse {
	skillInfo := &types.ClawHubSkillInfo{
		Slug:        toCanonicalSlug(namespace, slug),
		DisplayName: displayName,
		Summary:     skill.Description,
		Tags:        skill.Tags,
		Stats: map[string]interface{}{
			"likes":     skill.Likes,
			"downloads": skill.Downloads,
		},
		CreatedAt: skill.CreatedAt.UnixMilli(),
		UpdatedAt: skill.UpdatedAt.UnixMilli(),
	}

	var latestVersion *types.ClawHubVersionInfo
	if len(versions) > 0 {
		latestVersion = versions[0]
	} else {
		latestVersion = &types.ClawHubVersionInfo{
			Version:   "latest",
			CreatedAt: skill.CreatedAt.UnixMilli(),
		}
		versions = append(versions, latestVersion)
	}

	return &types.ClawHubSkillResponse{
		Skill:         skillInfo,
		LatestVersion: latestVersion,
		Versions:      versions,
		Owner: &types.ClawHubOwnerInfo{
			Handle:      skill.User.Username,
			DisplayName: skill.User.Nickname,
			Image:       skill.User.Avatar,
		},
		Moderation: &types.ClawHubModerationInfo{
			IsSuspicious:     false,
			IsMalwareBlocked: false,
		},
	}
}

func clawHubVersionInfosFromSkill(versions []types.SkillVersion) []*types.ClawHubVersionInfo {
	versionInfos := make([]*types.ClawHubVersionInfo, 0, len(versions))
	for _, version := range versions {
		versionInfos = append(versionInfos, clawHubVersionInfoFromSkill(version))
	}
	return versionInfos
}

func clawHubVersionInfoFromSkill(version types.SkillVersion) *types.ClawHubVersionInfo {
	versionInfo := &types.ClawHubVersionInfo{
		Version:   clawHubResponseVersion(version.Version),
		Commit:    version.Commit,
		CreatedAt: version.CreatedAt.UnixMilli(),
		Changelog: version.Changelog,
	}
	if version.License != "" {
		versionInfo.License = &version.License
	}
	return versionInfo
}

func clawHubSkillVersionInfoFromDB(version *database.SkillVersion) *types.ClawHubSkillVersionInfo {
	versionInfo := &types.ClawHubSkillVersionInfo{
		Version:   clawHubResponseVersion(version.Version),
		CreatedAt: version.CreatedAt.UnixMilli(),
		Changelog: version.Changelog,
	}
	if version.License != "" {
		versionInfo.License = &version.License
	}
	return versionInfo
}

func (c *clawHubComponent) findSkillVersion(ctx context.Context, skillID int64, version string) (*database.SkillVersion, error) {
	var lastErr error
	for _, candidate := range skillVersionCandidates(version) {
		skillVersion, err := c.skillVersionStore.BySkillIDAndVersion(ctx, skillID, candidate)
		if err == nil {
			return skillVersion, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, errorx.SkillVersionNotFound(err, errorx.Ctx().Set("skill_id", fmt.Sprintf("%d", skillID)).Set("version", version))
		}
		lastErr = err
	}
	return nil, errorx.SkillVersionNotFound(lastErr, errorx.Ctx().Set("skill_id", fmt.Sprintf("%d", skillID)).Set("version", version))
}

func skillVersionCandidates(version string) []string {
	version = strings.TrimSpace(version)
	if version == "" {
		return []string{version}
	}
	if strings.HasPrefix(version, "v") {
		withoutPrefix := strings.TrimPrefix(version, "v")
		if withoutPrefix == "" {
			return []string{version}
		}
		return []string{version, withoutPrefix}
	}
	return []string{version, "v" + version}
}

func clawHubResponseVersion(version string) string {
	trimmed := strings.TrimPrefix(version, "v")
	if trimmed != version && isVersionPattern(trimmed) {
		return trimmed
	}
	return version
}

func NormalizeClawHubSkillIdentity(slug, displayName string) (string, string) {
	normalizedSlug, _ := parseSkillNameAndVersion(slug)
	if normalizedSlug == "" {
		normalizedSlug = slug
	}
	return normalizedSlug, resolveClawHubDisplayName(slug, displayName, normalizedSlug)
}

func parseSkillNameAndVersion(folderName string) (name string, version string) {
	lastDash := strings.LastIndex(folderName, "-")
	if lastDash <= 0 || lastDash == len(folderName)-1 {
		return folderName, ""
	}

	suffix := folderName[lastDash+1:]
	if isVersionPattern(suffix) {
		return folderName[:lastDash], "v" + suffix
	}

	parts := strings.Split(folderName, "-")
	if len(parts) >= 4 {
		versionParts := parts[len(parts)-3:]
		if isNumericVersionParts(versionParts) {
			return strings.Join(parts[:len(parts)-3], "-"), "v" + strings.Join(versionParts, ".")
		}
	}

	return folderName, ""
}

func isVersionPattern(version string) bool {
	parts := strings.Split(version, ".")
	return isNumericVersionParts(parts)
}

func isNumericVersionParts(parts []string) bool {
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

func (c *clawHubComponent) validatePublishFiles(files map[string][]byte) error {
	fileCount := len(files)
	maxCount := c.config.Skill.MaxPublishFileCount
	if maxCount > 0 && fileCount > maxCount {
		return errorx.SkillPublishFileCountExceeded(maxCount, fileCount)
	}

	var totalSize int64
	for _, content := range files {
		totalSize += int64(len(content))
	}
	maxSize := c.config.Skill.MaxPublishFileSize
	if maxSize > 0 && totalSize > maxSize {
		return errorx.SkillPublishFileSizeExceeded(maxSize, totalSize)
	}

	return nil
}

func (c *clawHubComponent) PublishSkill(ctx context.Context, req *types.ClawHubPublishRequest, files map[string][]byte, username string) (*types.ClawHubPublishSkillResponse, error) {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return nil, errorx.SkillUserNotFound(err, errorx.Ctx().Set("username", username))
	}

	if err := c.validatePublishFiles(files); err != nil {
		return nil, err
	}

	namespace := user.Username
	slug, displayName := NormalizeClawHubSkillIdentity(req.Slug, req.DisplayName)

	version := req.Version
	if version == "" {
		version = "latest"
	}

	skillDB, err := c.skillStore.FindByPath(ctx, namespace, slug)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to find skill: %w", err)
		}
		return c.createSkill(ctx, files, username, namespace, slug, displayName, version, req.Changelog)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, username, skillDB.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission: %w", err)
	}
	if !permission.CanWrite {
		return nil, errorx.ErrForbidden
	}

	if err := c.commitSkillFiles(ctx, files, username, namespace, slug, fmt.Sprintf("Publish version %s", version)); err != nil {
		return nil, err
	}

	commitHash := c.latestCommitHash(ctx, namespace, slug)
	newVersion, err := c.createOrUpdateVersion(ctx, skillDB.ID, version, commitHash, req.Changelog)
	if err != nil {
		return nil, errorx.SkillPublishFailed(err, errorx.Ctx().Set("slug", slug).Set("version", version))
	}

	return &types.ClawHubPublishSkillResponse{
		Ok:        true,
		SkillId:   fmt.Sprintf("%d", skillDB.ID),
		VersionId: fmt.Sprintf("%d", newVersion.ID),
	}, nil
}

func resolveClawHubDisplayName(originalSlug, displayName, normalizedSlug string) string {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || displayName == originalSlug {
		return normalizedSlug
	}

	_, version := parseSkillNameAndVersion(originalSlug)
	if version != "" {
		return stripDisplayNameVersion(displayName, version)
	}

	return displayName
}

func stripDisplayNameVersion(displayName, version string) string {
	plainVersion := strings.TrimPrefix(version, "v")
	for _, versionText := range []string{plainVersion, strings.ReplaceAll(plainVersion, ".", "-")} {
		for _, separator := range []string{" ", "-"} {
			for _, prefix := range []string{"", "v"} {
				suffix := separator + prefix + versionText
				if strings.HasSuffix(displayName, suffix) {
					stripped := strings.TrimSpace(strings.TrimSuffix(displayName, suffix))
					if stripped != "" {
						return stripped
					}
				}
			}
		}
	}
	return displayName
}

func (c *clawHubComponent) createSkill(ctx context.Context, files map[string][]byte, username, namespace, slug, displayName, version, changelog string) (*types.ClawHubPublishSkillResponse, error) {
	createReq := &types.CreateSkillReq{
		CreateRepoReq: types.CreateRepoReq{
			Namespace: namespace,
			Name:      slug,
			Nickname:  displayName,
			Private:   false,
			RepoType:  types.SkillRepo,
			Username:  username,
		},
	}

	for filePath, content := range files {
		createReq.CommitFiles = append(createReq.CommitFiles, types.CommitFile{
			Path:    filePath,
			Content: string(content),
		})
	}

	skill, err := c.skill.Create(ctx, createReq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create skill", slog.String("slug", slug), slog.Any("error", err))
		return nil, errorx.SkillPublishFailed(err, errorx.Ctx().Set("slug", slug).Set("version", version))
	}

	skillDB, err := c.skillStore.FindByPath(ctx, namespace, slug)
	if err != nil {
		return nil, errorx.SkillPublishFailed(err, errorx.Ctx().Set("slug", slug).Set("version", version))
	}

	commitHash := c.latestCommitHash(ctx, namespace, slug)
	newVersion, err := c.createOrUpdateVersion(ctx, skillDB.ID, version, commitHash, changelog)
	if err != nil {
		return nil, errorx.SkillPublishFailed(err, errorx.Ctx().Set("slug", slug).Set("version", version))
	}

	return &types.ClawHubPublishSkillResponse{
		Ok:        true,
		SkillId:   fmt.Sprintf("%d", skill.ID),
		VersionId: fmt.Sprintf("%d", newVersion.ID),
	}, nil
}

func (c *clawHubComponent) commitSkillFiles(ctx context.Context, files map[string][]byte, username, namespace, slug, message string) error {
	var commitErr error
	for attempt := 1; attempt <= 2; attempt++ {
		uploadedPaths := sortedUploadPaths(files)

		existingUploadedFiles, err := c.gitServer.GetFilesByRevisionAndPaths(ctx, gitserver.GetFilesByRevisionAndPathsReq{
			Namespace: namespace,
			Name:      slug,
			RepoType:  types.SkillRepo,
			Revision:  types.MainBranch,
			Paths:     uploadedPaths,
		})
		if err != nil {
			return fmt.Errorf("failed to get existing uploaded skill files: %w", err)
		}

		existingFiles, err := c.gitServer.GetRepoAllFiles(ctx, gitserver.GetRepoAllFilesReq{
			Namespace: namespace,
			Name:      slug,
			RepoType:  types.SkillRepo,
			Ref:       types.MainBranch,
		})
		if err != nil {
			return fmt.Errorf("failed to get existing skill files: %w", err)
		}

		commitFiles := buildSkillSyncCommitFiles(files, existingUploadedFiles, existingFiles)
		if len(commitFiles) == 0 {
			return nil
		}

		commitErr = c.gitServer.CommitFiles(ctx, gitserver.CommitFilesReq{
			Namespace: namespace,
			Name:      slug,
			RepoType:  types.SkillRepo,
			Revision:  types.MainBranch,
			Files:     commitFiles,
			Username:  username,
			Email:     fmt.Sprintf("%s@users.noreply.csghub.com", username),
			Message:   message,
		})
		if commitErr == nil {
			return nil
		}
		if attempt < 2 {
			slog.WarnContext(ctx, "failed to commit files to skill, retrying with latest file tree", slog.String("slug", slug), slog.Int("attempt", attempt), slog.Any("error", commitErr))
		}
	}
	return fmt.Errorf("failed to commit files to skill: %w", commitErr)
}

func buildSkillSyncCommitFiles(files map[string][]byte, existingUploadedFiles []*types.File, existingFiles []*types.File) []gitserver.CommitFile {
	existingUploaded := filePathSet(existingUploadedFiles)
	existing := filePathSet(existingFiles)
	uploadedPaths := sortedUploadPaths(files)

	commitFiles := make([]gitserver.CommitFile, 0, len(uploadedPaths)+len(existing))
	for _, filePath := range uploadedPaths {
		action := gitserver.CommitActionCreate
		if _, ok := existingUploaded[filePath]; ok {
			action = gitserver.CommitActionUpdate
		}
		commitFiles = append(commitFiles, gitserver.CommitFile{
			Path:    filePath,
			Content: base64.StdEncoding.EncodeToString(files[filePath]),
			Action:  action,
		})
	}

	uploaded := make(map[string]struct{}, len(uploadedPaths))
	for _, filePath := range uploadedPaths {
		uploaded[filePath] = struct{}{}
	}
	existingPaths := make([]string, 0, len(existing))
	for filePath := range existing {
		if _, ok := uploaded[filePath]; ok {
			continue
		}
		existingPaths = append(existingPaths, filePath)
	}
	sort.Strings(existingPaths)

	for _, filePath := range existingPaths {
		commitFiles = append(commitFiles, gitserver.CommitFile{
			Path:   filePath,
			Action: gitserver.CommitActionDelete,
		})
	}

	return commitFiles
}

func sortedUploadPaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for filePath := range files {
		if filePath == "" {
			continue
		}
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	return paths
}

func filePathSet(files []*types.File) map[string]struct{} {
	paths := make(map[string]struct{}, len(files))
	for _, file := range files {
		if file != nil && file.Path != "" {
			paths[file.Path] = struct{}{}
		}
	}
	return paths
}

func (c *clawHubComponent) checkSkillReadPermission(ctx context.Context, username string, repo *database.Repository) error {
	permission, err := c.repoComponent.GetUserRepoPermission(ctx, username, repo)
	if err != nil {
		return fmt.Errorf("failed to get user repo permission: %w", err)
	}
	if !permission.CanRead {
		return errorx.ErrForbidden
	}
	return nil
}

func (c *clawHubComponent) latestCommitHash(ctx context.Context, namespace, slug string) string {
	lastCommit, err := c.gitServer.GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: namespace,
		Name:      slug,
		RepoType:  types.SkillRepo,
		Ref:       types.MainBranch,
	})
	if err != nil || lastCommit == nil {
		return ""
	}
	return lastCommit.ID
}

func (c *clawHubComponent) ResolveSkill(ctx context.Context, canonicalSlug string, username string) (*types.ClawHubResolveResponse, error) {
	namespace, slug := parseNormalizedCanonicalSlug(canonicalSlug)

	skillDB, err := c.skillStore.FindByPath(ctx, namespace, slug)
	if err != nil {
		return nil, errorx.SkillNotFound(err, errorx.Ctx().Set("slug", canonicalSlug))
	}

	if err := c.checkSkillReadPermission(ctx, username, skillDB.Repository); err != nil {
		return nil, err
	}

	latestVersion := &types.ClawHubResolveVersionInfo{Version: "latest"}
	latest, err := c.skillVersionStore.LatestBySkillID(ctx, skillDB.ID)
	if err == nil && latest != nil {
		latestVersion.Version = clawHubResponseVersion(latest.Version)
	}

	return &types.ClawHubResolveResponse{
		Match:         latestVersion,
		LatestVersion: latestVersion,
	}, nil
}

func (c *clawHubComponent) DownloadSkill(ctx context.Context, canonicalSlug string, version string, username string) ([]byte, string, error) {
	namespace, slug := parseNormalizedCanonicalSlug(canonicalSlug)

	skillDB, err := c.skillStore.FindByPath(ctx, namespace, slug)
	if err != nil {
		return nil, "", errorx.SkillNotFound(err, errorx.Ctx().Set("slug", canonicalSlug))
	}

	if err := c.checkSkillReadPermission(ctx, username, skillDB.Repository); err != nil {
		return nil, "", err
	}

	actualVersion := version
	archiveRevision := types.MainBranch
	if actualVersion == "" || actualVersion == "latest" {
		actualVersion = "latest"
		latest, err := c.skillVersionStore.LatestBySkillID(ctx, skillDB.ID)
		if err == nil && latest != nil {
			actualVersion = clawHubResponseVersion(latest.Version)
			if latest.Hash != "" {
				archiveRevision = latest.Hash
			}
		}
	} else {
		versionInfo, err := c.findSkillVersion(ctx, skillDB.ID, actualVersion)
		if err != nil {
			return nil, "", err
		}
		actualVersion = clawHubResponseVersion(versionInfo.Version)
		if versionInfo.Hash != "" {
			archiveRevision = versionInfo.Hash
		}
	}

	archiveData, err := c.gitServer.GetArchive(ctx, gitserver.GetArchiveReq{
		Namespace: namespace,
		Name:      slug,
		Revision:  archiveRevision,
		RepoType:  types.SkillRepo,
	})
	if err != nil {
		return nil, "", errorx.SkillDownloadFailed(err, errorx.Ctx().Set("slug", canonicalSlug).Set("version", actualVersion))
	}

	return archiveData, actualVersion, nil
}

func (c *clawHubComponent) Whoami(ctx context.Context, username string) (*types.ClawHubUserResponse, error) {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return nil, errorx.SkillUserNotFound(err, errorx.Ctx().Set("username", username))
	}

	return &types.ClawHubUserResponse{
		User: types.ClawHubUserInfo{
			Handle:      user.Username,
			DisplayName: user.NickName,
			Image:       user.Avatar,
		},
	}, nil
}
