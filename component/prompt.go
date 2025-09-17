package component

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

var (
	UserRole      string = "user"
	SystemRole    string = "system"
	AssistantRole string = "assistant"
)

type promptComponentImpl struct {
	config            *config.Config
	userStore         database.UserStore
	userLikeStore     database.UserLikesStore
	userSvcClient     rpc.UserSvcClient
	promptConvStore   database.PromptConversationStore
	promptPrefixStore database.PromptPrefixStore
	llmConfigStore    database.LLMConfigStore
	promptStore       database.PromptStore
	repoStore         database.RepoStore
	repoComponent     RepoComponent
	gitServer         gitserver.GitServer
	namespaceStore    database.NamespaceStore
	recomStore        database.RecomStore
	llmClient         *llm.Client
	maxPromptFS       int64
}

type PromptComponent interface {
	ListPrompt(ctx context.Context, req types.PromptReq) ([]types.PromptOutput, error)
	GetPrompt(ctx context.Context, req types.PromptReq) (*types.PromptOutput, error)
	ParseJsonFile(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (*types.PromptOutput, error)
	CreatePrompt(ctx context.Context, req types.PromptReq, body *types.CreatePromptReq) (*types.Prompt, error)
	UpdatePrompt(ctx context.Context, req types.PromptReq, body *types.UpdatePromptReq) (*types.Prompt, error)
	DeletePrompt(ctx context.Context, req types.PromptReq) error
	NewConversation(ctx context.Context, req types.ConversationTitleReq) (*database.PromptConversation, error)
	ListConversationsByUserID(ctx context.Context, currentUser string) ([]database.PromptConversation, error)
	GetConversation(ctx context.Context, req types.ConversationReq) (*database.PromptConversation, error)
	SubmitMessage(ctx context.Context, req types.ConversationReq) (<-chan string, error)
	SaveGeneratedText(ctx context.Context, req types.Conversation) (*database.PromptConversationMessage, error)
	RemoveConversation(ctx context.Context, req types.ConversationReq) error
	UpdateConversation(ctx context.Context, req types.ConversationTitleReq) (*database.PromptConversation, error)
	LikeConversationMessage(ctx context.Context, req types.ConversationMessageReq) error
	HateConversationMessage(ctx context.Context, req types.ConversationMessageReq) error
	SummarizeConversationTitle(ctx context.Context, req types.ConversationTitleReq) (*database.PromptConversation, error)
	SetRelationModels(ctx context.Context, req types.RelationModels) error
	AddRelationModel(ctx context.Context, req types.RelationModel) error
	DelRelationModel(ctx context.Context, req types.RelationModel) error
	CreatePromptRepo(ctx context.Context, req *types.CreatePromptRepoReq) (*types.PromptRes, error)
	IndexPromptRepo(ctx context.Context, filter *types.RepoFilter, per, page int) ([]types.PromptRes, int, error)
	UpdatePromptRepo(ctx context.Context, req *types.UpdatePromptRepoReq) (*types.PromptRes, error)
	RemoveRepo(ctx context.Context, namespace, name, currentUser string) error
	Show(ctx context.Context, namespace, name, currentUser string, needOpWeight, needMultiSync bool) (*types.PromptRes, error)
	Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error)
	OrgPrompts(ctx context.Context, req *types.OrgPromptsReq) ([]types.PromptRes, int, error)
}

