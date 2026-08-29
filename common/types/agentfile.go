package types

import (
	"fmt"

	"github.com/naoina/toml"
)

const AgentFileName = "agent.toml"

// AgentFileEnv describes one [[image.env]] entry in agentfile/v1.
type AgentFileEnv struct {
	Name        string `toml:"name" json:"name"`
	Required    bool   `toml:"required" json:"required"`
	Secret      bool   `toml:"secret" json:"secret"`
	Default     string `toml:"default" json:"default"`
	Description string `toml:"description" json:"description"`
}

// AgentFileImage is the [image] section of agentfile/v1.
type AgentFileImage struct {
	Ref string         `toml:"ref" json:"ref"`
	Env []AgentFileEnv `toml:"env" json:"env"`
}

// AgentFile is the schema_version "agentfile/v1" manifest committed to code repos.
type AgentFile struct {
	Name          string         `toml:"name" json:"name"`
	Role          string         `toml:"role" json:"role"`
	Description   string         `toml:"description" json:"description"`
	RuntimeKind   string         `toml:"runtime_kind" json:"runtime_kind"`
	UpdatedAt     string         `toml:"updated_at" json:"updated_at"`
	Version       string         `toml:"version" json:"version"`
	SchemaVersion string         `toml:"schema_version" json:"schema_version"`
	Tags          []string       `toml:"tags" json:"tags"`
	Image         AgentFileImage `toml:"image" json:"image"`
}

// SupportedSandboxRuntimeKinds lists runtime_kind values that can be auto-deployed
// as sandboxes from code repo pushes. Extend when new runtimes are supported.
var SupportedSandboxRuntimeKinds = map[string]bool{
	"codex": true,
}

func ParseAgentFile(content string) (*AgentFile, error) {
	var agentFile AgentFile
	if err := toml.Unmarshal([]byte(content), &agentFile); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", AgentFileName, err)
	}
	if agentFile.Name == "" {
		return nil, fmt.Errorf("%s missing required field: name", AgentFileName)
	}
	return &agentFile, nil
}
