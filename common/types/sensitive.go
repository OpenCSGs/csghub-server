package types

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

type SensitiveRequestV2 interface {
	GetSensitiveFields() []SensitiveField
}

type SensitiveField struct {
	Name  string
	Value func() string
	// like nickname, chat, comment, etc. See sensitive.Scenario for more details.
	Scenario SensitiveScenario
}

type SensitiveScenario string

// for text
const (
	ScenarioNicknameDetection SensitiveScenario = "nickname_detection"
	ScenarioChatDetection     SensitiveScenario = "chat_detection"
	ScenarioCommentDetection  SensitiveScenario = "comment_detection"
)

// for llm text
const (
	ScenarioLLMQueryModeration SensitiveScenario = "llm_query_moderation"
	ScenarioLLMResModeration   SensitiveScenario = "llm_response_moderation"
)

// for image
const (
	ScenarioImageProfileCheck  SensitiveScenario = "profilePhotoCheck"
	ScenarioImageBaseLineCheck SensitiveScenario = "baselineCheck"
)

func (s SensitiveScenario) FromString(scenario string) (SensitiveScenario, bool) {
	switch scenario {
	case "nickname_detection":
		return ScenarioNicknameDetection, true
	case "chat_detection":
		return ScenarioChatDetection, true
	case "comment_detection":
		return ScenarioCommentDetection, true
	case "profilePhotoCheck":
		return ScenarioImageProfileCheck, true
	case "baselineCheck":
		return ScenarioImageBaseLineCheck, true
	case "llm_response_moderation":
		return ScenarioLLMResModeration, true
	case "llm_query_moderation":
		return ScenarioLLMQueryModeration, true
	default:
		return SensitiveScenario(""), false
	}
}

// MediaType identifies the kind of media referenced by a comment.
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeAudio MediaType = "audio"
	MediaTypeVideo MediaType = "video"
)

// MediaRef identifies one immutable, UUID-named resource in the configured
// public bucket referenced by a comment.
type MediaRef struct {
	URL       string
	Bucket    string
	ObjectKey string
	Type      MediaType
}

// ResourceKey returns a stable SHA-256 hex digest of bucket+objectKey. It is
// the idempotency key for one immutable media resource in media_moderations.
func (r MediaRef) ResourceKey() (string, error) {
	if r.Bucket == "" {
		return "", fmt.Errorf("media resource bucket is required")
	}
	if r.ObjectKey == "" {
		return "", fmt.Errorf("media resource object key is required")
	}
	digest := sha256.New()
	for _, field := range []string{r.Bucket, r.ObjectKey} {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		_, _ = digest.Write(length[:])
		_, _ = digest.Write([]byte(field))
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// MediaModerationRequest describes one audio or video resource to submit to or
// query from the moderation provider. URL is the public-bucket URL; TaskID is
// set only when querying an already-submitted task.
type MediaModerationRequest struct {
	URL    string    `json:"url"`
	Type   MediaType `json:"type"`
	DataID string    `json:"data_id"`
	Seed   string    `json:"seed"`
	TaskID string    `json:"task_id"`
}

// MediaModerationSubmission identifies an asynchronous provider submission.
// TaskID is the provider task handle used to poll the moderation result.
type MediaModerationSubmission struct {
	DataID string `json:"data_id"`
	Seed   string `json:"seed"`
	TaskID string `json:"task_id"`
}

// MediaModerationResult is the polled terminal state of one provider task.
type MediaModerationResult struct {
	DataID string `json:"data_id"`
	Status string `json:"status"` // pass | reject | error | pending
	Reason string `json:"reason"`
}

type MediaModerationDecision string

const (
	MediaModerationDecisionPass    MediaModerationDecision = "pass"
	MediaModerationDecisionPending MediaModerationDecision = "pending"
	MediaModerationDecisionReject  MediaModerationDecision = "reject"
	MediaModerationDecisionError   MediaModerationDecision = "error"
)

// CommentMediaItem describes one audio/video reference that a comment depends
// on, including the provider task handle needed to poll its moderation result.
type CommentMediaItem struct {
	DataID    string    `json:"data_id"`
	TaskID    string    `json:"task_id"`
	MediaType MediaType `json:"media_type"`
}

// CommentMediaDecision is the result of checking a comment's media references.
// Items is populated when Decision is pending so the caller can link the
// comment to the media rows and start a poll workflow.
type CommentMediaDecision struct {
	Decision MediaModerationDecision `json:"decision"`
	Items    []CommentMediaItem      `json:"items"`
}
