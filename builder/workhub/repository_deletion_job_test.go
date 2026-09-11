package workhub

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestRepositoryDeletionArgsJSONIsStable(t *testing.T) {
	payload, err := json.Marshal(RepositoryDeletionArgs{
		RepositoryID:   42,
		RepositoryType: types.ModelRepo,
		Path:           "acme/model",
		GitalyPath:     "models_acme/model",
		Migrated:       true,
		OwnerType:      database.OrgNamespace,
		OwnerUUID:      "organization-uuid",
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"repository_id":42,
		"repository_type":"model",
		"path":"acme/model",
		"gitaly_path":"models_acme/model",
		"migrated":true,
		"owner_type":"organization",
		"owner_uuid":"organization-uuid"
	}`, string(payload))
}

func TestRepositoryDeletionArgsUseDedicatedQueue(t *testing.T) {
	args := RepositoryDeletionArgs{}
	require.Equal(t, RepositoryDeletionQueue, args.Kind())
	require.Equal(t, RepositoryDeletionQueue, args.InsertOpts().Queue)
}

func TestRepositoryDeletionJobClientForwardsTransactionAndSnapshot(t *testing.T) {
	ctx := context.Background()
	jobClient := &fakeJobClient{}
	client := NewRepositoryDeletionJobClient(jobClient)
	tx := (*sql.Tx)(nil)
	input := database.RepositoryDeletionJobInput{
		RepositoryID:   42,
		RepositoryType: types.ModelRepo,
		Path:           "acme/model",
		GitalyPath:     "models_acme/model",
		Migrated:       true,
		OwnerType:      database.OrgNamespace,
		OwnerUUID:      "organization-uuid",
	}

	jobID, err := client.InsertRepositoryDeletionJobTx(ctx, tx, input)

	require.NoError(t, err)
	require.Equal(t, int64(123), jobID)
	require.True(t, jobClient.called)
	require.Same(t, tx, jobClient.tx)
	require.Equal(t, RepositoryDeletionArgs{
		RepositoryID:   input.RepositoryID,
		RepositoryType: input.RepositoryType,
		Path:           input.Path,
		GitalyPath:     input.GitalyPath,
		Migrated:       input.Migrated,
		OwnerType:      input.OwnerType,
		OwnerUUID:      input.OwnerUUID,
	}, jobClient.args)
	require.Equal(t, RepositoryDeletionQueue, jobClient.opts.Queue)
	require.Equal(t, RepositoryDeletionJobMaxAttempts, jobClient.opts.MaxAttempts)
}

func TestRepositoryDeletionJobClientRequiresJobClient(t *testing.T) {
	client := NewRepositoryDeletionJobClient(nil)

	_, err := client.InsertRepositoryDeletionJobTx(context.Background(), nil, database.RepositoryDeletionJobInput{})

	require.ErrorContains(t, err, "workhub job client is required")
}
