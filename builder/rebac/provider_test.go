package rebac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderContractShape(t *testing.T) {
	provider := &mockProvider{
		name: "test",
		check: func(context.Context, CheckRequest) (Decision, error) {
			return Decision{Allowed: true}, nil
		},
	}
	var contract Provider = provider
	decision, err := contract.Check(context.Background(), validCheckRequest())
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, "test", contract.Name())
}
