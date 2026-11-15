package types

// SaveAgentTemplateConfigurationRequest carries the final selected resources.
type SaveAgentTemplateConfigurationRequest struct {
	SourceRevision        string                `json:"source_revision,omitempty"`
	Instructions          *string               `json:"instructions,omitempty"`
	ModelConfig           *AgentFileModelConfig `json:"model_config,omitempty"`
	MCPServerIDs          []string              `json:"mcp_server_ids"`
	SkillIDs              []int64               `json:"skill_ids"`
	KnowledgeBaseIDs      []int64               `json:"knowledge_base_ids"`
	RemovePreservedMCPs   []string              `json:"remove_preserved_mcps,omitempty"`
	RemovePreservedSkills []string              `json:"remove_preserved_skills,omitempty"`
}

type SaveAgentTemplateConfigurationResponse struct {
	RepoPath string                              `json:"repo_path"`
	Status   string                              `json:"status"`
	Warnings []AgentTemplateConfigurationWarning `json:"warnings,omitempty"`
}

type AgentTemplateConfigurationWarning struct {
	Code       string `json:"code"`
	ResourceID string `json:"resource_id"`
	Message    string `json:"message"`
}

type AgentTemplateConfigurationPreview struct {
	SourceRevision   string                `json:"source_revision"`
	Instructions     string                `json:"instructions"`
	ModelConfig      *AgentFileModelConfig `json:"model_config,omitempty"`
	MCPServerIDs     []string              `json:"mcp_server_ids"`
	SkillIDs         []int64               `json:"skill_ids"`
	KnowledgeBaseIDs []int64               `json:"knowledge_base_ids"`
	PreservedMCPs    []string              `json:"preserved_mcps"`
	PreservedSkills  []string              `json:"preserved_skills"`
}
