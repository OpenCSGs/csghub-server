package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Only serverless and inference deploys serve LLM traffic and are synced to
// the AIGateway upstream store; spaces, finetunes, evaluations, notebooks,
// and other deploy types must not be pushed as upstreams.
func TestDeployTypeSyncsUpstream(t *testing.T) {
	tests := []struct {
		name       string
		deployType int
		want       bool
	}{
		{"serverless deploys sync", ServerlessType, true},
		{"inference deploys sync", InferenceType, true},
		{"space deploys do not sync", SpaceType, false},
		{"finetune deploys do not sync", FinetuneType, false},
		{"evaluation deploys do not sync", EvaluationType, false},
		{"notebook deploys do not sync", NotebookType, false},
		{"job deploys do not sync", JobType, false},
		{"unknown types do not sync", UnknownType, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DeployTypeSyncsUpstream(tt.deployType))
		})
	}
}
