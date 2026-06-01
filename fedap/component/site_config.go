//go:build saas

// Package component implements the business logic layer for the fedap service.
// This file defines the SiteConfigProvider interface and the registry-backed implementation
// that fetches site configurations from the trust registry service via the api-server proxy.
package component

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/fedap/types"
)

// SiteConfigProvider abstracts the retrieval of federation site configurations.
// The fedap service uses this interface to look up connection details (Casdoor endpoint,
// client credentials, scopes, etc.) for a given site_id.
type SiteConfigProvider interface {
	// GetSiteConfig returns the configuration for the specified site.
	// Returns an error if loading the site configuration fails.
	GetSiteConfig(ctx context.Context, siteID string) (*types.SiteConfig, error)
	// ListSites returns all known site configurations.
	ListSites(ctx context.Context) ([]types.SiteConfig, error)
}

// registrySiteConfigProvider is a SiteConfigProvider backed by the trust registry service.
// It calls the trust registry API through the api-server reverse proxy and maps
// the FederationSite response to the fedap SiteConfig type.
type registrySiteConfigProvider struct {
	client rpc.TrustRegistrySvcClient
}

// NewSiteConfigProvider creates a SiteConfigProvider that fetches site configs from
// the trust registry service through the api-server proxy.
func NewSiteConfigProvider(cfg *config.Config) SiteConfigProvider {
	client := rpc.NewTrustRegistrySvcHttpClient(cfg.APIServer.PublicDomain, rpc.AuthWithApiKey(cfg.APIToken))
	return &registrySiteConfigProvider{
		client: client,
	}
}

// GetSiteConfig fetches a single federation site from the trust registry and converts
// it to a fedap SiteConfig.
func (p *registrySiteConfigProvider) GetSiteConfig(ctx context.Context, siteID string) (*types.SiteConfig, error) {
	site, err := p.client.GetFederationSite(ctx, siteID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting federation site", slog.String("siteID", siteID), slog.String("err", err.Error()))
		return nil, err
	}
	return p.toSiteConfig(site), nil
}

// ListSites fetches all federation sites from the trust registry and converts them
// to fedap SiteConfig values.
func (p *registrySiteConfigProvider) ListSites(ctx context.Context) ([]types.SiteConfig, error) {
	sites, _, err := p.client.ListFederationSites(ctx, 1000, 1)
	if err != nil {
		slog.ErrorContext(ctx, "error getting federation sites", slog.String("err", err.Error()))
		return nil, err
	}
	result := make([]types.SiteConfig, len(sites))
	for i := range sites {
		result[i] = *p.toSiteConfig(&sites[i])
	}
	return result, nil
}

// toSiteConfig maps a trust registry FederationSiteResponse to a fedap SiteConfig.
// AuthURL is mapped to CasdoorEndpoint. OAuth redirect_uri is request-scoped and
// supplied by the authorize flow, not persisted in site configuration.
func (p *registrySiteConfigProvider) toSiteConfig(site *rpc.FederationSiteResponse) *types.SiteConfig {
	return &types.SiteConfig{
		SiteID:          site.SiteID,
		Type:            site.Type,
		Name:            site.Name,
		Logo:            site.Logo,
		Status:          site.Status,
		BaseURL:         site.BaseURL,
		CreatedAt:       site.CreatedAt,
		UpdatedAt:       site.UpdatedAt,
		CasdoorEndpoint: site.AuthURL,
		ClientID:        site.ClientID,
		ClientSecret:    site.ClientSecret,
		Scopes:          site.Scopes,
	}
}
