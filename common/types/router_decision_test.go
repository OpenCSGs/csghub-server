package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRouterDecisionClone(t *testing.T) {
	valid := RouterDecision{Version: "simple-exploration-decision/1", DecisionID: "decision", Selected: true, TargetModelID: "model", CandidateDigest: "digest", CountsVersion: "counts"}
	tests := []struct {
		name  string
		value *RouterDecision
		want  bool
	}{
		{"nil", nil, false},
		{"selected", &valid, true},
	}
	for _, field := range []string{"version", "decision", "target", "digest", "counts"} {
		d := valid
		switch field {
		case "version":
			d.Version = "unknown"
		case "decision":
			d.DecisionID = ""
		case "target":
			d.TargetModelID = ""
		case "digest":
			d.CandidateDigest = strings.Repeat("猫", 257)
		case "counts":
			d.CountsVersion = strings.Repeat("猫", 257)
		}
		tests = append(tests, struct {
			name  string
			value *RouterDecision
			want  bool
		}{field, &d, false})
	}
	for _, field := range []string{"decision", "target", "digest", "counts"} {
		for _, length := range []int{256, 257} {
			d := valid
			value := strings.Repeat("猫", length)
			switch field {
			case "decision":
				d.DecisionID = value
			case "target":
				d.TargetModelID = value
			case "digest":
				d.CandidateDigest = value
			case "counts":
				d.CountsVersion = value
			}
			name := field + "/boundary"
			if length == 257 {
				name = field + "/oversize"
			}
			tests = append(tests, struct {
				name  string
				value *RouterDecision
				want  bool
			}{name, &d, length == 256})
		}
	}
	unselected := valid
	unselected.Selected = false
	unselected.TargetModelID = ""
	tests = append(tests, struct {
		name  string
		value *RouterDecision
		want  bool
	}{"unselected without target", &unselected, true})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.value.Clone()
			if (got != nil) != tt.want {
				t.Fatalf("Clone()=%+v, valid=%v", got, tt.want)
			}
			if got != nil {
				if got == tt.value || *got != *tt.value {
					t.Fatal("clone must be equal and independently owned")
				}
				got.TargetModelID = "changed"
				if tt.value.TargetModelID == "changed" {
					t.Fatal("clone aliases source")
				}
			}
		})
	}
}

func TestRouterDecisionUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name, raw       string
		valid, selected bool
	}{
		{"selected", `{"version":"simple-exploration-decision/1","decision_id":"d","selected":true,"target_model_id":"b","candidate_digest":"digest","counts_version":"counts"}`, true, true},
		{"unselected null target", `{"version":"simple-exploration-decision/1","decision_id":"d","selected":false,"target_model_id":null}`, true, false},
		{"missing selected", `{"version":"simple-exploration-decision/1","decision_id":"d"}`, false, false},
		{"null selected", `{"version":"simple-exploration-decision/1","decision_id":"d","selected":null}`, false, false},
		{"wrong selected type", `{"selected":"false"}`, false, false},
		{"wrong field type", `{"decision_id":123,"selected":false}`, false, false},
		{"null", `null`, false, false},
		{"array", `[]`, false, false},
		{"malformed", `{`, false, false},
		{"unknown version", `{"version":"future","decision_id":"d","selected":false}`, false, false},
		{"missing target", `{"version":"simple-exploration-decision/1","decision_id":"d","selected":true}`, false, false},
		{"oversized payload", `{"padding":"` + strings.Repeat("x", 8192) + `"}`, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := RouterDecision{Version: "stale", DecisionID: "stale", Selected: true, TargetModelID: "stale"}
			if err := d.UnmarshalJSON([]byte(tt.raw)); err != nil {
				t.Fatalf("optional metadata must fail open: %v", err)
			}
			if !tt.valid {
				if d != (RouterDecision{}) {
					t.Fatalf("invalid input retained stale state: %+v", d)
				}
				return
			}
			if d.Clone() == nil || d.DecisionID != "d" || d.Selected != tt.selected {
				t.Fatalf("valid decision lost: %+v", d)
			}
			if tt.selected && (d.TargetModelID != "b" || d.CandidateDigest != "digest" || d.CountsVersion != "counts") {
				t.Fatalf("fields lost: %+v", d)
			}
			raw, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			var roundTrip RouterDecision
			if err = json.Unmarshal(raw, &roundTrip); err != nil || roundTrip != d {
				t.Fatalf("round trip changed decision: %+v %v", roundTrip, err)
			}
		})
	}
}

func TestRouterDecisionInvalidMetadataPreservesRanking(t *testing.T) {
	var response struct {
		Models   []string        `json:"model_list"`
		Decision *RouterDecision `json:"routing_decision"`
	}
	err := json.Unmarshal([]byte(`{"model_list":["a","b"],"routing_decision":{"selected":"invalid"}}`), &response)
	if err != nil || len(response.Models) != 2 || response.Models[0] != "a" || response.Decision.Clone() != nil {
		t.Fatalf("optional audit damaged ranking: %+v %v", response, err)
	}
}
