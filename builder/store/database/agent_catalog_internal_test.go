package database

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentInstanceNameConflict(t *testing.T) {
	// pgdriver.Error cannot be constructed outside the driver, so the positive
	// case is covered by TestAgentCatalogStoreCreateLogicalDuplicateNameIsFriendly
	// against a real PostgreSQL (CI).
	require.False(t, agentInstanceNameConflict(errors.New(`ERROR: duplicate key value violates unique constraint "idx_agent_instances_type_content_id_unique_active" (SQLSTATE 23505)`)))
	require.False(t, agentInstanceNameConflict(errors.New("connection refused")))
	require.False(t, agentInstanceNameConflict(nil))
}
