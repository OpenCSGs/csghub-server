package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"strings"

	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/enum"
	"opencsg.com/csghub-server/common/utils/common"
)

// mirrorMetadataClientFactory creates an OpenCSG API client for one mirror source.
type mirrorMetadataClientFactory func(endpoint, accessToken string) multisync.Client

// openCSGSaaSMetadataEndpoint is the trusted metadata API endpoint for OpenCSG SaaS Git repositories.
const openCSGSaaSMetadataEndpoint = "https://hub.opencsg.com"

// mirrorRepoMetadata contains type-specific metadata fetched before transactional mirror creation.
type mirrorRepoMetadata struct {
	mcpServer *types.MCPServer
	skill     *types.Skill
}

// requeueMirrorRepoTask atomically schedules a new sync for an existing mirror target.
func (m *mirrorComponentImpl) requeueMirrorRepoTask(ctx context.Context, repo *database.Repository, mirror *database.Mirror, username, accessToken *string, priority types.MirrorPriority, urgent bool) (database.MirrorTask, error) {
	metadata, err := m.prepareExistingMirrorRepoMetadataUpdate(ctx, repo, mirror, username, accessToken)
	if err != nil {
		return database.MirrorTask{}, err
	}
	return m.requeueMirrorRepoTaskWithMetadata(ctx, repo, mirror, username, accessToken, priority, urgent, metadata)
}

// prepareExistingMirrorRepoMetadataUpdate converts an existing mirror into one metadata update payload.
func (m *mirrorComponentImpl) prepareExistingMirrorRepoMetadataUpdate(ctx context.Context, repo *database.Repository, mirror *database.Mirror, username, accessToken *string) (*database.MirrorRepoMetadataUpdate, error) {
	if repo.RepositoryType != types.MCPServerRepo && repo.RepositoryType != types.SkillRepo {
		return nil, nil
	}
	sourcePath := strings.Trim(strings.TrimSuffix(mirror.SourceRepoPath, ".git"), "/")
	if sourcePath == "" {
		_, sourcePath, _ = common.GetSourceTypeAndPathFromURL(mirror.SourceUrl)
		sourcePath = strings.Trim(strings.TrimSuffix(sourcePath, ".git"), "/")
	}
	sourceNamespace, sourceName := path.Split(sourcePath)
	sourceNamespace = strings.Trim(sourceNamespace, "/")
	if sourceNamespace == "" || sourceName == "" {
		return nil, fmt.Errorf("failed to resolve source repository path for %s metadata refresh", repo.RepositoryType)
	}
	sourceUsername, sourceAccessToken := mirror.Username, mirror.AccessToken
	if username != nil && accessToken != nil {
		sourceUsername, sourceAccessToken = *username, *accessToken
	}
	req := types.CreateMirrorRepoReq{
		SourceNamespace:   sourceNamespace,
		SourceName:        sourceName,
		MirrorSourceID:    mirror.MirrorSourceID,
		RepoType:          repo.RepositoryType,
		SourceGitCloneUrl: mirror.SourceUrl,
		Username:          sourceUsername,
		AccessToken:       sourceAccessToken,
	}
	metadata, err := m.prepareExistingMirrorRepoMetadata(ctx, repo, req)
	if err != nil {
		return nil, err
	}
	return buildMirrorRepoMetadataUpdate(req, repo, metadata)
}

// prepareExistingMirrorRepoMetadata fetches and applies source API metadata to an existing repository.
func (m *mirrorComponentImpl) prepareExistingMirrorRepoMetadata(ctx context.Context, repo *database.Repository, req types.CreateMirrorRepoReq) (*mirrorRepoMetadata, error) {
	if repo.RepositoryType != types.MCPServerRepo && repo.RepositoryType != types.SkillRepo {
		return nil, nil
	}
	metadata, err := m.fetchMirrorRepoMetadata(ctx, req)
	if err != nil {
		return nil, err
	}
	applyMirrorRepoMetadataToExistingRepository(repo, metadata)
	return metadata, nil
}

