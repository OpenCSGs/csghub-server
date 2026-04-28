package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestInferenceArchComponent_GetInferenceArch(t *testing.T) {
	// Create a mock inference arch store
	mockStore := mockdatabase.NewMockInferenceArchStore(t)

	// Set up the mock to return a test inference arch
	testArch := &database.InferenceArch{
		ID:       1,
		Patterns: "test-pattern",
	}

	mockStore.On("GetInferenceArch", mock.Anything).Return(testArch, nil)

	// Create the component with the mock store
	component := NewInferenceArchComponentWithStore(mockStore)

	// Test getting inference arch
	arch, err := component.GetInferenceArch(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, arch)
	assert.Equal(t, "test-pattern", arch.Patterns)

	// Verify the mock was called
	mockStore.AssertCalled(t, "GetInferenceArch", mock.Anything)
}

func TestInferenceArchComponent_UpdateInferenceArch(t *testing.T) {
	// Create a mock inference arch store
	mockStore := mockdatabase.NewMockInferenceArchStore(t)

	// Set up the mock to return a test inference arch
	testArch := &database.InferenceArch{
		ID:       1,
		Patterns: "updated-pattern",
	}

	mockStore.On("UpdateInferenceArch", mock.Anything, mock.Anything).Return(testArch, nil)

	// Create the component with the mock store
	component := NewInferenceArchComponentWithStore(mockStore)

	// Test updating inference arch
	req := &types.CreateInferenceArchReq{
		Patterns: "updated-pattern",
	}

	arch, err := component.UpdateInferenceArch(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, arch)
	assert.Equal(t, "updated-pattern", arch.Patterns)

	// Verify the mock was called
	mockStore.AssertCalled(t, "UpdateInferenceArch", mock.Anything, mock.Anything)
}
