//go:build !ee && !saas

package activity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActivities_BatchMigrateToXnet_CE(t *testing.T) {
	activities := &Activities{}

	err := activities.BatchMigrateToXnet(context.Background())
	require.NoError(t, err)
}
