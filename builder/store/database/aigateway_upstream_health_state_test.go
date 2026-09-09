package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAIGatewayUpstreamHealthStateStoreMutateSerializesConcurrentDimensions(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store := database.NewAIGatewayUpstreamHealthStateStoreWithDB(db)
	require.NoError(t, store.Create(ctx, &database.AIGatewayUpstreamHealthState{
		UpstreamID:  1,
		HealthState: "unknown",
		LastCheckAt: time.Now(),
		Metadata:    map[string]any{},
	}))

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondEntered := make(chan struct{})
	results := make(chan error, 2)

	go func() {
		_, err := store.MutateByUpstreamID(ctx, database.AIGatewayUpstreamHealthStateMutation{
			UpstreamID: 1,
			Mutate: func(state *database.AIGatewayUpstreamHealthState) error {
				close(firstEntered)
				<-releaseFirst
				state.Metadata["l7"] = "available"
				return nil
			},
		})
		results <- err
	}()
	select {
	case <-firstEntered:
	case err := <-results:
		require.NoError(t, err)
		t.Fatal("first mutation returned before entering its callback")
	case <-ctx.Done():
		t.Fatal("timed out waiting for the first mutation")
	}

	go func() {
		_, err := store.MutateByUpstreamID(ctx, database.AIGatewayUpstreamHealthStateMutation{
			UpstreamID: 1,
			Mutate: func(state *database.AIGatewayUpstreamHealthState) error {
				close(secondEntered)
				state.Metadata["inference"] = "available"
				return nil
			},
		})
		results <- err
	}()

	select {
	case <-secondEntered:
		t.Fatal("second mutation entered before the first row lock was released")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	require.NoError(t, <-results)
	require.NoError(t, <-results)

	persisted, err := store.GetByUpstreamID(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "available", persisted.Metadata["l7"])
	require.Equal(t, "available", persisted.Metadata["inference"])
}

func TestAIGatewayUpstreamHealthStateStoreMutateRollsBackRejectedMutation(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAIGatewayUpstreamHealthStateStoreWithDB(db)
	require.NoError(t, store.Create(ctx, &database.AIGatewayUpstreamHealthState{
		UpstreamID:  2,
		HealthState: "unhealthy",
		LastCheckAt: time.Now(),
		Metadata:    map[string]any{"preserved": true},
	}))

	expectedErr := context.Canceled
	_, err := store.MutateByUpstreamID(ctx, database.AIGatewayUpstreamHealthStateMutation{
		UpstreamID: 2,
		Mutate: func(state *database.AIGatewayUpstreamHealthState) error {
			state.HealthState = "healthy"
			state.Metadata = map[string]any{"overwritten": true}
			return expectedErr
		},
	})
	require.ErrorIs(t, err, expectedErr)

	persisted, err := store.GetByUpstreamID(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, "unhealthy", persisted.HealthState)
	require.Equal(t, true, persisted.Metadata["preserved"])
}

func TestAIGatewayUpstreamHealthStateStoreCreateAllowsTriggeringEvent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAIGatewayUpstreamHealthStateStoreWithDB(db)
	probeStartedAt := time.Now().Add(-time.Minute)

	persisted, err := store.MutateByUpstreamID(ctx, database.AIGatewayUpstreamHealthStateMutation{
		UpstreamID:      3,
		CreateIfMissing: true,
		Mutate: func(state *database.AIGatewayUpstreamHealthState) error {
			require.True(t, probeStartedAt.After(state.LastCheckAt))
			state.LastCheckAt = probeStartedAt
			return nil
		},
	})

	require.NoError(t, err)
	require.Equal(t, probeStartedAt, persisted.LastCheckAt)
}