// requeueMirrorRepoTaskWithMetadata schedules a new sync task and optionally applies source metadata atomically.
func (m *mirrorComponentImpl) requeueMirrorRepoTaskWithMetadata(ctx context.Context, repo *database.Repository, mirror *database.Mirror, username, accessToken *string, priority types.MirrorPriority, urgent bool, metadata *database.MirrorRepoMetadataUpdate) (database.MirrorTask, error) {
	task, err := m.mirrorTaskJobStore.RequeueMirrorRepoTask(ctx, database.RequeueMirrorRepoTaskInput{
		MirrorID:        mirror.ID,
		RepositoryID:    repo.ID,
		Username:        username,
		AccessToken:     accessToken,
		Priority:        priority,
		Urgent:          urgent,
		JobClient:       m.mirrorRepoJobClient,
		JobCancelClient: m.mirrorJobClient,
		Metadata:        metadata,
	})
	if err != nil {
		return database.MirrorTask{}, fmt.Errorf("failed to create mirror task: %w", err)
	}
	return task, nil
}

// requeueMirrorFromSaas atomically replaces existing SaaS mirror work without fetching source API metadata.
func (m *mirrorComponentImpl) requeueMirrorFromSaas(ctx context.Context, repo *database.Repository, mirror *database.Mirror) (database.MirrorTask, error) {
	return m.requeueMirrorRepoTaskWithMetadata(ctx, repo, mirror, nil, nil, types.LowMirrorPriority, false, nil)
}

// CreateMirrorRepo creates or binds one mirror source to one target repository.
func (m *mirrorComponentImpl) CreateMirrorRepo(ctx context.Context, req types.CreateMirrorRepoReq) (*database.Mirror, error) {
	if req.CurrentUser == "" {
		err := fmt.Errorf("current user is required")
		return nil, errorx.BadRequest(err, errorx.Ctx().Set("current user", req.CurrentUser))
	}
	priority, err := normalizeMirrorPriority(req.Priority)
	if err != nil {
		return nil, err
	}
	req.Priority = priority
	sourceURL, username, accessToken, err := normalizeMirrorSource(
		req.SourceGitCloneUrl, req.Username, req.AccessToken,
	)
	if err != nil {
		return nil, err
	}
	req.SourceGitCloneUrl = sourceURL
	req.Username = username
	req.AccessToken = accessToken

	namespace, name := m.resolveMirrorRepoTarget(req)
	if namespace == "" || name == "" {
		err := fmt.Errorf("fork namespace and fork name are required")
		return nil, errorx.BadRequest(err,
			errorx.Ctx().
				Set("fork namespace", namespace).
				Set("fork name", name),
		)
	}

	canWrite, err := m.repoComp.CheckCurrentUserPermission(ctx, req.CurrentUser, namespace, membership.RoleWrite)
	if err != nil {
		return nil, fmt.Errorf("failed to check mirror repo permission: %w", err)
	}
	if !canWrite {
		return nil, errorx.ErrForbiddenMsg("users do not have permission to create mirror in this namespace")
	}

	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, namespace, name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check target repo existence, error: %w", err)
	}

	// repo exists
	if repo != nil && repo.ID != 0 {
		if req.CreateTargetRepo != nil && *req.CreateTargetRepo {
			return nil, errorx.ErrRepoAlreadyExist
		}

		mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to find mirror by target repo, error: %w", err)
		}

		// mirror exists
		if mirror != nil && mirror.ID != 0 {
			if mirror.SourceUrl == req.SourceGitCloneUrl {
				metadata, err := m.prepareExistingMirrorRepoMetadata(ctx, repo, req)
				if err != nil {
					return nil, err
				}
				metadataUpdate, err := buildMirrorRepoMetadataUpdate(req, repo, metadata)
				if err != nil {
					return nil, err
				}

				var usernamePtr, accessTokenPtr *string
				if req.Username != "" {
					usernamePtr = &req.Username
					accessTokenPtr = &req.AccessToken
				}
				if _, err := m.requeueMirrorRepoTaskWithMetadata(ctx, repo, mirror, usernamePtr, accessTokenPtr, req.Priority, req.Urgent, metadataUpdate); err != nil {
					return nil, fmt.Errorf("failed to sync mirror repo, error: %w", err)
				}
				if req.Username != "" {
					mirror.Username = req.Username
					mirror.AccessToken = req.AccessToken
				}
				return mirror, nil
			}
			return &database.Mirror{RepositoryID: repo.ID}, errorx.MirrorSourceConflict(
				fmt.Errorf("target repo already has mirror source url: %s", mirror.SourceUrl),
				errorx.Ctx().
					Set("repo type", req.RepoType).
					Set("target namespace", namespace).
					Set("target name", name).
					Set("source url", req.SourceGitCloneUrl),
			)
		}

		metadata, err := m.prepareExistingMirrorRepoMetadata(ctx, repo, req)
		if err != nil {
			return nil, err
		}
		repoNamespace, _ := repo.NamespaceAndName()
		return m.createMirrorRepoRecords(ctx, req, repo, repoNamespace, repo.Name, false, metadata)
	}
	if req.CreateTargetRepo != nil && !*req.CreateTargetRepo {
		return nil, errorx.RepoNotFound(
			errors.New("target repository does not exist"),
			errorx.Ctx().
				Set("repo type", req.RepoType).
				Set("target namespace", namespace).
				Set("target name", name),
		)
	}

	private := true
	if req.Private != nil {
		private = *req.Private
	}
	metadata, err := m.fetchMirrorRepoMetadata(ctx, req)
	if err != nil {
		return nil, err
	}

	createRepoReq := types.CreateRepoReq{
		Username:      req.CurrentUser,
		Namespace:     namespace,
		Name:          name,
		Nickname:      name,
		Description:   req.Description,
		Private:       private,
		License:       req.License,
		DefaultBranch: req.DefaultBranch,
		RepoType:      req.RepoType,
		ToolCount:     len(req.MCPServerAttributes.Tools),
		StarCount:     req.MCPServerAttributes.StarCount,
	}
	applyMirrorRepoMetadata(&createRepoReq, metadata)

	sourceType, sourcePath := "", ""
	if !req.SkipSourcePath {
		sourceType, sourcePath, _ = common.GetSourceTypeAndPathFromURL(req.SourceGitCloneUrl)
	}
	dbRepo, err := m.prepareMirrorRepository(ctx, createRepoReq, sourceType, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare mirror repository, error: %w", err)
	}

	repoNamespace, _ := dbRepo.NamespaceAndName()
	return m.createMirrorRepoRecords(ctx, req, dbRepo, repoNamespace, dbRepo.Name, true, metadata)
}

