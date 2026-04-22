package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestCredentialStore_CreateFindListAndDuplicate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewCredentialStoreWithDB(db)

	credential := &database.Credential{
		NamespaceUUID:  "user-1",
		CredentialName: "gitlab-devops",
		Provider:       "generic",
		AuthType:       "bearer_token",
		Description:    "",
		SecretBackend:  "postgres_encrypted",
		SecretRef:      "pgp://credential_secrets/1",
		Status:         "active",
	}
	created, err := store.Create(ctx, credential)
	require.NoError(t, err)
	require.NotZero(t, created.ID)

	found, err := store.FindByName(ctx, "user-1", "gitlab-devops")
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)
	require.Empty(t, found.Description)

	_, err = store.Create(ctx, &database.Credential{
		NamespaceUUID:  "user-2",
		CredentialName: "gitlab-devops",
		Provider:       "generic",
		AuthType:       "bearer_token",
		Description:    "Other user",
		SecretBackend:  "postgres_encrypted",
		SecretRef:      "pgp://credential_secrets/3",
		Status:         "active",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, &database.Credential{
		NamespaceUUID:  "user-1",
		CredentialName: "revoked-token",
		Provider:       "generic",
		AuthType:       "bearer_token",
		Description:    "Revoked",
		SecretBackend:  "postgres_encrypted",
		SecretRef:      "pgp://credential_secrets/4",
		Status:         "revoked",
	})
	require.NoError(t, err)

	credentials, total, err := store.ListByUser(ctx, "user-1", types.CredentialFilter{}, 20, 1)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, credentials, 1)
	require.Equal(t, "gitlab-devops", credentials[0].CredentialName)

	credentials, total, err = store.ListByUser(ctx, "user-1", types.CredentialFilter{Search: "DEVOPS"}, 20, 1)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, credentials, 1)
	require.Equal(t, "gitlab-devops", credentials[0].CredentialName)

	credentials, total, err = store.ListByUser(ctx, "user-1", types.CredentialFilter{Search: "missing"}, 20, 1)
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Empty(t, credentials)

	_, err = store.Create(ctx, &database.Credential{
		NamespaceUUID:  "user-1",
		CredentialName: "gitlab-devops",
		Provider:       "generic",
		AuthType:       "bearer_token",
		Description:    "Duplicate",
		SecretBackend:  "postgres_encrypted",
		SecretRef:      "pgp://credential_secrets/2",
		Status:         "active",
	})
	require.ErrorIs(t, err, errorx.ErrDatabaseDuplicateKey)
}

func TestCredentialSecretStore_CreateUpdateDelete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewCredentialSecretStoreWithDB(db)

	created, err := store.Create(ctx, &database.CredentialSecret{
		SecretCiphertext: []byte("cipher-v1"),
		SecretNonce:      []byte("nonce-v1"),
		SecretVersion:    1,
		KMSKeyID:         "local-master-key-v1",
	})
	require.NoError(t, err)
	require.NotZero(t, created.ID)

	err = store.UpdateCipher(ctx, created.ID, []byte("cipher-v2"), []byte("nonce-v2"), 2, "local-master-key-v2")
	require.NoError(t, err)

	found, err := store.FindByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("cipher-v2"), found.SecretCiphertext)
	require.Equal(t, []byte("nonce-v2"), found.SecretNonce)
	require.Equal(t, 2, found.SecretVersion)
	require.Equal(t, "local-master-key-v2", found.KMSKeyID)

	require.NoError(t, store.Delete(ctx, created.ID))
	_, err = store.FindByID(ctx, created.ID)
	require.ErrorIs(t, err, errorx.ErrNotFound)
}

func TestTaskCredentialGrantStore_CreateBatchListAndRevoke(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	credentialStore := database.NewCredentialStoreWithDB(db)
	grantStore := database.NewTaskCredentialGrantStoreWithDB(db)

	credential, err := credentialStore.Create(ctx, &database.Credential{
		NamespaceUUID:  "user-1",
		CredentialName: "gitlab-devops",
		Provider:       "generic",
		AuthType:       "bearer_token",
		Description:    "GitLab",
		SecretBackend:  "postgres_encrypted",
		SecretRef:      "pgp://credential_secrets/1",
		Status:         "active",
	})
	require.NoError(t, err)

	expiresAt := time.Now().UTC().Add(time.Hour)
	created, err := grantStore.CreateBatchWithAuditLogs(ctx, []*database.TaskCredentialGrant{
		{
			TaskID:       "task-1",
			SessionID:    "session-1",
			AgentID:      "agent-1",
			CredentialID: credential.ID,
			ExpiresAt:    expiresAt,
		},
	}, []*database.CredentialAuditLog{
		{
			NamespaceUUID: "user-1",
			AgentID:       "agent-1",
			TaskID:        "task-1",
			SessionID:     "session-1",
			CredentialID:  credential.ID,
			Provider:      "generic",
			Action:        "credential_grant_create",
			Result:        "success",
		},
	})
	require.NoError(t, err)
	require.Len(t, created, 1)
	require.NotZero(t, created[0].ID)

	grants, err := grantStore.ListBySessionID(ctx, "session-1")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, credential.ID, grants[0].CredentialID)

	validGrant, validCredential, err := grantStore.FindValidBySessionAndCredentialID(ctx, "session-1", credential.ID)
	require.NoError(t, err)
	require.Equal(t, grants[0].ID, validGrant.ID)
	require.Equal(t, credential.ID, validCredential.ID)

	require.NoError(t, credentialStore.MarkRevoked(ctx, credential.ID))
	_, _, err = grantStore.FindValidBySessionAndCredentialID(ctx, "session-1", credential.ID)
	require.True(t, errors.Is(err, errorx.ErrNotFound), "expected ErrNotFound for revoked credential, got %v", err)

	revoked, err := grantStore.RevokeBySessionID(ctx, "session-1")
	require.NoError(t, err)
	require.Equal(t, int64(1), revoked)

	_, _, err = grantStore.FindValidBySessionAndCredentialID(ctx, "session-1", credential.ID)
	require.True(t, errors.Is(err, errorx.ErrNotFound), "expected ErrNotFound, got %v", err)
}
