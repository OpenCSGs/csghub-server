package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestResolveModelTarget_ExternalModel(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "backend-model",
		},
		Endpoint: "https://api.example.com/v1/chat/completions",
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1")

	require.NoError(t, err)
	require.Equal(t, model, resolved.Model)
	require.Equal(t, "https://api.example.com/v1/chat/completions", resolved.Target)
	require.Empty(t, resolved.Host)
	require.Equal(t, "backend-model", resolved.ModelName)
}

func TestResolveModelTarget_ModelNotFound(t *testing.T) {
	tester, _, _ := setupTest(t)
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "missing-model").Return(nil, nil).Once()

	_, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "missing-model")

	require.Error(t, err)
	targetErr, ok := err.(*modelTargetError)
	require.True(t, ok)
	require.Equal(t, "model_not_found", targetErr.APIError.Code)
}