// fetchMirrorRepoMetadata imports MCP or skill metadata from the source API before repository creation.
func (m *mirrorComponentImpl) fetchMirrorRepoMetadata(ctx context.Context, req types.CreateMirrorRepoReq) (*mirrorRepoMetadata, error) {
	if req.RepoType != types.MCPServerRepo && req.RepoType != types.SkillRepo {
		return nil, nil
	}
	// Explicit MCP attributes remain authoritative for compatibility with existing callers.
	if req.RepoType == types.MCPServerRepo && hasMCPServerAttributes(req.MCPServerAttributes) {
		return nil, nil
	}

	endpoint, err := m.resolveMirrorMetadataEndpoint(ctx, req)
	if err != nil {
		return nil, err
	}

	clientFactory := m.mirrorMetadataClientFactory
	if clientFactory == nil {
		clientFactory = multisync.FromOpenCSG
	}
	client := clientFactory(endpoint, req.AccessToken)
	version := types.SyncVersion{
		RepoPath: path.Join(path.Base(strings.TrimSpace(req.SourceNamespace)), req.SourceName),
		RepoType: req.RepoType,
	}
	metadata := &mirrorRepoMetadata{}
	switch req.RepoType {
	case types.MCPServerRepo:
		metadata.mcpServer, err = client.MCPServerInfo(ctx, version)
		if err == nil && metadata.mcpServer == nil {
			err = errors.New("source returned empty mcp server metadata")
		}
	case types.SkillRepo:
		metadata.skill, err = client.SkillInfo(ctx, version)
		if err == nil && metadata.skill == nil {
			err = errors.New("source returned empty skill metadata")
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s mirror metadata: %w", req.RepoType, err)
	}
	return metadata, nil
}

// resolveMirrorMetadataEndpoint returns a trusted OpenCSG-compatible API endpoint for MCP and skill metadata.
func (m *mirrorComponentImpl) resolveMirrorMetadataEndpoint(ctx context.Context, req types.CreateMirrorRepoReq) (string, error) {
	parsedURL, err := url.Parse(req.SourceGitCloneUrl)
	if err != nil {
		return "", errorx.BadRequest(
			fmt.Errorf("failed to parse source git clone url for metadata: %w", err),
			errorx.Ctx().Set("source url", req.SourceGitCloneUrl),
		)
	}
	isOpenCSGSaaS := isOpenCSGSaaSHost(parsedURL.Hostname())

	if req.MirrorSourceID != 0 {
		source, err := m.mirrorSourceStore.Get(ctx, req.MirrorSourceID)
		if errors.Is(err, sql.ErrNoRows) {
			return "", errorx.BadRequest(
				fmt.Errorf("mirror source %d does not exist", req.MirrorSourceID),
				errorx.Ctx().Set("mirror source id", req.MirrorSourceID),
			)
		}
		if err != nil {
			return "", fmt.Errorf("failed to get mirror source metadata endpoint: %w", err)
		}
		if source == nil {
			return "", errorx.BadRequest(
				fmt.Errorf("mirror source %d does not exist", req.MirrorSourceID),
				errorx.Ctx().Set("mirror source id", req.MirrorSourceID),
			)
		}
		if strings.TrimSpace(source.InfoAPIUrl) == "" {
			if isOpenCSGSaaS {
				return openCSGSaaSMetadataEndpoint, nil
			}
			return "", errorx.MirrorSourceURLInvalid(
				fmt.Errorf("source host %q is neither configured by mirror source %d nor OpenCSG SaaS", parsedURL.Hostname(), req.MirrorSourceID),
				errorx.Ctx().
					Set("mirror source id", req.MirrorSourceID).
					Set("repo type", req.RepoType).
					Set("source host", parsedURL.Hostname()).
					Set("source url", req.SourceGitCloneUrl),
			)
		}
		endpoint, err := normalizeMirrorMetadataEndpoint(source.InfoAPIUrl)
		if err != nil {
			return "", errorx.BadRequest(
				fmt.Errorf("invalid info_api_url for mirror source %d: %w", req.MirrorSourceID, err),
				errorx.Ctx().Set("mirror source id", req.MirrorSourceID).Set("info api url", source.InfoAPIUrl),
			)
		}
		return endpoint, nil
	}

	if isOpenCSGSaaS {
		return openCSGSaaSMetadataEndpoint, nil
	}

	return "", errorx.MirrorSourceURLInvalid(
		fmt.Errorf("source host %q is neither a configured mirror source nor OpenCSG SaaS", parsedURL.Hostname()),
		errorx.Ctx().
			Set("repo type", req.RepoType).
			Set("source host", parsedURL.Hostname()).
			Set("source url", req.SourceGitCloneUrl),
	)
}

// isOpenCSGSaaSHost reports whether a Git host uses the trusted OpenCSG SaaS metadata API.
func isOpenCSGSaaSHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "opencsg.com", "hub.opencsg.com":
		return true
	default:
		return false
	}
}

