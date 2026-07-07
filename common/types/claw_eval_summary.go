package types

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	ClawEvalTasksNormal        = "normal"
	ClawEvalDefaultJudgeModel  = "qwen3.7-max"
)

// ClawEvalSummary mirrors claw-eval batch_summary.json.
// PassHatK and PassAtK serialize as pass_hat_{trials_per_task} and pass_at_{trials_per_task}
// to stay compatible with the upstream evaluator output format.
type ClawEvalSummary struct {
	Tasks         int     `json:"tasks"`
	TrialsPerTask int     `json:"trials_per_task"`
	Errored       int     `json:"errored"`
	AvgScore      float64 `json:"avg_score"`
	PassHatK      int     `json:"-"`
	PassAtK       int     `json:"-"`
}

func (s ClawEvalSummary) MarshalJSON() ([]byte, error) {
	payload := map[string]any{
		"tasks":           s.Tasks,
		"trials_per_task": s.TrialsPerTask,
		"errored":         s.Errored,
		"avg_score":       s.AvgScore,
	}
	k := s.TrialsPerTask
	if k <= 0 {
		k = 1
	}
	payload[fmt.Sprintf("pass_hat_%d", k)] = s.PassHatK
	payload[fmt.Sprintf("pass_at_%d", k)] = s.PassAtK
	return json.Marshal(payload)
}

func ParseClawEvalSummary(raw []byte) (*ClawEvalSummary, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid claw-eval summary json: %w", err)
	}

	summary := &ClawEvalSummary{}
	if err := decodeIntField(payload, "tasks", &summary.Tasks); err != nil {
		return nil, err
	}
	if err := decodeIntField(payload, "trials_per_task", &summary.TrialsPerTask); err != nil {
		return nil, err
	}
	if err := decodeIntField(payload, "errored", &summary.Errored); err != nil {
		return nil, err
	}
	if err := decodeFloatField(payload, "avg_score", &summary.AvgScore); err != nil {
		return nil, err
	}

	k := summary.TrialsPerTask
	if k <= 0 {
		k = 1
	}
	if err := decodeIntField(payload, fmt.Sprintf("pass_hat_%d", k), &summary.PassHatK); err != nil {
		return nil, err
	}
	if err := decodeIntField(payload, fmt.Sprintf("pass_at_%d", k), &summary.PassAtK); err != nil {
		return nil, err
	}
	return summary, nil
}

func decodeIntField(payload map[string]json.RawMessage, key string, target *int) error {
	raw, ok := payload[key]
	if !ok {
		return fmt.Errorf("claw-eval summary missing field %q", key)
	}
	if err := json.Unmarshal(raw, target); err == nil {
		return nil
	}
	var asFloat float64
	if err := json.Unmarshal(raw, &asFloat); err != nil {
		return fmt.Errorf("claw-eval summary field %q is not an integer: %w", key, err)
	}
	*target = int(asFloat)
	return nil
}

func decodeFloatField(payload map[string]json.RawMessage, key string, target *float64) error {
	raw, ok := payload[key]
	if !ok {
		return fmt.Errorf("claw-eval summary missing field %q", key)
	}
	if err := json.Unmarshal(raw, target); err == nil {
		return nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err != nil {
		return fmt.Errorf("claw-eval summary field %q is not a number: %w", key, err)
	}
	value, err := strconv.ParseFloat(asString, 64)
	if err != nil {
		return fmt.Errorf("claw-eval summary field %q is not a number: %w", key, err)
	}
	*target = value
	return nil
}
