package activity

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/naoina/toml"
)

// eagle3DraftEntry is one registry row: a suggested draft repo for base
// models whose repository path contains model_name (configs/eagle3/models.toml).
type eagle3DraftEntry struct {
	ModelName string `toml:"model_name"`
	DraftRepo string `toml:"draft_repo"`
	Revision  string `toml:"revision"`
}

// eagle3DraftRegistryFile mirrors the [[models]] array of the registry file.
type eagle3DraftRegistryFile struct {
	Models []eagle3DraftEntry `toml:"models"`
}

const eagle3RegistryRelPath = "configs/eagle3/models.toml"

var (
	eagle3DraftOnce    sync.Once
	eagle3DraftEntries []eagle3DraftEntry
)

// lookupEagle3DraftRepo resolves a draft repo suggestion for a deploy whose
// user left spec-draft-model empty. A nil result means "no suggestion" and
// the deploy keeps failing eagle3 composition as before (fail-closed). The
// file is read once per process; operators update it by shipping new config.
func lookupEagle3DraftRepo(repoPath string) *eagle3DraftEntry {
	eagle3DraftOnce.Do(func() {
		eagle3DraftEntries = loadEagle3DraftRegistry(eagle3RegistryFilePath())
	})
	return matchEagle3DraftEntry(eagle3DraftEntries, repoPath)
}

// eagle3RegistryFilePath resolves the registry file against the process
// working directory (same resolution as the PD recommendation loader).
func eagle3RegistryFilePath() string {
	currentDir, err := filepath.Abs(filepath.Dir("."))
	if err != nil {
		return eagle3RegistryRelPath
	}
	return eagle3RegistryFilePathIn(currentDir)
}

// eagle3RegistryFilePathIn joins the registry file onto baseDir, undoing the
// cmd/csghub-server prefix used by go run.
func eagle3RegistryFilePathIn(baseDir string) string {
	baseDir = strings.TrimSuffix(baseDir, "/cmd/csghub-server")
	return filepath.Join(baseDir, eagle3RegistryRelPath)
}

// loadEagle3DraftRegistry reads the registry file. Any problem (missing file,
// invalid TOML, incomplete entry) only logs and yields fewer or no entries:
// speculative decoding must never break because a suggestion file is stale.
func loadEagle3DraftRegistry(path string) []eagle3DraftEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// The file ships in configs/, so a missing file in production
			// means a broken deployment layout: log it instead of failing
			// silently (at most once per process via sync.Once).
			slog.Warn("eagle3 draft registry file not found, ignoring",
				slog.String("path", path))
		} else {
			slog.Warn("failed to read eagle3 draft registry, ignoring",
				slog.String("path", path), slog.Any("error", err))
		}
		return nil
	}

	// Log unknown fields instead of silently ignoring them to catch typos in
	// config while keeping forward compatibility (same as the PD loader).
	config := toml.DefaultConfig
	config.MissingField = func(typ reflect.Type, key string) error {
		slog.Warn("unknown field in eagle3 draft registry, ignoring",
			slog.String("field", key),
			slog.String("type", typ.String()),
			slog.String("file", path))
		return nil
	}

	var file eagle3DraftRegistryFile
	if err := config.NewDecoder(bytes.NewReader(data)).Decode(&file); err != nil {
		slog.Warn("failed to parse eagle3 draft registry, ignoring",
			slog.String("path", path), slog.Any("error", err))
		return nil
	}

	entries := make([]eagle3DraftEntry, 0, len(file.Models))
	for _, entry := range file.Models {
		if entry.ModelName == "" || entry.DraftRepo == "" {
			slog.Warn("eagle3 draft registry entry missing model_name or draft_repo, skipping",
				slog.String("model_name", entry.ModelName), slog.String("draft_repo", entry.DraftRepo))
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// matchEagle3DraftEntry returns the most specific entry whose model_name is a
// case-insensitive substring of the repository path, or nil. Longest
// model_name wins so "Llama-3.1-8B-Instruct" beats a generic "Llama-3.1".
func matchEagle3DraftEntry(entries []eagle3DraftEntry, repoPath string) *eagle3DraftEntry {
	repoPath = strings.ToLower(repoPath)
	var best *eagle3DraftEntry
	for i := range entries {
		entry := &entries[i]
		if !strings.Contains(repoPath, strings.ToLower(entry.ModelName)) {
			continue
		}
		if best == nil || len(entry.ModelName) > len(best.ModelName) {
			best = entry
		}
	}
	return best
}
