package database

import (
	"context"
	"testing"
	"time"

	"opencsg.com/csghub-server/common/config"
)

var (
	TestRepoId    int64   = 40
	TestRepoIds   []int64 = []int64{40}
	TestDSN       string  = "postgresql://postgres:postgres@localhost:5433/starhub_server?sslmode=disable"
	TestNamespace string  = "wanghh2003"
	TestName      string  = "gitalyds2"
)

func InitTestDB(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	cfg.Database.DSN = TestDSN
	dbConfig := DBConfig{
		Dialect: DatabaseDialect(cfg.Database.Driver),
		DSN:     cfg.Database.DSN,
	}
	InitDB(dbConfig)
}

func TestCreate(t *testing.T) {
	InitTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := NewPromptStore()

	p := Prompt{
		RepositoryID: TestRepoId,
	}

	res, err := ps.Create(ctx, p)

	if err != nil {
		t.Fatalf("failed to create prompt: %v", err)
	}
	t.Logf("created prompt: %d", res.ID)
}

func TestUpdate(t *testing.T) {
	InitTestDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := NewPromptStore()

	p, err := ps.ByRepoID(ctx, TestRepoId)

	ps.Update(ctx, *p)

	if err != nil {
		t.Fatalf("failed to update prompt: %v", err)
	}
}

func TestByRepoID(t *testing.T) {
	InitTestDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := NewPromptStore()

	_, err := ps.ByRepoID(ctx, TestRepoId)

	if err != nil {
		t.Fatalf("failed to get prompt by repo id: %v", err)
	}
}

func TestByRepoIDs(t *testing.T) {
	InitTestDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := NewPromptStore()

	_, err := ps.ByRepoIDs(ctx, TestRepoIds)

	if err != nil {
		t.Fatalf("failed to get prompt by repo ids: %v", err)
	}
}

func TestFindByPath(t *testing.T) {
	InitTestDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := NewPromptStore()

	_, err := ps.FindByPath(ctx, TestNamespace, TestName)

	if err != nil {
		t.Fatalf("failed to find prompt by repo path: %v", err)
	}
}

func TestDelete(t *testing.T) {
	InitTestDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := NewPromptStore()

	p, err := ps.ByRepoID(ctx, TestRepoId)

	ps.Delete(ctx, *p)

	if err != nil {
		t.Fatalf("failed to delete prompt: %v", err)
	}
}