func NewPromptComponent(cfg *config.Config) (PromptComponent, error) {
	r, err := NewRepoComponentImpl(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component,cause:%w", err)
	}
	gs, err := git.NewGitServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server,cause:%w", err)
	}
	usc := rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", cfg.User.Host, cfg.User.Port),
		rpc.AuthWithApiKey(cfg.APIToken))
	return &promptComponentImpl{
		config:            cfg,
		userStore:         database.NewUserStore(),
		userLikeStore:     database.NewUserLikesStore(),
		userSvcClient:     usc,
		promptConvStore:   database.NewPromptConversationStore(),
		promptPrefixStore: database.NewPromptPrefixStore(cfg),
		llmConfigStore:    database.NewLLMConfigStore(cfg),
		promptStore:       database.NewPromptStore(),
		llmClient:         llm.NewClient(),
		repoStore:         database.NewRepoStore(),
		repoComponent:     r,
		gitServer:         gs,
		maxPromptFS:       cfg.Dataset.PromptMaxJsonlFileSize,
		namespaceStore:    database.NewNamespaceStore(),
		recomStore:        database.NewRecomStore(),
	}, nil
}

func (c *promptComponentImpl) ListPrompt(ctx context.Context, req types.PromptReq) ([]types.PromptOutput, error) {
	r, err := c.repoStore.FindByPath(ctx, types.PromptRepo, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find prompt set, error: %w", err)
	}

	slog.Debug("ListPrompt check user permission begin")
	allow, err := c.repoComponent.AllowReadAccessRepo(ctx, r, req.CurrentUser)
	slog.Debug("ListPrompt check user permission end")
	if err != nil {
		return nil, fmt.Errorf("failed to check prompt set permission, error: %w", err)
	}
	if !allow {
		return nil, errorx.ErrUnauthorized
	}

	slog.Debug("ListPrompt get repo file tree begin")
	tree, err := GetFilePathObjects(ctx, req.Namespace, req.Name, "", types.PromptRepo, "", c.gitServer.GetTree)
	slog.Debug("ListPrompt get repo file tree end")
	if err != nil {
		return nil, fmt.Errorf("failed to get repo file tree, error: %w", err)
	}
	if tree == nil {
		return nil, fmt.Errorf("failed to find any files")
	}
	var prompts []types.PromptOutput
	wg := &sync.WaitGroup{}
	chPrompts := make(chan *types.PromptOutput, len(tree))
	done := make(chan struct{}, 1)

	go func() {
		for p := range chPrompts {
			prompts = append(prompts, *p)
		}
		done <- struct{}{}
	}()

	for _, file := range tree {
		if file.Lfs || file.Size > c.maxPromptFS {
			slog.Warn("ListPromp skip large prompt file", slog.Any("filePath", file.Path), slog.Int64("size", file.Size))
			continue
		}
		if !strings.HasSuffix(strings.ToLower(file.Path), ".jsonl") {
			continue
		}
		getFileContentReq := gitserver.GetRepoInfoByPathReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       types.MainBranch,
			Path:      file.Path,
			RepoType:  types.PromptRepo,
		}

		wg.Add(1)
		go func(req gitserver.GetRepoInfoByPathReq) {
			slog.Debug("ListPrompt parse prompt file begin", slog.String("file", req.Path))
			p, err := c.ParseJsonFile(ctx, getFileContentReq)
			if err != nil {
				slog.Warn("fail to parse jsonl file", slog.Any("getFileContentReq", getFileContentReq), slog.Any("error", err))
			}
			slog.Debug("ListPrompt parse prompt file end", slog.String("file", req.Path))
			chPrompts <- p
			wg.Done()
		}(getFileContentReq)
	}

	wg.Wait()
	close(chPrompts)
	<-done

	return prompts, nil
}

func (c *promptComponentImpl) GetPrompt(ctx context.Context, req types.PromptReq) (*types.PromptOutput, error) {
	r, err := c.repoStore.FindByPath(ctx, types.PromptRepo, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find prompt repo, error: %w", err)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, req.CurrentUser, r)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrUnauthorized
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       types.MainBranch,
		Path:      req.Path,
		RepoType:  types.PromptRepo,
	}
	p, err := c.ParseJsonFile(ctx, getFileContentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to parse jsonl %s, error: %w", req.Path, err)
	}
	p.CanWrite = permission.CanWrite
	p.CanManage = permission.CanAdmin
	return p, nil
}

