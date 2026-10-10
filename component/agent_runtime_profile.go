package component

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

const (
	csgclawRuntimeProfileName          = "sandbox_runtime.csgclaw"
	csgclawWorkspaceRuntimeProfileName = "sandbox_runtime.csgclaw-workspace"
	sandboxRuntimeProfileNamePrefix    = "sandbox_runtime."
)

type CSGClawRuntimeProfile struct {
	AgentType      string                       `json:"agent_type"`
	Version        string                       `json:"version"`
	Image          string                       `json:"image"`
	Port           int                          `json:"port"`
	Command        []string                     `json:"command"`
	ReadinessProbe SandboxRuntimeReadinessProbe `json:"readiness_probe"`
	DefaultEnv     map[string]string            `json:"default_env"`
	ContentSHA     string                       `json:"content_sha"`
}

type SandboxRuntimeReadinessProbe = types.SandboxReadinessProbe

func sandboxRuntimeProfileName(agentType string) string {
	return sandboxRuntimeProfileNamePrefix + agentType
}

func initAgentRuntimeProfiles(ctx context.Context) error {
	return syncAgentRuntimeProfiles(ctx, database.NewAgentConfigStore())
}

func syncAgentRuntimeProfiles(ctx context.Context, store database.AgentConfigStore) error {
	dir, err := agentRuntimeProfileDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read agent runtime profile directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if err := syncAgentRuntimeProfile(ctx, store, filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func syncAgentRuntimeProfile(ctx context.Context, store database.AgentConfigStore, path string) error {
	profile, configName, err := parseRuntimeProfileFile(path)
	if err != nil {
		return err
	}
	config := map[string]any{}
	encoded, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(encoded, &config); err != nil {
		return err
	}
	existing, err := store.GetByName(ctx, configName)
	if err != nil {
		return fmt.Errorf("get %s runtime profile: %w", profile.AgentType, err)
	}
	if existing == nil {
		return store.Create(ctx, &database.AgentConfig{Name: configName, Config: config})
	}
	if existing.Config["content_sha"] == profile.ContentSHA {
		return nil
	}
	existing.Config = config
	return store.Update(ctx, existing)
}

func parseRuntimeProfileFile(path string) (*CSGClawRuntimeProfile, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read %s runtime profile: %w", filepath.Base(path), err)
	}
	var profile CSGClawRuntimeProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return nil, "", fmt.Errorf("parse %s runtime profile: %w", filepath.Base(path), err)
	}
	expectedType := strings.TrimSuffix(filepath.Base(path), ".json")
	if err := validateSandboxRuntimeProfile(&profile, expectedType); err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(raw)
	profile.ContentSHA = hex.EncodeToString(hash[:])
	return &profile, sandboxRuntimeProfileName(expectedType), nil
}

func GetCSGClawRuntimeProfile(ctx context.Context, store database.AgentConfigStore) (*CSGClawRuntimeProfile, error) {
	return GetSandboxRuntimeProfile(ctx, store, csgclawRuntimeProfileName)
}

func GetSandboxRuntimeProfile(ctx context.Context, store database.AgentConfigStore, name string) (*CSGClawRuntimeProfile, error) {
	config, err := store.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, fmt.Errorf("%s runtime profile is not initialized", name)
	}
	raw, err := json.Marshal(config.Config)
	if err != nil {
		return nil, err
	}
	var profile CSGClawRuntimeProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return nil, fmt.Errorf("parse stored %s runtime profile: %w", name, err)
	}
	expectedType := strings.TrimPrefix(name, sandboxRuntimeProfileNamePrefix)
	if err := validateSandboxRuntimeProfile(&profile, expectedType); err != nil {
		return nil, err
	}
	return &profile, nil
}

func validateSandboxRuntimeProfile(profile *CSGClawRuntimeProfile, expectedAgentType string) error {
	if profile == nil {
		return fmt.Errorf("invalid %s runtime profile", expectedAgentType)
	}
	if profile.AgentType != expectedAgentType || strings.TrimSpace(profile.Image) == "" || strings.TrimSpace(profile.Version) == "" || profile.Port <= 0 {
		return fmt.Errorf("invalid %s runtime profile", expectedAgentType)
	}
	for _, arg := range profile.Command {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("%s runtime profile command cannot contain empty arguments", expectedAgentType)
		}
	}
	if profile.ReadinessProbe.Protocol == "" && profile.ReadinessProbe.Path == "" {
		return nil
	}
	if profile.ReadinessProbe.Protocol != "http" {
		return fmt.Errorf("%s runtime profile readiness_probe.protocol must be http", expectedAgentType)
	}
	if !strings.HasPrefix(profile.ReadinessProbe.Path, "/") {
		return fmt.Errorf("%s runtime profile readiness_probe.path must start with /", expectedAgentType)
	}
	return nil
}

func agentRuntimeProfileDir() (string, error) {
	path, err := agentRuntimeProfilePath("csgclaw.json")
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

func agentRuntimeProfilePath(name string) (string, error) {
	dir, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(dir, "configs", "agent_runtime", name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("agent runtime profile %s not found", name)
}
