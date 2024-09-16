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

	"gopkg.in/yaml.v3"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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
		llm:           llm.NewClient(),
		RepoComponent: r,
		maxPromptFS:   cfg.Dataset.PromptMaxJsonlFileSize,
	}, nil
}

func (c *PromptComponent) ListPrompt(ctx context.Context, req types.PromptReq) ([]types.Prompt, error) {
	r, err := c.repo.FindByPath(ctx, types.DatasetRepo, req.Namespace, req.Name)
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

	tree, err := GetFilePathObjects(req.Namespace, req.Name, "", types.DatasetRepo, c.git.GetRepoFileTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo file tree, error: %w", err)
	}
	if tree == nil {
		return nil, fmt.Errorf("failed to find any files")
	}
	var prompts []types.Prompt
	for _, file := range tree {
		if file.Lfs || file.Size > c.maxPromptFS {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(file.Path), ".jsonl") {
			continue
		}
		getFileContentReq := gitserver.GetRepoInfoByPathReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       "main",
			Path:      file.Path,
			RepoType:  types.DatasetRepo,
		}
		p, err := c.ParseJsonFile(ctx, getFileContentReq)
		if err != nil {
			slog.Warn("fail to parse jsonl file", slog.Any("getFileContentReq", getFileContentReq), slog.Any("error", err))
			continue
		}
		prompts = append(prompts, *p)
	}
	return prompts, nil
}

func (c *PromptComponent) GetPrompt(ctx context.Context, req types.PromptReq) (*types.Prompt, error) {
	r, err := c.repo.FindByPath(ctx, types.DatasetRepo, req.Namespace, req.Name)
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

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       "main",
		Path:      req.Path,
		RepoType:  types.DatasetRepo,
	}
	p, err := c.ParseJsonFile(ctx, getFileContentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to parse jsonl file %s, error: %w", req.Path, err)
	}
	return p, nil
}

func (c *PromptComponent) ParseJsonFile(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (*types.Prompt, error) {
	f, err := c.gs.GetRepoFileContents(ctx, req)
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
	prompt.FilePath = req.Path
	return &prompt, nil
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
	err = c.pc.SaveConversationMessage(ctx, reqMsg)
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

func (c *PromptComponent) SaveGeneratedText(ctx context.Context, req types.Conversation) error {
	respMsg := database.PromptConversationMessage{
		ConversationID: req.Uuid,
		Role:           AssistantRole,
		Content:        req.Message,
	}
	err := c.pc.SaveConversationMessage(ctx, respMsg)
	if err != nil {
		return fmt.Errorf("save system generated response error: %w", err)
	}
	return nil
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
