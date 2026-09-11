package component

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRepositoryDeletionJobClientWithDBAllowsUninitializedDatabase(t *testing.T) {
	client, err := newRepositoryDeletionJobClientWithDB(nil)

	require.NoError(t, err)
	require.Nil(t, client)
}
