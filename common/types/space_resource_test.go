package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScenarioName(t *testing.T) {
	tests := []struct {
		name string
		in   ScenarioType
		want string
	}{
		{"deploy space", ScenarioSpace, "space"},
		{"deploy inference", ScenarioInference, "inference"},
		{"deploy finetune", ScenarioFinetune, "finetune"},
		{"deploy sandbox", ScenarioSandbox, "sandbox"},
		{"workflow evaluation", ScenarioWfEvaluation, "wf_evaluation"},
		{"workflow dataflow", ScenarioWfDataflow, "wf_dataflow"},
		{"workflow llm log dataflow", ScenarioWfDataflowLLMLog, "wf_dataflow_llmlog"},
		{"zero value", 0, ""},
		{"all scenarios", ScenarioAll, ""},
		{"multi-bit mask", ScenarioSpace | ScenarioInference, ""},
		{"unassigned bit", 1 << 10, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ScenarioName(tt.in))
		})
	}
}
