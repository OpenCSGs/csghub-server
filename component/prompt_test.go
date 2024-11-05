package component

import (
	"context"
	"testing"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func InitTestDB(t *testing.T) *config.Config {
	cfg, err := config.LoadConfig()
	cfg.GitServer.Type = types.GitServerTypeGitaly
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if len(tests.TestDSN) > 0 {
		cfg.Database.DSN = tests.TestDSN
	}
	dbConfig := database.DBConfig{
		Dialect: database.DatabaseDialect(cfg.Database.Driver),
		DSN:     cfg.Database.DSN,
	}
	database.InitDB(dbConfig)
	return cfg
}

func TestCreatePrompt(t *testing.T) {
	cfg := InitTestDB(t)
	pc, err := NewPromptComponent(cfg)
	if err != nil {
		t.Fatalf("failed to create prompt component:  %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := types.PromptReq{
		Namespace:   tests.TestPromptNamespace,
		Name:        tests.TestPromptName,
		CurrentUser: tests.CurrentUser,
	}

	testTitle := "test1"
	testContent := "test prompt1"
	testLanguage := "zh"
	testSource := "https://github.com/prompt"

	body := CreatePromptReq{
		Prompt: Prompt{
			Title:    testTitle,
			Content:  testContent,
			Language: testLanguage,
			Source:   testSource,
		},
	}

	testRes, err := pc.CreatePrompt(ctx, req, &body)
	if err != nil {
		t.Errorf("failed to create prompt: %v", err)
	}
	if testRes.Title != testTitle || testRes.Content != testContent || testRes.Language != testLanguage || testRes.Source != testSource {
		t.Errorf("failed to create prompt with %v", body)
	}
}

func TestUpdatePrompt(t *testing.T) {
	cfg := InitTestDB(t)
	pc, err := NewPromptComponent(cfg)
	if err != nil {
		t.Fatalf("failed to create prompt component:  %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := types.PromptReq{
		Namespace:   tests.TestPromptNamespace,
		Name:        tests.TestPromptName,
		CurrentUser: tests.CurrentUser,
		Path:        "test1.jsonl",
	}

	updateTitle := "test2"
	updateContent := "test prompt2"
	updateLanguage := "en"
	updateSource := "https://github.com/prompt2"

	body := UpdatePromptReq{
		Prompt: Prompt{
			Title:    updateTitle,
			Content:  updateContent,
			Language: updateLanguage,
			Source:   updateSource,
		},
	}
	testRes, err := pc.UpdatePrompt(ctx, req, &body)
	if err != nil {
		t.Errorf("failed to update prompt: %v", err)
	}

	if testRes.Title != updateTitle || testRes.Content != updateContent || testRes.Language != updateLanguage || testRes.Source != updateSource {
		t.Errorf("failed to update prompt with %v", body)
	}
}

func TestDeletePrompt(t *testing.T) {
	cfg := InitTestDB(t)
	pc, err := NewPromptComponent(cfg)
	if err != nil {
		t.Fatalf("failed to create prompt component:  %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := types.PromptReq{
		Namespace:   tests.TestPromptNamespace,
		Name:        tests.TestPromptName,
		CurrentUser: tests.CurrentUser,
		Path:        "test1.jsonl",
	}

	err = pc.DeletePrompt(ctx, req)
	if err != nil {
		t.Errorf("failed to delete prompt: %v", err)
	}
}

func TestCreatePromptRepo(t *testing.T) {
	cfg := InitTestDB(t)
	pc, err := NewPromptComponent(cfg)
	if err != nil {
		t.Fatalf("failed to create prompt component:  %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := types.CreatePromptRepoReq{
		CreateRepoReq: types.CreateRepoReq{
			Username:    tests.CurrentUser,
			Namespace:   tests.TestNewPromptNamespace,
			Name:        tests.TestNewPromptName,
			Nickname:    "mynickname",
			Description: "this is test prompt repo",
			Private:     true,
			License:     "MIT",
			Labels:      "text",
			Readme:      "test prompt",
		},
	}

	testRes, err := pc.CreatePromptRepo(ctx, &req)
	if err != nil {
		t.Errorf("failed to create repo: %v", err)
	}
	if testRes.Name != tests.TestNewPromptName || testRes.Nickname != "mynickname" {
		t.Errorf("failed to create repo with %v", req)
	}
}

func TestUpdatePromptRepo(t *testing.T) {
	cfg := InitTestDB(t)
	pc, err := NewPromptComponent(cfg)
	if err != nil {
		t.Fatalf("failed to create prompt component:  %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	NewNickName := "mynewnickname"
	NewDesc := "this is test prompt repo"
	NewPrivate := true

	req := types.UpdatePromptRepoReq{
		UpdateRepoReq: types.UpdateRepoReq{
			Username:    tests.CurrentUser,
			Namespace:   tests.TestNewPromptNamespace,
			Name:        tests.TestNewPromptName,
			Nickname:    &NewNickName,
			Description: &NewDesc,
			Private:     &NewPrivate,
		},
	}

	testRes, err := pc.UpdatePromptRepo(ctx, &req)
	if err != nil {
		t.Errorf("failed to update repo: %v", err)
	}

	if testRes.Nickname != NewNickName || testRes.Description != NewDesc || !testRes.Private {
		t.Errorf("failed to update repo with %v", req)
	}
}

func TestRemoveRepo(t *testing.T) {
	cfg := InitTestDB(t)
	pc, err := NewPromptComponent(cfg)
	if err != nil {
		t.Fatalf("failed to create prompt component:  %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = pc.RemoveRepo(ctx, tests.TestNewPromptNamespace, tests.TestNewPromptName, tests.CurrentUser)
	if err != nil {
		t.Errorf("failed to remove repo: %v", err)
	}
}

func TestSetRelationModels(t *testing.T) {
	cfg := InitTestDB(t)
	pc, err := NewPromptComponent(cfg)
	if err != nil {
		t.Fatalf("failed to create prompt component:   %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := types.RelationModels{
		CurrentUser: tests.CurrentUser,
		Namespace:   tests.TestPromptNamespace,
		Name:        tests.TestPromptName,
		Models:      []string{"wanghh2003/csg-wukong-1B", "wanghh2003/abc", "wanghh2003/gitalym1"},
	}

	err = pc.SetRelationModels(ctx, req)

	if err != nil {
		t.Errorf("failed to set relation models: %v", err)
	}
}