func (c *promptComponentImpl) ParseJsonFile(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (*types.PromptOutput, error) {
	f, err := c.gitServer.GetRepoFileContents(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s contents, cause:%w", req.Path, err)
	}
	decodedContent, err := base64.StdEncoding.DecodeString(f.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode %s contents, cause:%w", req.Path, err)
	}
	var prompt types.Prompt
	err = yaml.Unmarshal(decodedContent, &prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to Unmarshal %s contents, cause: %w, decodedContent: %v", req.Path, err, string(decodedContent))
	}
	if len(prompt.Title) < 1 {
		prompt.Title = f.Name
	}
	po := types.PromptOutput{
		Prompt:   prompt,
		FilePath: req.Path,
	}
	return &po, nil
}

func (c *promptComponentImpl) CreatePrompt(ctx context.Context, req types.PromptReq, body *types.CreatePromptReq) (*types.Prompt, error) {
	u, err := c.checkPromptRepoPermission(ctx, req)
	if err != nil {
		return nil, errorx.ErrForbiddenMsg("user do not allowed create prompt")
	}
	req.Path = fmt.Sprintf("%s.jsonl", body.Title)
	exist, _ := c.checkFileExist(ctx, req)
	if exist {
		return nil, fmt.Errorf("prompt %s already exists", req.Path)
	}
	// generate json format string
	promptJson, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to convert prompt to JSON, cause: %w", err)
	}
	promptJsonStr := base64.StdEncoding.EncodeToString(promptJson)

	fileReq := types.CreateFileReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		Branch:      types.MainBranch,
		FilePath:    req.Path,
		Content:     promptJsonStr,
		RepoType:    types.PromptRepo,
		CurrentUser: req.CurrentUser,
		Username:    req.CurrentUser,
		Email:       u.Email,
		Message:     fmt.Sprintf("create prompt %s", req.Path),
	}
	_, err = c.repoComponent.CreateFile(ctx, &fileReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt file %s, cause: %w", req.Path, err)
	}
	return &body.Prompt, nil
}

func (c *promptComponentImpl) UpdatePrompt(ctx context.Context, req types.PromptReq, body *types.UpdatePromptReq) (*types.Prompt, error) {
	u, err := c.checkPromptRepoPermission(ctx, req)
	if err != nil {
		return nil, errorx.ErrForbiddenMsg("user do not allowed update prompt")
	}
	if !strings.HasSuffix(req.Path, ".jsonl") {
		return nil, fmt.Errorf("prompt name must be end with .jsonl")
	}
	exist, _ := c.checkFileExist(ctx, req)
	if !exist {
		return nil, fmt.Errorf("prompt %s does not exist", req.Path)
	}
	promptJson, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to convert prompt to JSON, cause: %w", err)
	}
	promptJsonStr := base64.StdEncoding.EncodeToString(promptJson)

	fileReq := types.UpdateFileReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		Branch:      types.MainBranch,
		FilePath:    req.Path,
		Content:     promptJsonStr,
		RepoType:    types.PromptRepo,
		CurrentUser: req.CurrentUser,
		Username:    req.CurrentUser,
		Email:       u.Email,
		Message:     fmt.Sprintf("update prompt %s", req.Path),
	}
	_, err = c.repoComponent.UpdateFile(ctx, &fileReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update prompt file %s, cause: %w", req.Path, err)
	}
	return &body.Prompt, nil
}

