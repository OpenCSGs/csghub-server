package memmachine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/types"
)

type memmachineSearchRequest struct {
	OrgID          string             `json:"org_id,omitempty"`
	ProjectID      string             `json:"project_id,omitempty"`
	Query          string             `json:"query,omitempty"`
	TopK           int                `json:"top_k,omitempty"`
	Types          []types.MemoryType `json:"types,omitempty"`
	ScoreThreshold *float64           `json:"score_threshold,omitempty"`
	PageSize       int                `json:"page_size,omitempty"`
	PageNum        int                `json:"page_num,omitempty"`
	Filter         string             `json:"filter,omitempty"`
}

type memmachineListByUIDRequest struct {
	OrgID     string `json:"org_id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Type      string `json:"type,omitempty"`
	Filter    string `json:"filter,omitempty"`
}

type memmachineAddRequest struct {
	OrgID     string              `json:"org_id,omitempty"`
	ProjectID string              `json:"project_id,omitempty"`
	Types     []types.MemoryType  `json:"types,omitempty"`
	Messages  []memmachineMessage `json:"messages"`
}

type memmachineMessage struct {
	Content      string         `json:"content"`
	ProducerRole string         `json:"producer_role,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type memmachineDeleteEpisodicRequest struct {
	OrgID       string   `json:"org_id,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
	EpisodicID  string   `json:"episodic_id,omitempty"`
	EpisodicIDs []string `json:"episodic_ids,omitempty"`
}

type memmachineDeleteSemanticRequest struct {
	OrgID       string   `json:"org_id,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
	SemanticID  string   `json:"semantic_id,omitempty"`
	SemanticIDs []string `json:"semantic_ids,omitempty"`
}

type memmachineSearchResponse struct {
	Status  int                     `json:"status,omitempty"`
	Content memmachineSearchContent `json:"content"`
}

type memmachineSearchContent struct {
	EpisodicMemory json.RawMessage      `json:"episodic_memory,omitempty"`
	SemanticMemory []memmachineSemantic `json:"semantic_memory,omitempty"`
}

type memmachineEpisodic struct {
	UID          string         `json:"uid"`
	Content      string         `json:"content"`
	CreatedAt    string         `json:"created_at,omitempty"`
	ProducerRole string         `json:"producer_role,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Similarity   *float64       `json:"similarity,omitempty"`
}

type memmachineSemantic struct {
	Category    string         `json:"category,omitempty"`
	FeatureName string         `json:"feature_name,omitempty"`
	Tag         string         `json:"tag,omitempty"`
	Value       string         `json:"value,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Similarity  *float64       `json:"similarity,omitempty"`
}

func mapSearchRequest(req *types.SearchMemoriesRequest) *memmachineSearchRequest {
	if req == nil {
		return nil
	}
	orgID, projectID := resolveQueryOrgProject(req.OrgID, req.ProjectID)
	mapped := &memmachineSearchRequest{
		OrgID:     orgID,
		ProjectID: projectID,
		Query:     req.ContentQuery,
		TopK:      req.TopK,
		Types:     req.Types,
		PageSize:  req.PageSize,
		PageNum:   req.PageNum,
		Filter:    req.Filter,
	}
	if req.MinSimilarity != nil {
		mapped.ScoreThreshold = req.MinSimilarity
	}
	return mapped
}

func mapListRequest(req *types.ListMemoriesRequest) *memmachineSearchRequest {
	if req == nil {
		return nil
	}
	orgID, projectID := resolveQueryOrgProject(req.OrgID, req.ProjectID)
	return &memmachineSearchRequest{
		OrgID:     orgID,
		ProjectID: projectID,
		Types:     req.Types,
		PageSize:  req.PageSize,
		PageNum:   req.PageNum,
	}
}

func mapAddRequest(req *types.AddMemoriesRequest) *memmachineAddRequest {
	if req == nil {
		return nil
	}
	orgID, projectID := resolveOrgProject(req)

	messages := make([]memmachineMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, memmachineMessage{
			Content:      msg.Content,
			ProducerRole: msg.Role,
			Metadata:     buildMemmachineMetadata(msg, req),
		})
	}
	return &memmachineAddRequest{
		OrgID:     orgID,
		ProjectID: projectID,
		Types:     req.Types,
		Messages:  messages,
	}
}

func resolveOrgProject(req *types.AddMemoriesRequest) (string, string) {
	orgID := ""
	projectID := ""
	if req != nil {
		orgID = req.OrgID
		projectID = req.ProjectID
	}
	if orgID == "" {
		orgID = "_global"
	}
	if projectID == "" {
		projectID = "_public"
	}
	return orgID, projectID
}

func resolveQueryOrgProject(orgID, projectID string) (string, string) {
	if orgID == "" && projectID == "" {
		return "_global", "_public"
	}
	return orgID, projectID
}

func mapSearchResponse(resp memmachineSearchResponse, req *types.SearchMemoriesRequest) *types.SearchMemoriesResponse {
	return &types.SearchMemoriesResponse{
		Status:  resp.Status,
		Content: mapMemmachineContent(resp.Content, req),
	}
}

func mapListResponse(resp memmachineSearchResponse, req *types.ListMemoriesRequest) *types.ListMemoriesResponse {
	return &types.ListMemoriesResponse{
		Status:  resp.Status,
		Content: mapMemmachineContent(resp.Content, req),
	}
}

