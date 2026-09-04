package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commonType "opencsg.com/csghub-server/common/types"
)

func TestToTypesModelTarget_Nil(t *testing.T) {
	assert.Nil(t, toTypesModelTarget(nil))
}

func TestToTypesModelTarget_FullConversion(t *testing.T) {
	model := &types.Model{BaseModel: types.BaseModel{ID: "model-1"}}
	upstream := commonType.UpstreamConfig{URL: "http://upstream", Provider: "test"}
	attempts := []commonType.UpstreamConfig{{URL: "http://fallback"}}

	r := &resolvedModelTarget{
		Model:          model,
		Upstream:       upstream,
		Target:         "http://upstream/v1/messages",
		Host:           "upstream",
		ModelName:      "model-1",
		AttemptTargets: attempts,
	}

	mt := toTypesModelTarget(r)
	require.NotNil(t, mt)
	assert.Equal(t, model, mt.Model)
	assert.Equal(t, upstream, mt.Upstream)
	assert.Equal(t, "http://upstream/v1/messages", mt.Target)
	assert.Equal(t, "upstream", mt.Host)
	assert.Equal(t, "model-1", mt.ModelName)
	assert.Equal(t, attempts, mt.AttemptTargets)
}

func TestModelTargetError_ImplementsCodedError(t *testing.T) {
	// Verify that modelTargetError implements plan.CodedError via ModelErrorCode().
	// This is checked at compile time by the errors.As call in categorizePlanError.
	err := newInvalidRequestModelTargetError("model_not_found", "model not found", modelTargetErrorOptions{})
	assert.Equal(t, "model_not_found", err.ModelErrorCode())

	err2 := newServerModelTargetError("model_unavailable", "unavailable", modelTargetErrorOptions{})
	assert.Equal(t, "model_unavailable", err2.ModelErrorCode())
}