func (c *promptComponentImpl) DeletePrompt(ctx context.Context, req types.PromptReq) error {
	u, err := c.checkPromptRepoPermission(ctx, req)
	if err != nil {
		return errorx.ErrForbiddenMsg("user do not allowed delete prompt")
	}
	if !strings.HasSuffix(req.Path, ".jsonl") {
		return fmt.Errorf("prompt name must be end with .jsonl")
	}

	fileReq := types.DeleteFileReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		Branch:      types.MainBranch,
		FilePath:    req.Path,
		Content:     "",
		RepoType:    types.PromptRepo,
		CurrentUser: req.CurrentUser,
		Username:    req.CurrentUser,
		Email:       u.Email,
		Message:     fmt.Sprintf("delete prompt %s", req.Path),
		OriginPath:  "",
	}

	_, err = c.repoComponent.DeleteFile(ctx, &fileReq)
	if err != nil {
		return fmt.Errorf("failed to delete prompt %s, cause: %w", req.Path, err)
	}
	return nil
}

func (c *promptComponentImpl) checkFileExist(ctx context.Context, req types.PromptReq) (bool, error) {
	getFileRawReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       types.MainBranch,
		Path:      req.Path,
		RepoType:  types.PromptRepo,
	}
	_, err := c.gitServer.GetRepoFileRaw(ctx, getFileRawReq)
	if err != nil {
		return false, fmt.Errorf("failed to get prompt repository %s/%s file %s raw, error: %w", req.Namespace, req.Name, req.Path, err)
	}
	return true, nil
}

func (c *promptComponentImpl) checkPromptRepoPermission(ctx context.Context, req types.PromptReq) (*database.User, error) {
	namespace, err := c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleWrite)
			if err != nil {
				return nil, err
			}
			if !canWrite {
				return nil, errors.New("user do not have permission to update repo in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, errors.New("user do not have permission to update repo in this namespace")
			}
		}
	}
	return &user, nil
}

func (c *promptComponentImpl) SetRelationModels(ctx context.Context, req types.RelationModels) error {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("user does not exist, %w", err)
	}

	repo, err := c.repoStore.FindByPath(ctx, types.PromptRepo, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find prompt, error: %w", err)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return fmt.Errorf("failed to get user repo permission, error: %w", err)
	}

	if !permission.CanWrite {
		return errorx.ErrForbiddenMsg("user do not allowed to set relation models")
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       types.MainBranch,
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
	}
	metaMap, splits, err := GetMetaMapFromReadMe(c.gitServer, getFileContentReq)
	if err != nil {
		return fmt.Errorf("failed parse meta from readme, cause: %w", err)
	}
	metaMap["models"] = req.Models
	output, err := GetOutputForReadme(metaMap, splits)
	if err != nil {
		return fmt.Errorf("failed generate output for readme, cause: %w", err)
	}

	var readmeReq types.UpdateFileReq
	readmeReq.Branch = types.MainBranch
	readmeReq.Message = "update model relation tags"
	readmeReq.FilePath = types.REPOCARD_FILENAME
	readmeReq.RepoType = types.PromptRepo
	readmeReq.Namespace = req.Namespace
	readmeReq.Name = req.Name
	readmeReq.Username = req.CurrentUser
	readmeReq.Email = user.Email
	readmeReq.Content = base64.StdEncoding.EncodeToString([]byte(output))

	err = c.gitServer.UpdateRepoFile(&readmeReq)
	if err != nil {
		return fmt.Errorf("failed to set models tag to %s file, cause: %w", readmeReq.FilePath, err)
	}

	return nil
}

func GetMetaMapFromReadMe(git gitserver.GitServer, getFileContentReq gitserver.GetRepoInfoByPathReq) (map[string]any, []string, error) {
	f, err := git.GetRepoFileContents(context.Background(), getFileContentReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get readme.md contents, cause:%w", err)
	}
	decodedContent, err := base64.StdEncoding.DecodeString(f.Content)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to base64 decode readme.md contents, cause:%w", err)
	}
	decodedContentStr := string(decodedContent)
	// slog.Info("get prompt readme", slog.Any("decodedContentStr", decodedContentStr))

	splits := strings.Split(decodedContentStr, "---")
	// slog.Info("split readme", slog.Any("len(splits)", len(splits)), slog.Any("splits", splits))

	metaMap := make(map[string]any)
	if len(splits) > 1 {
		meta := splits[1]
		//parse yaml string
		err := yaml.Unmarshal([]byte(meta), metaMap)
		if err != nil {
			return nil, nil, fmt.Errorf("error unmarshall meta for prompt, cause: %w", err)
		}
	}
	return metaMap, splits, nil
}