// normalizeMirrorMetadataEndpoint validates a configured OpenCSG API base URL.
func normalizeMirrorMetadataEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid mirror metadata endpoint: %w", err)
	}
	if (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Hostname() == "" {
		return "", errors.New("mirror metadata endpoint must be an HTTP(S) URL with a host")
	}
	if parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return "", errors.New("mirror metadata endpoint must not contain query or fragment")
	}
	return endpoint, nil
}

// hasMCPServerAttributes reports whether a caller supplied legacy MCP metadata explicitly.
func hasMCPServerAttributes(attributes types.MCPServerAttributes) bool {
	return attributes.StarCount != 0 || len(attributes.Tools) != 0 || attributes.AvatarURL != "" ||
		attributes.Configuration.Type != "" || len(attributes.Configuration.Properties) != 0 ||
		len(attributes.Configuration.Required) != 0
}

// applyMirrorRepoMetadata fills API metadata fields while leaving README content to the mirrored Git repository.
func applyMirrorRepoMetadata(req *types.CreateRepoReq, metadata *mirrorRepoMetadata) {
	if metadata == nil {
		return
	}

	var nickname, description, license, defaultBranch string
	switch {
	case metadata.mcpServer != nil:
		nickname = metadata.mcpServer.Nickname
		description = metadata.mcpServer.Description
		license = metadata.mcpServer.License
		defaultBranch = metadata.mcpServer.DefaultBranch
		req.ToolCount = metadata.mcpServer.ToolsNum
		req.StarCount = metadata.mcpServer.StarNum
	case metadata.skill != nil:
		nickname = metadata.skill.Nickname
		description = metadata.skill.Description
		license = metadata.skill.License
		defaultBranch = metadata.skill.DefaultBranch
	}
	if nickname != "" {
		req.Nickname = nickname
	}
	if req.Description == "" {
		req.Description = description
	}
	if req.License == "" {
		req.License = license
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = defaultBranch
	}
}

