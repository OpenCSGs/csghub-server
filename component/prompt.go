package component

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

var (
	UserRole      string = "user"
	SystemRole    string = "system"
	AssistantRole string = "assistant"
)

type PromptComponent struct {
	gs   gitserver.GitServer
	user *database.UserStore
	pc   *database.PromptConversationStore
	pp   *database.PromptPrefixStore
	lc   *database.LLMConfigStore
	pt   *database.PromptStore
	llm  *llm.Client
	*RepoComponent
	maxPromptFS int64
}

func NewPromptComponent(cfg *config.Config) (*PromptComponent, error) {
	r, err := NewRepoComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component,cause:%w", err)
	}
	gs, err := git.NewGitServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server,cause:%w", err)
	}
	return &PromptComponent{
		gs:            gs,
		user:          database.NewUserStore(),
		pc:            database.NewPromptConversationStore(),
		pp:            database.NewPromptPrefixStore(),
		lc:            database.NewLLMConfigStore(),
		pt:            database.NewPromptStore(),
		llm:           llm.NewClient(),
		RepoComponent: r,
		maxPromptFS:   cfg.Dataset.PromptMaxJsonlFileSize,
	}, nil
}

func (c *PromptComponent) ListPrompt(ctx context.Context, req types.PromptReq) ([]PromptOutput, error) {
	r, err := c.repo.FindByPath(ctx, types.PromptRepo, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	allow, err := c.AllowReadAccessRepo(ctx, r, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check dataset permission, error: %w", err)
	}
	if !allow {
		return nil, ErrUnauthorized
	}

	slog.Debug("ListPrompt get repo file tree begin")
	tree, err := GetFilePathObjects(req.Namespace, req.Name, "", types.PromptRepo, "", c.git.GetRepoFileTree)
	slog.Debug("ListPrompt get repo file tree end")
	if err != nil {
		return nil, fmt.Errorf("failed to get repo file tree, error: %w", err)
	}
	if tree == nil {
		return nil, fmt.Errorf("failed to find any files")
	}
	var prompts []PromptOutput
	wg := &sync.WaitGroup{}
	chPrompts := make(chan *PromptOutput, len(tree))
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

func (c *PromptComponent) GetPrompt(ctx context.Context, req types.PromptReq) (*PromptOutput, error) {
	r, err := c.repo.FindByPath(ctx, types.PromptRepo, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find prompt repo, error: %w", err)
	}

	permission, err := c.getUserRepoPermission(ctx, req.CurrentUser, r)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrUnauthorized
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

func (c *PromptComponent) ParseJsonFile(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (*PromptOutput, error) {
	f, err := c.gs.GetRepoFileContents(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s contents, cause:%w", req.Path, err)
	}
	decodedContent, err := base64.StdEncoding.DecodeString(f.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode %s contents, cause:%w", req.Path, err)
	}
	var prompt Prompt
	err = yaml.Unmarshal(decodedContent, &prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to Unmarshal %s contents, cause: %w, decodedContent: %v", req.Path, err, string(decodedContent))
	}
	if len(prompt.Title) < 1 {
		prompt.Title = f.Name
	}
	po := PromptOutput{
		Prompt:   prompt,
		FilePath: req.Path,
	}
	return &po, nil
}

func (c *PromptComponent) CreatePrompt(ctx context.Context, req types.PromptReq, body *CreatePromptReq) (*Prompt, error) {
	u, err := c.checkPromptRepoPermission(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("user do not allowed create prompt")
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
	_, err = c.CreateFile(ctx, &fileReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt file %s, cause: %w", req.Path, err)
	}
	return &body.Prompt, nil
}

func (c *PromptComponent) UpdatePrompt(ctx context.Context, req types.PromptReq, body *UpdatePromptReq) (*Prompt, error) {
	u, err := c.checkPromptRepoPermission(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("user do not allowed update prompt")
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
	_, err = c.UpdateFile(ctx, &fileReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update prompt file %s, cause: %w", req.Path, err)
	}
	return &body.Prompt, nil
}

func (c *PromptComponent) DeletePrompt(ctx context.Context, req types.PromptReq) error {
	u, err := c.checkPromptRepoPermission(ctx, req)
	if err != nil {
		return fmt.Errorf("user do not allowed delete prompt")
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

	_, err = c.DeleteFile(ctx, &fileReq)
	if err != nil {
		return fmt.Errorf("failed to delete prompt %s, cause: %w", req.Path, err)
	}
	return nil
}

func (c *PromptComponent) checkFileExist(ctx context.Context, req types.PromptReq) (bool, error) {
	getFileRawReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       types.MainBranch,
		Path:      req.Path,
		RepoType:  types.PromptRepo,
	}
	_, err := c.git.GetRepoFileRaw(ctx, getFileRawReq)
	if err != nil {
		return false, fmt.Errorf("failed to get prompt repository %s/%s file %s raw, error: %w", req.Namespace, req.Name, req.Path, err)
	}
	return true, nil
}

func (c *PromptComponent) checkPromptRepoPermission(ctx context.Context, req types.PromptReq) (*database.User, error) {
	namespace, err := c.namespace.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.checkCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleWrite)
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

func (c *PromptComponent) NewConversation(ctx context.Context, req types.ConversationTitleReq) (*database.PromptConversation, error) {
	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	conversation := database.PromptConversation{
		UserID:         user.ID,
		ConversationID: req.Uuid,
		Title:          req.Title,
	}

	err = c.pc.CreateConversation(ctx, conversation)
	if err != nil {
		return nil, fmt.Errorf("new conversation error: %w", err)
	}

	return &conversation, nil
}

func (c *PromptComponent) ListConversationsByUserID(ctx context.Context, currentUser string) ([]database.PromptConversation, error) {
	user, err := c.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	conversations, err := c.pc.FindConversationsByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("find conversations by user %s error: %w", currentUser, err)
	}
	return conversations, nil
}

func (c *PromptComponent) GetConversation(ctx context.Context, req types.ConversationReq) (*database.PromptConversation, error) {
	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	conversation, err := c.pc.GetConversationByID(ctx, user.ID, req.Uuid, true)
	if err != nil {
		return nil, fmt.Errorf("get conversation by id %s error: %w", req.Uuid, err)
	}
	return conversation, nil
}

func (c *PromptComponent) SubmitMessage(ctx context.Context, req types.ConversationReq) (<-chan string, error) {
	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	_, err = c.pc.GetConversationByID(ctx, user.ID, req.Uuid, false)
	if err != nil {
		return nil, fmt.Errorf("invalid conversation by uuid %s error: %w", req.Uuid, err)
	}

	reqMsg := database.PromptConversationMessage{
		ConversationID: req.Uuid,
		Role:           UserRole,
		Content:        req.Message,
	}
	_, err = c.pc.SaveConversationMessage(ctx, reqMsg)
	if err != nil {
		return nil, fmt.Errorf("save user prompt input error: %w", err)
	}

	llmConfig, err := c.lc.GetOptimization(ctx)
	if err != nil {
		return nil, fmt.Errorf("get llm config error: %w", err)
	}
	slog.Debug("use llm", slog.Any("llmConfig", llmConfig))
	var headers map[string]string
	err = json.Unmarshal([]byte(llmConfig.AuthHeader), &headers)
	if err != nil {
		return nil, fmt.Errorf("parse llm config header error: %w", err)
	}

	promptPrefix := ""
	prefix, err := c.pp.Get(ctx)
	if err != nil {
		slog.Warn("fail to find prompt prefix", slog.Any("err", err))
	} else {
		chs := isChinese(reqMsg.Content)
		if chs {
			promptPrefix = prefix.ZH
		} else {
			promptPrefix = prefix.EN
		}
	}

	reqData := types.LLMReqBody{
		Model: llmConfig.ModelName,
		Messages: []types.LLMMessage{
			{Role: SystemRole, Content: promptPrefix},
			{Role: UserRole, Content: reqMsg.Content},
		},
		Stream:      true,
		Temperature: 0.2,
	}
	if req.Temperature != nil {
		reqData.Temperature = *req.Temperature
	}

	slog.Debug("llm request", slog.Any("reqData", reqData))
	ch, err := c.llm.Chat(ctx, llmConfig.ApiEndpoint, headers, reqData)
	if err != nil {
		return nil, fmt.Errorf("call llm error: %w", err)
	}
	return ch, nil
}

func (c *PromptComponent) SaveGeneratedText(ctx context.Context, req types.Conversation) (*database.PromptConversationMessage, error) {
	respMsg := database.PromptConversationMessage{
		ConversationID: req.Uuid,
		Role:           AssistantRole,
		Content:        req.Message,
	}
	msg, err := c.pc.SaveConversationMessage(ctx, respMsg)
	if err != nil {
		return nil, fmt.Errorf("save system generated response error: %w", err)
	}
	return msg, nil
}

func (c *PromptComponent) RemoveConversation(ctx context.Context, req types.ConversationReq) error {
	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return errors.New("user does not exist")
	}

	err = c.pc.DeleteConversationsByID(ctx, user.ID, req.Uuid)
	if err != nil {
		return fmt.Errorf("remove conversation error: %w", err)
	}
	return nil
}

func (c *PromptComponent) UpdateConversation(ctx context.Context, req types.ConversationTitleReq) (*database.PromptConversation, error) {
	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	err = c.pc.UpdateConversation(ctx, database.PromptConversation{
		UserID:         user.ID,
		ConversationID: req.Uuid,
		Title:          req.Title,
	})
	if err != nil {
		return nil, fmt.Errorf("update conversation title error: %w", err)
	}

	resp, err := c.pc.GetConversationByID(ctx, user.ID, req.Uuid, false)
	if err != nil {
		return nil, fmt.Errorf("invalid conversation by uuid %s error: %w", req.Uuid, err)
	}
	return resp, nil
}

func (c *PromptComponent) LikeConversationMessage(ctx context.Context, req types.ConversationMessageReq) error {
	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return errors.New("user does not exist")
	}
	_, err = c.pc.GetConversationByID(ctx, user.ID, req.Uuid, false)
	if err != nil {
		return fmt.Errorf("invalid conversation by uuid %s error: %w", req.Uuid, err)
	}
	err = c.pc.LikeMessageByID(ctx, req.Id)
	if err != nil {
		return fmt.Errorf("update like message by id %d error: %w", req.Id, err)
	}
	return nil
}

func (c *PromptComponent) HateConversationMessage(ctx context.Context, req types.ConversationMessageReq) error {
	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return errors.New("user does not exist")
	}
	_, err = c.pc.GetConversationByID(ctx, user.ID, req.Uuid, false)
	if err != nil {
		return fmt.Errorf("invalid conversation by uuid %s error: %w", req.Uuid, err)
	}
	err = c.pc.HateMessageByID(ctx, req.Id)
	if err != nil {
		return fmt.Errorf("update hate message by id %d error: %w", req.Id, err)
	}
	return nil
}

func isChinese(s string) bool {
	re := regexp.MustCompile(`[\p{Han}]`)
	return re.MatchString(s)
}

func (c *PromptComponent) SetRelationModels(ctx context.Context, req types.RelationModels) error {
	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("user does not exist, %w", err)
	}

	if !user.CanAdmin() {
		return fmt.Errorf("only admin was allowed to set models for prompt")
	}

	_, err = c.repo.FindByPath(ctx, types.PromptRepo, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find prompt dataset, error: %w", err)
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       types.MainBranch,
		Path:      REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
	}
	metaMap, splits, err := GetMetaMapFromReadMe(c.gs, getFileContentReq)
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
	readmeReq.FilePath = REPOCARD_FILENAME
	readmeReq.RepoType = types.PromptRepo
	readmeReq.Namespace = req.Namespace
	readmeReq.Name = req.Name
	readmeReq.Username = req.CurrentUser
	readmeReq.Email = user.Email
	readmeReq.Content = base64.StdEncoding.EncodeToString([]byte(output))

	err = c.git.UpdateRepoFile(&readmeReq)
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

func (c *PromptComponent) AddRelationModel(ctx context.Context, req types.RelationModel) error {
	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("user does not exist, %w", err)
	}

	if !user.CanAdmin() {
		return fmt.Errorf("only admin was allowed to set models for prompt")
	}

	_, err = c.repo.FindByPath(ctx, types.PromptRepo, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find prompt dataset, error: %w", err)
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       types.MainBranch,
		Path:      REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
	}
	metaMap, splits, err := GetMetaMapFromReadMe(c.gs, getFileContentReq)
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
	readmeReq.FilePath = REPOCARD_FILENAME
	readmeReq.RepoType = types.PromptRepo
	readmeReq.Namespace = req.Namespace
	readmeReq.Name = req.Name
	readmeReq.Username = req.CurrentUser
	readmeReq.Email = user.Email
	readmeReq.Content = base64.StdEncoding.EncodeToString([]byte(output))

	err = c.git.UpdateRepoFile(&readmeReq)
	if err != nil {
		return fmt.Errorf("failed to add model tag to %s file, cause: %w", readmeReq.FilePath, err)
	}

	return nil
}

func (c *PromptComponent) DelRelationModel(ctx context.Context, req types.RelationModel) error {
	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("user does not exist, %w", err)
	}

	if !user.CanAdmin() {
		return fmt.Errorf("only admin was allowed to delete model for prompt")
	}

	_, err = c.repo.FindByPath(ctx, types.PromptRepo, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find prompt, error: %w", err)
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       types.MainBranch,
		Path:      REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
	}
	metaMap, splits, err := GetMetaMapFromReadMe(c.git, getFileContentReq)
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
	readmeReq.FilePath = REPOCARD_FILENAME
	readmeReq.RepoType = types.PromptRepo
	readmeReq.Namespace = req.Namespace
	readmeReq.Name = req.Name
	readmeReq.Username = req.CurrentUser
	readmeReq.Email = user.Email
	readmeReq.Content = base64.StdEncoding.EncodeToString([]byte(output))

	err = c.git.UpdateRepoFile(&readmeReq)
	if err != nil {
		return fmt.Errorf("failed to delete model tag to %s file, cause: %w", readmeReq.FilePath, err)
	}

	return nil
}