func GetOutputForReadme(metaMap map[string]any, splits []string) (string, error) {
	yamlData, err := yaml.Marshal(metaMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metaMap to YAML, cause: %w", err)
	}
	metaOutput := strings.Join([]string{"---", string(yamlData), "---"}, "\n")

	output := ""
	if len(splits) == 0 {
		output = metaOutput
	} else if len(splits) == 1 {
		output = strings.Join([]string{metaOutput, splits[0]}, "\n")
	} else {
		splits[1] = metaOutput
		output = strings.Join(splits, "")
	}
	// slog.Debug("update prompt readme", slog.Any("output", output))
	return output, nil
}

func (c *promptComponentImpl) AddRelationModel(ctx context.Context, req types.RelationModel) error {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("user does not exist, %w", err)
	}

	_, err = c.repoStore.FindByPath(ctx, types.PromptRepo, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find prompt dataset, error: %w", err)
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       types.MainBranch,
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
	}
	metaMap, splits, err := GetMetaMapFromReadMe(c.gitServer, getFileContentReq)
	if err != nil {
		return fmt.Errorf("failed parse meta from readme, cause: %w", err)
	}
	models, ok := metaMap["models"]
	if !ok {
		models = []string{req.Model}
	} else {
		models = append(models.([]interface{}), req.Model)
	}
	metaMap["models"] = models
	output, err := GetOutputForReadme(metaMap, splits)
	if err != nil {
		return fmt.Errorf("failed generate output for readme, cause: %w", err)
	}

	var readmeReq types.UpdateFileReq
	readmeReq.Branch = types.MainBranch
	readmeReq.Message = "add relation model"
	readmeReq.FilePath = types.REPOCARD_FILENAME
	readmeReq.RepoType = types.PromptRepo
	readmeReq.Namespace = req.Namespace
	readmeReq.Name = req.Name
	readmeReq.Username = req.CurrentUser
	readmeReq.Email = user.Email
	readmeReq.Content = base64.StdEncoding.EncodeToString([]byte(output))

	err = c.gitServer.UpdateRepoFile(&readmeReq)
	if err != nil {
		return fmt.Errorf("failed to add model tag to %s file, cause: %w", readmeReq.FilePath, err)
	}

	return nil
}

func (c *promptComponentImpl) DelRelationModel(ctx context.Context, req types.RelationModel) error {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("user does not exist, %w", err)
	}

	_, err = c.repoStore.FindByPath(ctx, types.PromptRepo, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find prompt, error: %w", err)
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       types.MainBranch,
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
	}
	metaMap, splits, err := GetMetaMapFromReadMe(c.gitServer, getFileContentReq)
	if err != nil {
		return fmt.Errorf("failed parse meta from readme, cause: %w", err)
	}
	models, ok := metaMap["models"]
	if !ok {
		return nil
	} else {
		var newModels []string
		for _, v := range models.([]interface{}) {
			if v.(string) != req.Model {
				newModels = append(newModels, v.(string))
			}
		}
		metaMap["models"] = newModels
	}
	output, err := GetOutputForReadme(metaMap, splits)
	if err != nil {
		return fmt.Errorf("failed generate output for readme, cause: %w", err)
	}

	var readmeReq types.UpdateFileReq
	readmeReq.Branch = types.MainBranch
	readmeReq.Message = "delete relation model"
	readmeReq.FilePath = types.REPOCARD_FILENAME
	readmeReq.RepoType = types.PromptRepo
	readmeReq.Namespace = req.Namespace
	readmeReq.Name = req.Name
	readmeReq.Username = req.CurrentUser
	readmeReq.Email = user.Email
	readmeReq.Content = base64.StdEncoding.EncodeToString([]byte(output))

	err = c.gitServer.UpdateRepoFile(&readmeReq)
	if err != nil {
		return fmt.Errorf("failed to delete model tag to %s file, cause: %w", readmeReq.FilePath, err)
	}

	return nil
}

