//go:build saas

package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/rpc"
)

func TestRegistrySiteConfigProvider_GetSiteConfig(t *testing.T) {
	ctx := context.TODO()
	mockClient := mockrpc.NewMockTrustRegistrySvcClient(t)
	provider := &registrySiteConfigProvider{
		client: mockClient,
	}

	t.Run("existing site", func(t *testing.T) {
		mockClient.EXPECT().GetFederationSite(ctx, "site-1").Return(&rpc.FederationSiteResponse{
			SiteID:       "site-1",
			Type:         "site",
			Name:         "Test Site",
			Logo:         "https://example.com/logo.png",
			BaseURL:      "https://example.com",
			AuthURL:      "https://casdoor.example.com",
			ClientID:     "client-123",
			ClientSecret: "secret-456",
			Scopes:       []string{"openid", "profile"},
			Status:       "active",
			CreatedAt:    "2026-05-01T00:00:00Z",
			UpdatedAt:    "2026-05-02T00:00:00Z",
		}, nil).Once()

		cfg, err := provider.GetSiteConfig(ctx, "site-1")
		require.NoError(t, err)
		assert.Equal(t, "site-1", cfg.SiteID)
		assert.Equal(t, "site", cfg.Type)
		assert.Equal(t, "Test Site", cfg.Name)
		assert.Equal(t, "https://example.com/logo.png", cfg.Logo)
		assert.Equal(t, "https://example.com", cfg.BaseURL)
		assert.Equal(t, "https://casdoor.example.com", cfg.CasdoorEndpoint)
		assert.Equal(t, "client-123", cfg.ClientID)
		assert.Equal(t, "secret-456", cfg.ClientSecret)
		assert.Equal(t, "active", cfg.Status)
		assert.Equal(t, "2026-05-01T00:00:00Z", cfg.CreatedAt)
		assert.Equal(t, "2026-05-02T00:00:00Z", cfg.UpdatedAt)
		assert.Equal(t, []string{"openid", "profile"}, cfg.Scopes)
	})

	t.Run("non-existing site", func(t *testing.T) {
		expectedErr := errors.New("not found")
		mockClient.EXPECT().GetFederationSite(ctx, "non-existing").Return(nil, expectedErr).Once()

		_, err := provider.GetSiteConfig(ctx, "non-existing")
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestRegistrySiteConfigProvider_ListSites(t *testing.T) {
	ctx := context.TODO()
	mockClient := mockrpc.NewMockTrustRegistrySvcClient(t)
	provider := &registrySiteConfigProvider{
		client: mockClient,
	}

	mockClient.EXPECT().ListFederationSites(ctx, 1000, 1).Return([]rpc.FederationSiteResponse{
		{
			SiteID:    "site-1",
			Type:      "site",
			Name:      "Site 1",
			Logo:      "https://example.com/logo-a.png",
			BaseURL:   "https://a.com",
			AuthURL:   "https://casdoor-a.com",
			ClientID:  "client-a",
			Scopes:    []string{"openid"},
			Status:    "active",
			CreatedAt: "2026-05-01T00:00:00Z",
			UpdatedAt: "2026-05-02T00:00:00Z",
		},
		{
			SiteID:    "site-2",
			Type:      "site",
			Name:      "Site 2",
			Logo:      "https://example.com/logo-b.png",
			BaseURL:   "https://b.com",
			AuthURL:   "https://casdoor-b.com",
			ClientID:  "client-b",
			Scopes:    []string{"openid", "profile"},
			Status:    "inactive",
			CreatedAt: "2026-05-03T00:00:00Z",
			UpdatedAt: "2026-05-04T00:00:00Z",
		},
	}, 2, nil).Once()

	sites, err := provider.ListSites(ctx)
	require.NoError(t, err)
	assert.Len(t, sites, 2)
	assert.Equal(t, "site-1", sites[0].SiteID)
	assert.Equal(t, "site", sites[0].Type)
	assert.Equal(t, "Site 1", sites[0].Name)
	assert.Equal(t, "https://example.com/logo-a.png", sites[0].Logo)
	assert.Equal(t, "https://casdoor-a.com", sites[0].CasdoorEndpoint)
	assert.Equal(t, "site-2", sites[1].SiteID)
}

func TestRegistrySiteConfigProvider_ListSites_ReturnsUnderlyingError(t *testing.T) {
	ctx := context.TODO()
	mockClient := mockrpc.NewMockTrustRegistrySvcClient(t)
	provider := &registrySiteConfigProvider{
		client: mockClient,
	}

	expectedErr := errors.New("list failed")
	mockClient.EXPECT().ListFederationSites(ctx, 1000, 1).Return(nil, 0, expectedErr).Once()

	sites, err := provider.ListSites(ctx)
	require.Error(t, err)
	assert.Nil(t, sites)
	assert.ErrorIs(t, err, expectedErr)
}
