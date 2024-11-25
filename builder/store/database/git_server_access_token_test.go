package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestGitServerAccessTokenStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := database.NewGitServerAccessTokenStoreWithDB(db)

	token := &database.GitServerAccessToken{
		Token:      "test-token",
		ServerType: database.MirrorServer,
	}

	createdToken, err := store.Create(ctx, token)
	require.Nil(t, err)
	require.NotNil(t, createdToken)
	require.Equal(t, "test-token", createdToken.Token)
	require.Equal(t, database.MirrorServer, createdToken.ServerType)
}

func TestGitServerAccessTokenStore_Index(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := database.NewGitServerAccessTokenStoreWithDB(db)

	// Insert multiple tokens
	tokens := []*database.GitServerAccessToken{
		{Token: "token1", ServerType: database.MirrorServer},
		{Token: "token2", ServerType: database.GitServer},
	}
	for _, token := range tokens {
		_, err := store.Create(ctx, token)
		require.Nil(t, err)
	}

	// Fetch all tokens
	allTokens, err := store.Index(ctx)
	require.Nil(t, err)
	require.Equal(t, len(tokens), len(allTokens))

	tokensMap := make(map[string]database.GitServerType)
	for _, token := range allTokens {
		tokensMap[token.Token] = token.ServerType
	}
	require.Equal(t, database.MirrorServer, tokensMap["token1"])
	require.Equal(t, database.GitServer, tokensMap["token2"])
}

func TestGitServerAccessTokenStore_FindByType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := database.NewGitServerAccessTokenStoreWithDB(db)

	// Insert tokens with different server types
	tokens := []*database.GitServerAccessToken{
		{Token: "token1", ServerType: database.MirrorServer},
		{Token: "token2", ServerType: database.GitServer},
		{Token: "token3", ServerType: database.MirrorServer},
	}
	for _, token := range tokens {
		_, err := store.Create(ctx, token)
		require.Nil(t, err)
	}

	// Fetch tokens by server type
	mirrorTokens, err := store.FindByType(ctx, string(database.MirrorServer))
	require.Nil(t, err)
	require.Equal(t, 2, len(mirrorTokens))
	require.ElementsMatch(t, []string{"token1", "token3"}, []string{mirrorTokens[0].Token, mirrorTokens[1].Token})

	gitTokens, err := store.FindByType(ctx, string(database.GitServer))
	require.Nil(t, err)
	require.Equal(t, 1, len(gitTokens))
	require.Equal(t, "token2", gitTokens[0].Token)
}
