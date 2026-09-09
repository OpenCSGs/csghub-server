package rebac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObserverImplementations(t *testing.T) {
	called := false
	observer := ObserverFunc(func(context.Context, Observation) {
		called = true
	})
	observer.Observe(context.Background(), Observation{Operation: OperationCheck})
	require.True(t, called)
	require.NotPanics(t, func() {
		NoopObserver().Observe(context.Background(), Observation{})
		ObserverFunc(nil).Observe(context.Background(), Observation{})
	})
}