func mapMemmachineContent(content memmachineSearchContent, req any) []types.MemoryMessage {
	var messages []types.MemoryMessage
	baseScopes, baseUserID := deriveBaseScopes(req)
	if baseScopes != nil {
		if baseScopes.OrgID == "" && baseScopes.ProjectID == "" {
			baseScopes.OrgID = "_global"
			baseScopes.ProjectID = "_public"
		}
	}

	for _, item := range parseEpisodicMemory(content.EpisodicMemory) {
		msg := types.MemoryMessage{
			UID:        "e_" + item.UID,
			Content:    item.Content,
			Role:       item.ProducerRole,
			Scopes:     baseScopes,
			UserID:     baseUserID,
			Similarity: item.Similarity,
		}
		if item.CreatedAt != "" {
			if ts, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil {
				msg.Timestamp = ts
			}
		}
		msg.MetaData, msg.UserID, msg.Scopes = splitMetadata(item.Metadata, msg.UserID, msg.Scopes)
		messages = append(messages, msg)
	}

	for _, item := range content.SemanticMemory {
		meta := map[string]any{}
		if item.Category != "" {
			meta["category"] = item.Category
		}
		if item.Tag != "" {
			meta["tag"] = item.Tag
		}
		if item.FeatureName != "" {
			meta["feature_name"] = item.FeatureName
		}
		for k, v := range item.Metadata {
			if k == "id" {
				continue
			}
			meta[k] = v
		}
		uid := ""
		if rawID, ok := item.Metadata["id"]; ok {
			uid = "s_" + toString(rawID)
		}
		msg := types.MemoryMessage{
			UID:        uid,
			Content:    item.Value,
			Scopes:     baseScopes,
			UserID:     baseUserID,
			MetaData:   meta,
			Similarity: item.Similarity,
		}
		msg.MetaData, msg.UserID, msg.Scopes = splitMetadata(msg.MetaData, msg.UserID, msg.Scopes)
		messages = append(messages, msg)
	}

	return messages
}

type memmachineEpisodeGroup struct {
	Episodes []memmachineEpisodic `json:"episodes,omitempty"`
}

type memmachineEpisodicContainer struct {
	LongTermMemory  memmachineEpisodeGroup `json:"long_term_memory,omitempty"`
	ShortTermMemory memmachineEpisodeGroup `json:"short_term_memory,omitempty"`
	Episodes        []memmachineEpisodic   `json:"episodes,omitempty"`
}

func parseEpisodicMemory(raw json.RawMessage) []memmachineEpisodic {
	if len(raw) == 0 {
		return nil
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	var episodes []memmachineEpisodic
	if raw[0] == '[' {
		if err := json.Unmarshal(raw, &episodes); err == nil {
			return dedupeEpisodes(episodes)
		}
		return nil
	}
	var container memmachineEpisodicContainer
	if err := json.Unmarshal(raw, &container); err != nil {
		return nil
	}
	episodes = append(episodes, container.Episodes...)
	episodes = append(episodes, container.LongTermMemory.Episodes...)
	episodes = append(episodes, container.ShortTermMemory.Episodes...)
	return dedupeEpisodes(episodes)
}

func dedupeEpisodes(items []memmachineEpisodic) []memmachineEpisodic {
	seen := map[string]struct{}{}
	result := make([]memmachineEpisodic, 0, len(items))
	for _, item := range items {
		key := item.UID
		if key == "" {
			result = append(result, item)
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func deriveBaseScopes(req any) (*types.MemoryMessageScopes, string) {
	scopes := &types.MemoryMessageScopes{}
	userID := ""
	switch v := req.(type) {
	case *types.SearchMemoriesRequest:
		scopes.AgentID = v.AgentID
		scopes.OrgID, scopes.ProjectID = resolveQueryOrgProject(v.OrgID, v.ProjectID)
		scopes.SessionID = v.SessionID
		userID = v.UserID
	case *types.ListMemoriesRequest:
		scopes.AgentID = v.AgentID
		scopes.OrgID, scopes.ProjectID = resolveQueryOrgProject(v.OrgID, v.ProjectID)
		scopes.SessionID = v.SessionID
		userID = v.UserID
	default:
		return nil, ""
	}
	if scopes.AgentID == "" && scopes.OrgID == "" && scopes.ProjectID == "" && scopes.SessionID == "" {
		scopes = nil
	}
	return scopes, userID
}

func splitMetadata(metadata map[string]any, userID string, scopes *types.MemoryMessageScopes) (map[string]any, string, *types.MemoryMessageScopes) {
	if metadata == nil {
		return nil, userID, scopes
	}
	meta := map[string]any{}
	for k, v := range metadata {
		key := strings.ToLower(k)
		switch key {
		case "user_id":
			userID = toString(v)
		case "agent_id":
			if scopes == nil {
				scopes = &types.MemoryMessageScopes{}
			}
			scopes.AgentID = toString(v)
		case "org_id":
			if scopes == nil {
				scopes = &types.MemoryMessageScopes{}
			}
			scopes.OrgID = toString(v)
		case "project_id":
			if scopes == nil {
				scopes = &types.MemoryMessageScopes{}
			}
			scopes.ProjectID = toString(v)
		case "session_id":
			if scopes == nil {
				scopes = &types.MemoryMessageScopes{}
			}
			scopes.SessionID = toString(v)
		default:
			meta[k] = v
		}
	}
	if len(meta) == 0 {
		meta = nil
	}
	return meta, userID, scopes
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", v), "0"), ".")
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	default:
		return ""
	}
}

func buildMemmachineMetadata(msg types.MemoryMessage, req *types.AddMemoriesRequest) map[string]any {
	if msg.MetaData == nil && msg.UserID == "" && req == nil {
		return nil
	}
	meta := map[string]any{}
	for k, v := range msg.MetaData {
		meta[k] = v
	}
	if msg.UserID != "" {
		meta["user_id"] = msg.UserID
	}
	if req != nil {
		if req.AgentID != "" {
			meta["agent_id"] = req.AgentID
		}
		if req.SessionID != "" {
			meta["session_id"] = req.SessionID
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}