func (c *PromptComponent) CreatePromptRepo(ctx context.Context, req *types.CreatePromptRepoReq) (*types.PromptRes, error) {
	var (
		nickname string
		tags     []types.RepoTag
	)

	namespace, err := c.namespace.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.checkCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
			if err != nil {
				return nil, err
			}
			if !canWrite {
				return nil, errors.New("users do not have permission to create prompt in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, errors.New("users do not have permission to create prompt in this namespace")
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
	_, dbRepo, err := c.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbPrompt := database.Prompt{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	prompt, err := c.pt.Create(ctx, dbPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to create database prompt, cause: %w", err)
	}

	// Create README.md file
	err = c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   req.Readme,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  readmeFileName,
	}, types.PromptRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt repo README.md file, cause: %w", err)
	}

	// Create .gitattributes file
	err = c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   datasetGitattributesContent,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  gitattributesFileName,
	}, types.PromptRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create .gitattributes file, cause: %w", err)
	}

	for _, tag := range prompt.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.ShowName,
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
		Repository:   common.BuildCloneInfo(c.config, prompt.Repository),
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

	return resPrompt, nil
}

func (c *PromptComponent) IndexPromptRepo(ctx context.Context, filter *types.RepoFilter, per, page int) ([]types.PromptRes, int, error) {
	var (
		err        error
		resPrompts []types.PromptRes
	)
	repos, total, err := c.PublicToUser(ctx, types.PromptRepo, filter.Username, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public prompt repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	prompts, err := c.pt.ByRepoIDs(ctx, repoIDs)
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
				ShowName:  tag.ShowName,
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
			Repository:   common.BuildCloneInfo(c.config, prompt.Repository),

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

func (c *PromptComponent) UpdatePromptRepo(ctx context.Context, req *types.UpdatePromptRepoReq) (*types.PromptRes, error) {
	req.RepoType = types.PromptRepo
	dbRepo, err := c.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	prompt, err := c.pt.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find prompt, error: %w", err)
	}

	// update times of prompt repo
	err = c.pt.Update(ctx, *prompt)
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

func (c *PromptComponent) RemoveRepo(ctx context.Context, namespace, name, currentUser string) error {
	prompt, err := c.pt.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find prompt, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.PromptRepo,
	}
	_, err = c.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of prompt, error: %w", err)
	}

	err = c.pt.Delete(ctx, *prompt)
	if err != nil {
		return fmt.Errorf("failed to delete database prompt, error: %w", err)
	}
	return nil
}

