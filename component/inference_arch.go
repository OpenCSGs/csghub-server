package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// InferenceArchComponent defines the interface for inference arch operations
type InferenceArchComponent interface {
	// GetInferenceArch gets the inference arch configuration
	GetInferenceArch(ctx context.Context) (*types.InferenceArch, error)

	// UpdateInferenceArch updates the inference arch configuration
	UpdateInferenceArch(ctx context.Context, req *types.CreateInferenceArchReq) (*types.InferenceArch, error)
}

// NewInferenceArchComponent creates a new InferenceArchComponent
func NewInferenceArchComponent() InferenceArchComponent {
	inferenceArchStore := database.NewInferenceArchStore()
	return &inferenceArchComponentImpl{
		inferenceArchStore: inferenceArchStore,
	}
}

// NewInferenceArchComponentWithStore creates a new InferenceArchComponent with the given store
func NewInferenceArchComponentWithStore(store database.InferenceArchStore) InferenceArchComponent {
	return &inferenceArchComponentImpl{
		inferenceArchStore: store,
	}
}

// inferenceArchComponent implements InferenceArchComponent

type inferenceArchComponentImpl struct {
	inferenceArchStore database.InferenceArchStore
}

// GetInferenceArch gets the inference arch configuration
func (c *inferenceArchComponentImpl) GetInferenceArch(ctx context.Context) (*types.InferenceArch, error) {
	arch, err := c.inferenceArchStore.GetInferenceArch(ctx)
	if err != nil {
		return nil, err
	}
	return arch.ToTypes(), nil
}

// UpdateInferenceArch updates the inference arch configuration
func (c *inferenceArchComponentImpl) UpdateInferenceArch(ctx context.Context, req *types.CreateInferenceArchReq) (*types.InferenceArch, error) {
	arch, err := c.inferenceArchStore.UpdateInferenceArch(ctx, req)
	if err != nil {
		return nil, err
	}
	return arch.ToTypes(), nil
}
