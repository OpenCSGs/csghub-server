package common

import (
	"testing"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestAddPrefixBySourceID(t *testing.T) {
	s := &database.SyncVersion{
		Version:        1,
		SourceID:       0,
		RepoPath:       "test/test",
		RepoType:       types.ModelRepo,
		LastModifiedAt: time.Now(),
		ChangeLog:      "test log",
	}
	str := AddPrefixBySourceID(s.SourceID, "test")
	if str != "OpenCSG_test" {
		t.Errorf("Expected str should be 'OpenCSG_test' but got %s", str)
	}

	s1 := &database.SyncVersion{
		Version:        1,
		SourceID:       1,
		RepoPath:       "test/test",
		RepoType:       types.ModelRepo,
		LastModifiedAt: time.Now(),
		ChangeLog:      "test log",
	}
	str1 := AddPrefixBySourceID(s1.SourceID, "test")
	if str1 != "Huggingface_test" {
		t.Errorf("Expected str should be 'Huggingface_test' but got %s", str)
	}
}

func TestTrimPrefixCloneURLBySourceID(t *testing.T) {
	s := &database.SyncVersion{
		Version:        1,
		SourceID:       0,
		RepoPath:       "test/test",
		RepoType:       types.ModelRepo,
		LastModifiedAt: time.Now(),
		ChangeLog:      "test log",
	}
	cloneURL := TrimPrefixCloneURLBySourceID(
		"https://opencsg.com",
		"model",
		"OpenCSG_namespace",
		"name",
		s.SourceID,
	)
	if cloneURL != "https://opencsg.com/models/namespace/name.git" {
		t.Errorf("Expected cloneURL should be 'https://opencsg.com/models/namespace/name' but got %s", cloneURL)
	}

	s1 := &database.SyncVersion{
		Version:        1,
		SourceID:       1,
		RepoPath:       "test/test",
		RepoType:       types.ModelRepo,
		LastModifiedAt: time.Now(),
		ChangeLog:      "test log",
	}
	cloneURL1 := TrimPrefixCloneURLBySourceID(
		"https://opencsg.com",
		"model",
		"Huggingface_namespace",
		"name",
		s1.SourceID,
	)
	if cloneURL1 != "https://opencsg.com/models/namespace/name.git" {
		t.Errorf("Expected cloneURL should be 'https://opencsg.com/models/namespace/name' but got %s", cloneURL1)
	}
}