// applyMirrorRepoMetadataToExistingRepository refreshes source-owned fields before task creation.
func applyMirrorRepoMetadataToExistingRepository(repo *database.Repository, metadata *mirrorRepoMetadata) {
	if metadata == nil {
		return
	}
	metadataReq := types.CreateRepoReq{
		Nickname:  repo.Nickname,
		StarCount: repo.StarCount,
	}
	applyMirrorRepoMetadata(&metadataReq, metadata)
	if metadataReq.Nickname != "" {
		repo.Nickname = metadataReq.Nickname
	}
	repo.Description = metadataReq.Description
	repo.License = metadataReq.License
	if metadataReq.DefaultBranch != "" {
		repo.DefaultBranch = metadataReq.DefaultBranch
	}
	repo.StarCount = metadataReq.StarCount
}

// buildMirrorRepoMetadataUpdate converts fetched metadata into one transactional update payload.
func buildMirrorRepoMetadataUpdate(req types.CreateMirrorRepoReq, repo *database.Repository, metadata *mirrorRepoMetadata) (*database.MirrorRepoMetadataUpdate, error) {
	if metadata == nil && !(req.RepoType == types.MCPServerRepo && hasMCPServerAttributes(req.MCPServerAttributes)) {
		return nil, nil
	}
	update := &database.MirrorRepoMetadataUpdate{Repository: *repo}
	if req.RepoType == types.MCPServerRepo {
		mcpServer, properties, err := buildMCPServerRows(req.RepoType, req.MCPServerAttributes, metadata)
		if err != nil {
			return nil, err
		}
		update.MCPServer = mcpServer
		update.MCPServerProperties = properties
	}
	if metadata != nil && metadata.skill != nil && !metadata.skill.UpdatedAt.IsZero() {
		updatedAt := metadata.skill.UpdatedAt
		update.SkillLastUpdatedAt = &updatedAt
	}
	return update, nil
}

// normalizeMirrorPriority defaults an omitted priority and rejects values unsupported by workhub.
func normalizeMirrorPriority(priority types.MirrorPriority) (types.MirrorPriority, error) {
	if priority == 0 {
		return types.LowMirrorPriority, nil
	}
	if priority < types.ASAPMirrorPriority || priority > types.LowMirrorPriority {
		err := fmt.Errorf("priority must be between %d and %d", types.ASAPMirrorPriority, types.LowMirrorPriority)
		return 0, errorx.BadRequest(err, errorx.Ctx().Set("priority", priority))
	}
	return priority, nil
}

// normalizeMirrorSource validates and canonicalizes an HTTP(S) mirror source and its credentials.
func normalizeMirrorSource(sourceURL, username, accessToken string) (string, string, string, error) {
	hasUsername := username != ""
	hasAccessToken := accessToken != ""
	if hasUsername != hasAccessToken {
		return "", "", "", errorx.MirrorSourceRepoAuthInvalid(
			errors.New("username and access token must be provided together"), errorx.Ctx(),
		)
	}

	sourceURL = strings.TrimRight(strings.TrimSpace(sourceURL), "/")
	parsedURL, err := url.Parse(sourceURL)
	if err != nil {
		return "", "", "", errorx.BadRequest(
			fmt.Errorf("invalid source git clone url: %w", err), errorx.Ctx(),
		)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", "", "", errorx.BadRequest(
			errors.New("source git clone url scheme must be http or https"), errorx.Ctx(),
		)
	}
	if parsedURL.Host == "" || parsedURL.Hostname() == "" {
		return "", "", "", errorx.BadRequest(
			errors.New("source git clone url must have a host"), errorx.Ctx(),
		)
	}
	if parsedURL.Path == "" || parsedURL.Path == "/" {
		return "", "", "", errorx.BadRequest(
			errors.New("source git clone url must have a repository path"), errorx.Ctx(),
		)
	}
	if parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return "", "", "", errorx.BadRequest(
			errors.New("source git clone url must not contain query or fragment"), errorx.Ctx(),
		)
	}
	if parsedURL.User != nil {
		if hasUsername {
			return "", "", "", errorx.MirrorSourceRepoAuthInvalid(
				errors.New("source URL and explicit credentials must not both contain authentication"), errorx.Ctx(),
			)
		}
		urlAccessToken, hasURLAccessToken := parsedURL.User.Password()
		if parsedURL.User.Username() == "" || !hasURLAccessToken || urlAccessToken == "" {
			return "", "", "", errorx.MirrorSourceRepoAuthInvalid(
				errors.New("source URL username and access token must be provided together"), errorx.Ctx(),
			)
		}
		username = parsedURL.User.Username()
		accessToken = urlAccessToken
		parsedURL.User = nil
	}
	if !strings.HasSuffix(parsedURL.Path, ".git") {
		parsedURL.Path += ".git"
	}
	return parsedURL.String(), username, accessToken, nil
}

