//go:build ee || saas

package activity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEagle3RegistryFilePathIn(t *testing.T) {
	t.Run("production layout joins the file onto the binary dir", func(t *testing.T) {
		require.Equal(t,
			filepath.Join("/starhub-bin", eagle3RegistryRelPath),
			eagle3RegistryFilePathIn("/starhub-bin"))
	})

	t.Run("go run layout strips the cmd/csghub-server prefix", func(t *testing.T) {
		require.Equal(t,
			filepath.Join("/repo", eagle3RegistryRelPath),
			eagle3RegistryFilePathIn("/repo/cmd/csghub-server"))
	})
}

func TestLoadEagle3DraftRegistry(t *testing.T) {
	t.Run("missing file yields no entries", func(t *testing.T) {
		entries := loadEagle3DraftRegistry(filepath.Join(t.TempDir(), "absent.toml"))
		require.Empty(t, entries)
	})

	t.Run("invalid toml yields no entries", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "models.toml")
		require.NoError(t, os.WriteFile(path, []byte("[[models"), 0o644))
		require.Empty(t, loadEagle3DraftRegistry(path))
	})

	t.Run("incomplete entries are skipped", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "models.toml")
		content := `
[[models]]
model_name = "Llama-3.1-8B-Instruct"
draft_repo = "opencsg/EAGLE3-Llama-3.1-8B-Instruct"
revision = "main"

[[models]]
model_name = ""
draft_repo = "opencsg/orphan"

[[models]]
model_name = "Qwen3-8B"
draft_repo = ""
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		entries := loadEagle3DraftRegistry(path)
		require.Len(t, entries, 1)
		require.Equal(t, eagle3DraftEntry{
			ModelName: "Llama-3.1-8B-Instruct",
			DraftRepo: "opencsg/EAGLE3-Llama-3.1-8B-Instruct",
			Revision:  "main",
		}, entries[0])
	})
}

func TestMatchEagle3DraftEntry(t *testing.T) {
	entries := []eagle3DraftEntry{
		{ModelName: "Llama-3.1", DraftRepo: "opencsg/generic"},
		{ModelName: "Llama-3.1-8B-Instruct", DraftRepo: "opencsg/specific"},
		{ModelName: "Qwen3-8B", DraftRepo: "opencsg/qwen3-8b"},
	}

	t.Run("no match returns nil", func(t *testing.T) {
		require.Nil(t, matchEagle3DraftEntry(entries, "opencsg/DeepSeek-R1"))
	})

	t.Run("case-insensitive substring match", func(t *testing.T) {
		entry := matchEagle3DraftEntry(entries, "opencsg/qwen3-8b-instruct")
		require.NotNil(t, entry)
		require.Equal(t, "opencsg/qwen3-8b", entry.DraftRepo)
	})

	t.Run("longest model_name wins", func(t *testing.T) {
		entry := matchEagle3DraftEntry(entries, "opencsg/Llama-3.1-8B-Instruct")
		require.NotNil(t, entry)
		require.Equal(t, "opencsg/specific", entry.DraftRepo)
	})

	t.Run("generic entry matches when specific key is absent", func(t *testing.T) {
		entry := matchEagle3DraftEntry(entries, "meta-llama/Llama-3.1-70B-Instruct")
		require.NotNil(t, entry)
		require.Equal(t, "opencsg/generic", entry.DraftRepo)
	})
}

func TestLookupEagle3DraftRepoFailClosed(t *testing.T) {
	// The registry file lives in the repo, but the path resolved from the
	// test process working directory does not exist, so the lookup must
	// degrade to nil instead of erroring or suggesting a repo.
	require.Nil(t, lookupEagle3DraftRepo("opencsg/Llama-3.1-8B-Instruct"))
}
