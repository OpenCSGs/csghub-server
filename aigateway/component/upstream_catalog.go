package component

import (
	"strings"

	commontypes "opencsg.com/csghub-server/common/types"
)

func normalizeUpstreamCatalog(apiEndpoint string, upstreams []commontypes.UpstreamConfig) []commontypes.UpstreamConfig {
	if len(upstreams) == 0 && strings.TrimSpace(apiEndpoint) != "" {
		upstreams = []commontypes.UpstreamConfig{
			{
				URL:     strings.TrimSpace(apiEndpoint),
				Weight:  1,
				Enabled: true,
			},
		}
	}

	normalized := make([]commontypes.UpstreamConfig, 0, len(upstreams))
	for _, upstream := range upstreams {
		upstream.URL = strings.TrimSpace(upstream.URL)
		if upstream.URL == "" {
			continue
		}
		if upstream.Weight <= 0 {
			upstream.Weight = 1
		}
		// Preserve explicitly disabled upstreams for config round-trip.
		normalized = append(normalized, upstream)
	}
	return normalized
}

func firstEnabledUpstream(upstreams []commontypes.UpstreamConfig) string {
	for _, upstream := range upstreams {
		if upstream.Enabled && strings.TrimSpace(upstream.URL) != "" {
			return upstream.URL
		}
	}
	return ""
}