func (c *promptComponentImpl) CreatePromptRepo(ctx context.Context, req *types.CreatePromptRepoReq) (*types.PromptRes, error) {
	var (
		nickname string
		tags     []types.RepoTag
	)

	namespace, err := c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
			if err != nil {
				return nil, err
			}
			if !canWrite {
				return nil, errorx.ErrForbiddenMsg("users do not have permission to create prompt in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, errorx.ErrForbiddenMsg("users do not have permission to create prompt in this namespace")
			}
		}
	}

	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = types.MainBranch
	}

	req.RepoType = types.PromptRepo
	req.Readme = generateReadmeData(req.License)
	req.Nickname = nickname

	req.CommitFiles = []types.CommitFile{
		{
			Content: req.Readme,
			Path:    types.ReadmeFileName,
		},
		{
			Content: types.DatasetGitattributesContent,
			Path:    types.GitattributesFileName,
		},
	}
	_, dbRepo, err := c.repoComponent.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbPrompt := database.Prompt{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	repoPath := path.Join(req.Namespace, req.Name)
	prompt, err := c.promptStore.CreateAndUpdateRepoPath(ctx, dbPrompt, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database prompt, cause: %w", err)
	}

	for _, tag := range prompt.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey, //ShowName:  tag.ShowName,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	resPrompt := &types.PromptRes{
		ID:           prompt.ID,
		Name:         prompt.Repository.Name,
		Nickname:     prompt.Repository.Nickname,
		Description:  prompt.Repository.Description,
		Likes:        prompt.Repository.Likes,
		Downloads:    prompt.Repository.DownloadCount,
		Path:         prompt.Repository.Path,
		RepositoryID: prompt.RepositoryID,
		Private:      prompt.Repository.Private,
		User: types.User{
			Username: user.Username,
			Nickname: user.NickName,
			Email:    user.Email,
		},
		Tags:      tags,
		CreatedAt: prompt.CreatedAt,
		UpdatedAt: prompt.UpdatedAt,
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.PromptRepo,
			RepoPath:  prompt.Repository.Path,
			Operation: types.OperationCreate,
			UserUUID:  dbRepo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return resPrompt, nil
}

func (c *promptComponentImpl) IndexPromptRepo(ctx context.Context, filter *types.RepoFilter, per, page int) ([]types.PromptRes, int, error) {
	var (
		err        error
		resPrompts []types.PromptRes
	)
	repos, total, err := c.repoComponent.PublicToUser(ctx, types.PromptRepo, filter.Username, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public prompt repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	prompts, err := c.promptStore.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get prompts by repo ids,error:%w", err)
		return nil, 0, newError
	}

	//loop through repos to keep the repos in sort order
	for _, repo := range repos {
		var prompt *database.Prompt
		for _, d := range prompts {
			if repo.ID == d.RepositoryID {
				prompt = &d
				break
			}
		}
		if prompt == nil {
			continue
		}
		var tags []types.RepoTag
		for _, tag := range repo.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.I18nKey, //ShowName:  tag.ShowName,
				I18nKey:   tag.I18nKey,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}
		resPrompts = append(resPrompts, types.PromptRes{
			ID:           prompt.ID,
			Name:         repo.Name,
			Nickname:     repo.Nickname,
			Description:  repo.Description,
			Likes:        repo.Likes,
			Downloads:    repo.DownloadCount,
			Path:         repo.Path,
			RepositoryID: repo.ID,
			Private:      repo.Private,
			Tags:         tags,
			CreatedAt:    prompt.CreatedAt,
			UpdatedAt:    repo.UpdatedAt,
			Source:       repo.Source,
			SyncStatus:   repo.SyncStatus,
			License:      repo.License,

			User: types.User{
				Username: prompt.Repository.User.Username,
				Nickname: prompt.Repository.User.NickName,
				Email:    prompt.Repository.User.Email,
				Avatar:   prompt.Repository.User.Avatar,
			},
		})
	}

	return resPrompts, total, nil
}

