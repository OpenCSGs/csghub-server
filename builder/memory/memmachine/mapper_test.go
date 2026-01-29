package memmachine

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/types"
)

func TestMapSearchRequestMinSimilarity(t *testing.T) {
	minSim := 0.12
	req := &types.SearchMemoriesRequest{
		OrgID:         "org",
		ProjectID:     "proj",
		ContentQuery:  "football",
		MinSimilarity: &minSim,
		Types:         []types.MemoryType{types.MemoryTypeEpisodic},
	}
	mapped := mapSearchRequest(req)
	if assert.NotNil(t, mapped) {
		assert.Equal(t, "org", mapped.OrgID)
		assert.Equal(t, "proj", mapped.ProjectID)
		assert.Equal(t, "football", mapped.Query)
		if assert.NotNil(t, mapped.ScoreThreshold) {
			assert.InDelta(t, 0.12, *mapped.ScoreThreshold, 0.0001)
		}
	}
}

func TestMapSearchResponseMapsEpisodic(t *testing.T) {
	req := &types.SearchMemoriesRequest{
		AgentID: "agent",
		UserID:  "u_req",
	}
	container := memmachineEpisodicContainer{
		LongTermMemory: memmachineEpisodeGroup{
			Episodes: []memmachineEpisodic{
				{
					UID:          "45",
					Content:      "hello",
					CreatedAt:    "2025-01-02T03:04:05Z",
					ProducerRole: "user",
					Similarity:   floatPtr(0.42),
					Metadata: map[string]any{
						"user_id":    "u_meta",
						"session_id": "s_meta",
						"foo":        "bar",
					},
				},
			},
		},
	}
	rawBytes, err := json.Marshal(container)
	assert.NoError(t, err)
	raw := memmachineSearchResponse{
		Content: memmachineSearchContent{
			EpisodicMemory: rawBytes,
		},
	}
	resp := mapSearchResponse(raw, req)
	if assert.Len(t, resp.Content, 1) {
		msg := resp.Content[0]
		assert.Equal(t, "e_45", msg.UID)
		assert.Equal(t, "hello", msg.Content)
		assert.Equal(t, "user", msg.Role)
		assert.Equal(t, "u_meta", msg.UserID)
		if assert.NotNil(t, msg.Scopes) {
			assert.Equal(t, "agent", msg.Scopes.AgentID)
			assert.Equal(t, "_global", msg.Scopes.OrgID)
			assert.Equal(t, "_public", msg.Scopes.ProjectID)
			assert.Equal(t, "s_meta", msg.Scopes.SessionID)
		}
		assert.Equal(t, map[string]any{"foo": "bar"}, msg.MetaData)
		if assert.NotNil(t, msg.Similarity) {
			assert.InDelta(t, 0.42, *msg.Similarity, 0.0001)
		}
		assert.True(t, msg.Timestamp.Equal(time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)))
	}
}

func TestMapSearchResponseMapsSemantic(t *testing.T) {
	req := &types.SearchMemoriesRequest{OrgID: "org"}
	raw := memmachineSearchResponse{
		Content: memmachineSearchContent{
			SemanticMemory: []memmachineSemantic{
				{
					Category:    "profile",
					FeatureName: "watched_sports",
					Tag:         "Hobbies",
					Value:       "User watches soccer.",
					Metadata: map[string]any{
						"id":  "20",
						"foo": "bar",
					},
				},
			},
		},
	}
	resp := mapSearchResponse(raw, req)
	if assert.Len(t, resp.Content, 1) {
		msg := resp.Content[0]
		assert.Equal(t, "s_20", msg.UID)
		assert.Equal(t, "User watches soccer.", msg.Content)
		assert.Equal(t, map[string]any{
			"category":     "profile",
			"feature_name": "watched_sports",
			"tag":          "Hobbies",
			"foo":          "bar",
		}, msg.MetaData)
	}
}

func TestMapAddRequestBuildsMetadata(t *testing.T) {
	req := &types.AddMemoriesRequest{
		AgentID:   "agent",
		SessionID: "sess",
		OrgID:     "org",
		ProjectID: "proj",
		Types:     []types.MemoryType{types.MemoryTypeEpisodic},
		Messages: []types.MemoryMessage{
			{
				Content:  "hi",
				Role:     "user",
				UserID:   "u1",
				MetaData: map[string]any{"k": "v"},
			},
		},
	}
	mapped := mapAddRequest(req)
	if assert.NotNil(t, mapped) {
		assert.Equal(t, "org", mapped.OrgID)
		assert.Equal(t, "proj", mapped.ProjectID)
		if assert.Len(t, mapped.Messages, 1) {
			msg := mapped.Messages[0]
			assert.Equal(t, "hi", msg.Content)
			assert.Equal(t, "user", msg.ProducerRole)
			assert.Equal(t, map[string]any{
				"k":          "v",
				"user_id":    "u1",
				"agent_id":   "agent",
				"session_id": "sess",
			}, msg.Metadata)
		}
	}
}

func floatPtr(val float64) *float64 {
	return &val
}

func TestMapAddRequestAddsAgentSessionMetadata(t *testing.T) {
	req := &types.AddMemoriesRequest{
		AgentID:   "agent",
		SessionID: "sess",
		OrgID:     "org",
		ProjectID: "proj",
		Messages: []types.MemoryMessage{
			{Content: "hello"},
		},
	}
	mapped := mapAddRequest(req)
	if assert.NotNil(t, mapped) {
		if assert.Len(t, mapped.Messages, 1) {
			assert.Equal(t, map[string]any{
				"agent_id":   "agent",
				"session_id": "sess",
			}, mapped.Messages[0].Metadata)
		}
	}
}
