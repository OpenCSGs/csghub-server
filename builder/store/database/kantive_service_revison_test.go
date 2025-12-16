package database_test

import (
	"context"
	"testing"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAddRevision(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewKnativeServiceRevisionStoreWithDB(db)
	revision := &database.KnativeServiceRevision{
		SvcName:        "test-svc",
		RevisionName:   "test-revision-1",
		CommitID:       "test-commit-1",
		TrafficPercent: 100,
		IsReady:        false,
		CreateTime:     time.Now(),
	}
	err := store.AddRevision(context.Background(), *revision)
	if err != nil {
		t.Errorf("Add revision failed: %v", err)
	}

	revision = &database.KnativeServiceRevision{
		SvcName:        "test-svc",
		RevisionName:   "test-revision-1",
		CommitID:       "test-commit-1",
		TrafficPercent: 80,
		IsReady:        false,
		CreateTime:     time.Now(),
	}

	revision2 := &database.KnativeServiceRevision{
		SvcName:        "test-svc",
		RevisionName:   "test-revision-1",
		CommitID:       "test-commit-2",
		TrafficPercent: 20,
		IsReady:        false,
		CreateTime:     time.Now(),
	}
	err = store.AddRevision(context.Background(), *revision)
	if err != nil {
		t.Errorf("Add revision failed: %v", err)
	}
	err = store.AddRevision(context.Background(), *revision2)
	if err != nil {
		t.Errorf("Add revision failed: %v", err)
	}
	result, err := store.ListRevisions(context.Background(), "test-svc")
	if err != nil {
		t.Errorf("Get revision failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Get revision failed: expect 2, got %d", len(result))
	}

}

func TestGetRevision(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewKnativeServiceRevisionStoreWithDB(db)
	result, err := store.ListRevisions(context.Background(), "test-svc")
	if err != nil {
		t.Errorf("Get revision failed: %v", err)
	}
	if result != nil {
		t.Errorf("Get revision failed: expect nil, got %v", result)
	}

}
