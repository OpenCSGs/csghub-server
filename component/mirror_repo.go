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
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/enum"
	"opencsg.com/csghub-server/common/utils/common"
)

// requeueMirrorRepoTask atomically schedules a new sync for an existing mirror target.
func (m *mirrorComponentImpl) requeueMirrorRepoTask(ctx context.Context, repo *database.Repository, mirror *database.Mirror, username, accessToken *string, priority types.MirrorPriority, urgent bool) (database.MirrorTask, error) {
	task, err := m.mirrorTaskJobStore.RequeueMirrorRepoTask(ctx, database.RequeueMirrorRepoTaskInput{
		MirrorID:        mirror.ID,
		RepositoryID:    repo.ID,
		Username:        username,
		AccessToken:     accessToken,
		Priority:        priority,
		Urgent:          urgent,
		JobClient:       m.mirrorRepoJobClient,
		JobCancelClient: m.mirrorJobClient,
	})
	if err != nil {
		return database.MirrorTask{}, fmt.Errorf("failed to create mirror task: %w", err)
	}
	return task, nil
}

// requeueMirrorFromSaas atomically replaces existing mirror work with a new workhub repo job.
func (m *mirrorComponentImpl) requeueMirrorFromSaas(ctx context.Context, repo *database.Repository, mirror *database.Mirror) (database.MirrorTask, error) {
	return m.requeueMirrorRepoTask(ctx, repo, mirror, nil, nil, types.LowMirrorPriority, false)
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
				var usernamePtr, accessTokenPtr *string
				if req.Username != "" {
					usernamePtr = &req.Username
					accessTokenPtr = &req.AccessToken
				}
				if _, err := m.requeueMirrorRepoTask(ctx, repo, mirror, usernamePtr, accessTokenPtr, req.Priority, req.Urgent); err != nil {
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

		return m.createMirrorRepoRecords(ctx, req, repo, namespace, name, false)
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

	sourceType, sourcePath, _ := common.GetSourceTypeAndPathFromURL(req.SourceGitCloneUrl)
	dbRepo, err := m.prepareMirrorRepository(ctx, createRepoReq, sourceType, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare mirror repository, error: %w", err)
	}

	return m.createMirrorRepoRecords(ctx, req, dbRepo, namespace, name, true)
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

// resolveMirrorRepoTarget chooses the local mirror target path from fork fields or namespace mapping.
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
func (m *mirrorComponentImpl) createMirrorRepoRecords(ctx context.Context, req types.CreateMirrorRepoReq, repo *database.Repository, namespace, name string, createRepository bool) (*database.Mirror, error) {
	mirror := buildMirrorRepoRecord(req, repo, namespace, name)
	if !createRepository {
		sourceType, sourcePath, _ := common.GetSourceTypeAndPathFromURL(req.SourceGitCloneUrl)
		applyMirrorRepositorySourcePath(repo, sourceType, sourcePath)
	}
	mcpServer, mcpServerProperties, err := buildMCPServerRows(req.RepoType, req.MCPServerAttributes)
	if err != nil {
		return nil, err
	}

	reqMirror, err := m.mirrorRepoStore.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository:          repo,
		CreateRepository:    createRepository,
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

	if _, err := m.namespaceStore.FindByPath(ctx, req.Namespace); err != nil {
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

	repoPath := path.Join(req.Namespace, req.Name)
	repo := &database.Repository{
		UserID:         user.ID,
		Path:           repoPath,
		GitPath:        fmt.Sprintf("%ss_%s", string(req.RepoType), repoPath),
		Name:           req.Name,
		Nickname:       req.Nickname,
		Description:    req.Description,
		Private:        req.Private,
		License:        req.License,
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

// buildMCPServerRows converts MCP mirror attributes into database rows before entering the transaction store.
func buildMCPServerRows(repoType types.RepositoryType, attributes types.MCPServerAttributes) (*database.MCPServer, []database.MCPServerProperty, error) {
	if repoType != types.MCPServerRepo {
		return nil, nil, nil
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