// resolveMirrorRepoTarget chooses and trims the local mirror target path from fork fields or namespace mapping.
func (m *mirrorComponentImpl) resolveMirrorRepoTarget(req types.CreateMirrorRepoReq) (string, string) {
	namespace := req.ForkNamespace
	if namespace == "" {
		namespace = m.mapNamespaceAndName(req.SourceNamespace)
	}
	name := req.ForkName
	if name == "" {
		name = req.SourceName
	}
	return strings.TrimSpace(namespace), strings.TrimSpace(name)
}

// createMirrorRepoRecords creates mirror rows transactionally, and optionally the target repo rows too.
func (m *mirrorComponentImpl) createMirrorRepoRecords(ctx context.Context, req types.CreateMirrorRepoReq, repo *database.Repository, namespace, name string, createRepository bool, metadata *mirrorRepoMetadata) (*database.Mirror, error) {
	mirror := buildMirrorRepoRecord(req, repo, namespace, name)
	if !createRepository && !req.SkipSourcePath {
		sourceType, sourcePath, _ := common.GetSourceTypeAndPathFromURL(req.SourceGitCloneUrl)
		applyMirrorRepositorySourcePath(repo, sourceType, sourcePath)
	}
	mcpServer, mcpServerProperties, err := buildMCPServerRows(req.RepoType, req.MCPServerAttributes, metadata)
	if err != nil {
		return nil, err
	}
	var metadataUpdate *database.MirrorRepoMetadataUpdate
	if !createRepository {
		metadataUpdate, err = buildMirrorRepoMetadataUpdate(req, repo, metadata)
		if err != nil {
			return nil, err
		}
	}

	reqMirror, err := m.mirrorRepoStore.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository:          repo,
		CreateRepository:    createRepository,
		Metadata:            metadataUpdate,
		MCPServer:           mcpServer,
		MCPServerProperties: mcpServerProperties,
		Mirror:              mirror,
		Urgent:              req.Urgent,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror repo records: %w", err)
	}
	return reqMirror, nil
}

// prepareMirrorRepository validates repo creation inputs and builds the repository row.
func (m *mirrorComponentImpl) prepareMirrorRepository(ctx context.Context, req types.CreateRepoReq, sourceType, sourcePath string) (*database.Repository, error) {
	valid, err := common.IsValidName(req.Name)
	if !valid {
		slog.ErrorContext(ctx, "repo name is invalid", slog.Any("error", err))
		return nil, errorx.ErrRepoNameInvalid
	}

	namespace, err := m.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		slog.ErrorContext(ctx, "namespace does not exist", slog.Any("error", err))
		return nil, errorx.ErrNamespaceNotFound
	}

	user, err := m.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		slog.ErrorContext(ctx, "user does not exist", slog.Any("error", err))
		return nil, errorx.ErrUserNotFound
	}
	if user.Email == "" {
		slog.ErrorContext(ctx, "user email is empty", slog.Any("user", user))
		return nil, errorx.ErrUserEmailEmpty
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = types.MainBranch
	}

	repoPath := path.Join(namespace.Path, req.Name)
	repo := &database.Repository{
		UserID:         user.ID,
		Path:           repoPath,
		GitPath:        fmt.Sprintf("%ss_%s", string(req.RepoType), repoPath),
		Name:           req.Name,
		Nickname:       req.Nickname,
		Description:    req.Description,
		Private:        req.Private,
		License:        req.License,
		Readme:         req.Readme,
		DefaultBranch:  req.DefaultBranch,
		RepositoryType: req.RepoType,
		StarCount:      req.StarCount,
		User:           user,
	}
	applyMirrorRepositorySourcePath(repo, sourceType, sourcePath)
	return repo, nil
}