func (c *PromptComponent) Show(ctx context.Context, namespace, name, currentUser string) (*types.PromptRes, error) {
	var tags []types.RepoTag
	prompt, err := c.pt.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find prompt, error: %w", err)
	}

	permission, err := c.getUserRepoPermission(ctx, currentUser, prompt.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrUnauthorized
	}

	ns, err := c.getNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for prompt, error: %w", err)
	}

	for _, tag := range prompt.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.ShowName,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	likeExists, err := c.uls.IsExist(ctx, currentUser, prompt.Repository.ID)
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
		Repository:    common.BuildCloneInfo(c.config, prompt.Repository),
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
	}

	return resPrompt, nil
}

func (c *PromptComponent) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	prompt, err := c.pt.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find prompt repo, error: %w", err)
	}

	allow, _ := c.AllowReadAccessRepo(ctx, prompt.Repository, currentUser)
	if !allow {
		return nil, ErrUnauthorized
	}

	return c.getRelations(ctx, prompt.RepositoryID, currentUser)
}

func (c *PromptComponent) getRelations(ctx context.Context, repoID int64, currentUser string) (*types.Relations, error) {
	res, err := c.relatedRepos(ctx, repoID, currentUser)
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

type Prompt struct {
	Title     string   `json:"title" binding:"required"`
	Content   string   `json:"content" binding:"required"`
	Language  string   `json:"language" binding:"required"`
	Tags      []string `json:"tags"`
	Type      string   `json:"type"` // "text|image|video|audio"
	Source    string   `json:"source"`
	Author    string   `json:"author"`
	Time      string   `json:"time"`
	Copyright string   `json:"copyright"`
	Feedback  []string `json:"feedback"`
}

type PromptOutput struct {
	Prompt
	FilePath  string `json:"file_path"`
	CanWrite  bool   `json:"can_write"`
	CanManage bool   `json:"can_manage"`
}

type CreatePromptReq struct {
	Prompt
}

type UpdatePromptReq struct {
	Prompt
}

var _ types.SensitiveRequestV2 = (*Prompt)(nil)

func (req *Prompt) GetSensitiveFields() []types.SensitiveField {
	var fields []types.SensitiveField
	fields = append(fields, types.SensitiveField{
		Name: "title",
		Value: func() string {
			return req.Title
		},
		Scenario: string(sensitive.ScenarioCommentDetection),
	})
	fields = append(fields, types.SensitiveField{
		Name: "content",
		Value: func() string {
			return req.Content
		},
		Scenario: string(sensitive.ScenarioCommentDetection),
	})
	if len(req.Source) > 0 {
		fields = append(fields, types.SensitiveField{
			Name: "source",
			Value: func() string {
				return req.Source
			},
			Scenario: string(sensitive.ScenarioCommentDetection),
		})
	}
	return fields
}