func (c *promptComponentImpl) UpdatePromptRepo(ctx context.Context, req *types.UpdatePromptRepoReq) (*types.PromptRes, error) {
	req.RepoType = types.PromptRepo
	dbRepo, err := c.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	prompt, err := c.promptStore.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find prompt, error: %w", err)
	}

	// update times of prompt repo
	err = c.promptStore.Update(ctx, *prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to update prompt, error: %w", err)
	}

	resPrompt := &types.PromptRes{
		ID:           prompt.ID,
		Name:         dbRepo.Name,
		Nickname:     dbRepo.Nickname,
		Description:  dbRepo.Description,
		Likes:        dbRepo.Likes,
		Downloads:    dbRepo.DownloadCount,
		Path:         dbRepo.Path,
		RepositoryID: dbRepo.ID,
		Private:      dbRepo.Private,
		CreatedAt:    prompt.CreatedAt,
		UpdatedAt:    prompt.UpdatedAt,
	}

	return resPrompt, nil
}

func (c *promptComponentImpl) RemoveRepo(ctx context.Context, namespace, name, currentUser string) error {
	prompt, err := c.promptStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find prompt, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.PromptRepo,
	}
	repo, err := c.repoComponent.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of prompt, error: %w", err)
	}

	err = c.promptStore.Delete(ctx, *prompt)
	if err != nil {
		return fmt.Errorf("failed to delete database prompt, error: %w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.PromptRepo,
			RepoPath:  repo.Path,
			Operation: types.OperationDelete,
			UserUUID:  repo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return nil
}

func (c *promptComponentImpl) Show(ctx context.Context, namespace, name, currentUser string, needOpWeight, needMultiSync bool) (*types.PromptRes, error) {
	var tags []types.RepoTag
	prompt, err := c.promptStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find prompt, error: %w", err)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, currentUser, prompt.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrUnauthorized
	}

	ns, err := c.repoComponent.GetNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for prompt, error: %w", err)
	}

	for _, tag := range prompt.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey, //ShowName:  tag.ShowName,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	likeExists, err := c.userLikeStore.IsExist(ctx, currentUser, prompt.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}

	resPrompt := &types.PromptRes{
		ID:            prompt.ID,
		Name:          prompt.Repository.Name,
		Nickname:      prompt.Repository.Nickname,
		Description:   prompt.Repository.Description,
		Likes:         prompt.Repository.Likes,
		Downloads:     prompt.Repository.DownloadCount,
		Path:          prompt.Repository.Path,
		RepositoryID:  prompt.Repository.ID,
		DefaultBranch: prompt.Repository.DefaultBranch,
		Tags:          tags,
		User: types.User{
			Username: prompt.Repository.User.Username,
			Nickname: prompt.Repository.User.NickName,
			Email:    prompt.Repository.User.Email,
			Avatar:   prompt.Repository.User.Avatar,
		},
		Private:    prompt.Repository.Private,
		CreatedAt:  prompt.CreatedAt,
		UpdatedAt:  prompt.Repository.UpdatedAt,
		UserLikes:  likeExists,
		Source:     prompt.Repository.Source,
		SyncStatus: prompt.Repository.SyncStatus,
		License:    prompt.Repository.License,
		CanWrite:   permission.CanWrite,
		CanManage:  permission.CanAdmin,
		Namespace:  ns,
		MultiSource: types.MultiSource{
			HFPath:  prompt.Repository.HFPath,
			MSPath:  prompt.Repository.MSPath,
			CSGPath: prompt.Repository.CSGPath,
		},
	}
	if permission.CanAdmin {
		resPrompt.SensitiveCheckStatus = prompt.Repository.SensitiveCheckStatus.String()
	}

	if needOpWeight {
		c.addOpWeightToPrompts(ctx, []int64{resPrompt.RepositoryID}, []*types.PromptRes{resPrompt})
	}

	// add recom_scores to prompt
	if needMultiSync {
		weightNames := []database.RecomWeightName{database.RecomWeightFreshness,
			database.RecomWeightDownloads,
			database.RecomWeightQuality,
			database.RecomWeightOp,
			database.RecomWeightTotal}
		c.addWeightsToPrompt(ctx, resPrompt.RepositoryID, resPrompt, weightNames)
	}

	return resPrompt, nil
}

