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
)

const csgclawRuntimeProfileName = "sandbox_runtime.csgclaw"

type CSGClawRuntimeProfile struct {
	AgentType   string                    `json:"agent_type"`
	Version     string                    `json:"version"`
	Image       string                    `json:"image"`
	Port        int                       `json:"port"`
	Command     []string                  `json:"command"`
	HealthCheck SandboxRuntimeHealthCheck `json:"health_check"`
	DefaultEnv  map[string]string         `json:"default_env"`
	ContentSHA  string                    `json:"content_sha"`
}

type SandboxRuntimeHealthCheck struct {
	Protocol string `json:"protocol"`
	Path     string `json:"path"`
}

func initAgentRuntimeProfiles(ctx context.Context) error {
	path, err := agentRuntimeProfilePath("csgclaw.json")
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read csgclaw runtime profile: %w", err)
	}
	var profile CSGClawRuntimeProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return fmt.Errorf("parse csgclaw runtime profile: %w", err)
	}
	if err := validateCSGClawRuntimeProfile(&profile); err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	profile.ContentSHA = hex.EncodeToString(hash[:])

	config := map[string]any{}
	encoded, _ := json.Marshal(profile)
	if err := json.Unmarshal(encoded, &config); err != nil {
		return err
	}
	store := database.NewAgentConfigStore()
	existing, err := store.GetByName(ctx, csgclawRuntimeProfileName)
	if err != nil {
		return fmt.Errorf("get csgclaw runtime profile: %w", err)
	}
	if existing == nil {
		return store.Create(ctx, &database.AgentConfig{Name: csgclawRuntimeProfileName, Config: config})
	}
	if existing.Config["content_sha"] == profile.ContentSHA {
		return nil
	}
	existing.Config = config
	return store.Update(ctx, existing)
}

func GetCSGClawRuntimeProfile(ctx context.Context, store database.AgentConfigStore) (*CSGClawRuntimeProfile, error) {
	config, err := store.GetByName(ctx, csgclawRuntimeProfileName)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, fmt.Errorf("csgclaw runtime profile is not initialized")
	}
	raw, err := json.Marshal(config.Config)
	if err != nil {
		return nil, err
	}
	var profile CSGClawRuntimeProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return nil, fmt.Errorf("parse stored csgclaw runtime profile: %w", err)
	}
	if err := validateCSGClawRuntimeProfile(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func validateCSGClawRuntimeProfile(profile *CSGClawRuntimeProfile) error {
	if profile.AgentType != "csgclaw" || strings.TrimSpace(profile.Image) == "" || strings.TrimSpace(profile.Version) == "" || profile.Port <= 0 {
		return fmt.Errorf("invalid csgclaw runtime profile")
	}
	for _, arg := range profile.Command {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("csgclaw runtime profile command cannot contain empty arguments")
		}
	}
	if profile.HealthCheck.Protocol == "" && profile.HealthCheck.Path == "" {
		return nil
	}
	if profile.HealthCheck.Protocol != "http" {
		return fmt.Errorf("csgclaw runtime profile health_check.protocol must be http")
	}
	if !strings.HasPrefix(profile.HealthCheck.Path, "/") {
		return fmt.Errorf("csgclaw runtime profile health_check.path must start with /")
	}
	return nil
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