// applyMirrorRepositorySourcePath stores known upstream source paths on new repositories.
func applyMirrorRepositorySourcePath(repo *database.Repository, sourceType, sourcePath string) {
	switch sourceType {
	case enum.CSGSource:
		repo.CSGPath = sourcePath
	case enum.HFSource:
		repo.HFPath = sourcePath
	case enum.MSSource:
		repo.MSPath = sourcePath
	case enum.GitHubSource:
		repo.GithubPath = sourcePath
	}
}

// buildMCPServerRows converts fetched or caller-provided MCP metadata into transactional database rows.
func buildMCPServerRows(repoType types.RepositoryType, attributes types.MCPServerAttributes, metadata *mirrorRepoMetadata) (*database.MCPServer, []database.MCPServerProperty, error) {
	if repoType != types.MCPServerRepo {
		return nil, nil, nil
	}
	if metadata != nil && metadata.mcpServer != nil {
		return buildFetchedMCPServerRows(metadata.mcpServer)
	}

	configuration, err := json.Marshal(attributes.Configuration)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal mcp configuration: %w", err)
	}
	tools, err := json.Marshal(struct {
		Tools []types.MCPTool `json:"tools"`
	}{
		Tools: attributes.Tools,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal mcp tools: %w", err)
	}

	mcpServer := &database.MCPServer{
		ToolsNum:      len(attributes.Tools),
		Configuration: string(configuration),
		Schema:        string(tools),
		AvatarURL:     attributes.AvatarURL,
	}
	properties := make([]database.MCPServerProperty, 0, len(attributes.Tools))
	for _, tool := range attributes.Tools {
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal tool input schema: %w", err)
		}
		properties = append(properties, database.MCPServerProperty{
			Kind:        types.MCPPropTool,
			Name:        tool.Name,
			Description: tool.Description,
			Schema:      string(schema),
		})
	}
	return mcpServer, properties, nil
}

// buildFetchedMCPServerRows preserves full MCP metadata and derives tool properties when the schema includes tools.
func buildFetchedMCPServerRows(metadata *types.MCPServer) (*database.MCPServer, []database.MCPServerProperty, error) {
	mcpServer := &database.MCPServer{
		ToolsNum:        metadata.ToolsNum,
		Configuration:   metadata.Configuration,
		Schema:          metadata.Schema,
		ProgramLanguage: metadata.ProgramLanguage,
		RunMode:         metadata.RunMode,
		InstallDepsCmds: metadata.InstallDepsCmds,
		BuildCmds:       metadata.BuildCmds,
		LaunchCmds:      metadata.LaunchCmds,
		AvatarURL:       metadata.AvatarURL,
	}
	if strings.TrimSpace(metadata.Schema) == "" {
		return mcpServer, nil, nil
	}

	var schema struct {
		Tools []types.MCPTool `json:"tools"`
	}
	if err := json.Unmarshal([]byte(metadata.Schema), &schema); err != nil {
		// The MCP schema is still persisted verbatim when it is not the legacy tools wrapper.
		return mcpServer, nil, nil
	}
	if mcpServer.ToolsNum == 0 {
		mcpServer.ToolsNum = len(schema.Tools)
	}
	properties := make([]database.MCPServerProperty, 0, len(schema.Tools))
	for _, tool := range schema.Tools {
		inputSchema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal fetched mcp tool input schema: %w", err)
		}
		properties = append(properties, database.MCPServerProperty{
			Kind:        types.MCPPropTool,
			Name:        tool.Name,
			Description: tool.Description,
			Schema:      string(inputSchema),
		})
	}
	return mcpServer, properties, nil
}

// buildMirrorRepoRecord builds the mirror row that will be inserted transactionally.
func buildMirrorRepoRecord(req types.CreateMirrorRepoReq, repo *database.Repository, namespace, name string) database.Mirror {
	mirror := database.Mirror{
		SourceUrl:      req.SourceGitCloneUrl,
		MirrorSourceID: req.MirrorSourceID,
		Username:       req.Username,
		AccessToken:    req.AccessToken,
		Repository:     repo,
		SourceRepoPath: fmt.Sprintf("%s/%s", req.SourceNamespace, req.SourceName),
		Priority:       req.Priority,
	}

	sourceType, _, err := common.GetSourceTypeAndPathFromURL(req.SourceGitCloneUrl)
	if err != nil {
		sourceType = enum.OtherSource
	}
	mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s_%s", sourceType, req.RepoType, namespace, name)
	return mirror
}