func (c *promptComponentImpl) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	prompt, err := c.promptStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find prompt repo, error: %w", err)
	}

	allow, _ := c.repoComponent.AllowReadAccessRepo(ctx, prompt.Repository, currentUser)
	if !allow {
		return nil, errorx.ErrUnauthorized
	}

	return c.getRelations(ctx, prompt.RepositoryID, currentUser)
}

func (c *promptComponentImpl) getRelations(ctx context.Context, repoID int64, currentUser string) (*types.Relations, error) {
	res, err := c.repoComponent.RelatedRepos(ctx, repoID, currentUser)
	if err != nil {
		return nil, err
	}
	rels := new(types.Relations)
	modelRepos := res[types.ModelRepo]
	for _, repo := range modelRepos {
		rels.Models = append(rels.Models, &types.Model{
			Path:        repo.Path,
			Name:        repo.Name,
			Nickname:    repo.Nickname,
			Description: repo.Description,
			UpdatedAt:   repo.UpdatedAt,
			Private:     repo.Private,
			Downloads:   repo.DownloadCount,
		})
	}

	return rels, nil
}

var _ types.SensitiveRequestV2 = (*types.Prompt)(nil)

func (c *promptComponentImpl) OrgPrompts(ctx context.Context, req *types.OrgPromptsReq) ([]types.PromptRes, int, error) {
	var resPrompts []types.PromptRes
	var err error
	r := membership.RoleUnknown
	if req.CurrentUser != "" {
		r, err = c.userSvcClient.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
		// log error, and treat user as unknown role in org
		if err != nil {
			slog.Error("faild to get member role",
				slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
				slog.String("error", err.Error()))
		}
	}
	onlyPublic := !r.CanRead()
	prompts, total, err := c.promptStore.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get user prompts,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range prompts {
		resPrompts = append(resPrompts, types.PromptRes{
			ID:           data.ID,
			Name:         data.Repository.Name,
			Nickname:     data.Repository.Nickname,
			Description:  data.Repository.Description,
			Likes:        data.Repository.Likes,
			Downloads:    data.Repository.DownloadCount,
			Path:         data.Repository.Path,
			RepositoryID: data.RepositoryID,
			Private:      data.Repository.Private,
			CreatedAt:    data.CreatedAt,
			UpdatedAt:    data.Repository.UpdatedAt,
		})
	}

	return resPrompts, total, nil
}

func (c *promptComponentImpl) addWeightsToPrompt(ctx context.Context, repoID int64, resPrompt *types.PromptRes, weightNames []database.RecomWeightName) {
	weights, err := c.recomStore.FindByRepoIDs(ctx, []int64{repoID})
	if err == nil {
		resPrompt.Scores = make([]types.WeightScore, 0)
		for _, weight := range weights {
			if slices.Contains(weightNames, weight.WeightName) {
				score := types.WeightScore{
					WeightName: string(weight.WeightName),
					Score:      weight.Score,
				}
				resPrompt.Scores = append(resPrompt.Scores, score)
			}
		}
	}
}
