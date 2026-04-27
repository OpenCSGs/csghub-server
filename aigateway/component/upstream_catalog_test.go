package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestNormalizeUpstreamCatalog(t *testing.T) {
	tests := []struct {
		name        string
		apiEndpoint string
		upstreams   []commontypes.UpstreamConfig
		expected    []commontypes.UpstreamConfig
	}{
		{
			name:        "empty upstreams and empty apiEndpoint",
			apiEndpoint: "",
			upstreams:   nil,
			expected:    []commontypes.UpstreamConfig{},
		},
		{
			name:        "empty upstreams but valid apiEndpoint",
			apiEndpoint: "https://example.com/api",
			upstreams:   nil,
			expected: []commontypes.UpstreamConfig{
				{
					URL:     "https://example.com/api",
					Weight:  1,
					Enabled: true,
				},
			},
		},
		{
			name:        "empty upstreams but valid apiEndpoint with spaces",
			apiEndpoint: "  https://example.com/api  ",
			upstreams:   []commontypes.UpstreamConfig{},
			expected: []commontypes.UpstreamConfig{
				{
					URL:     "https://example.com/api",
					Weight:  1,
					Enabled: true,
				},
			},
		},
		{
			name:        "valid upstreams provided, ignore apiEndpoint",
			apiEndpoint: "https://ignored.com/api",
			upstreams: []commontypes.UpstreamConfig{
				{
					URL:     "https://upstream1.com",
					Weight:  2,
					Enabled: true,
				},
			},
			expected: []commontypes.UpstreamConfig{
				{
					URL:     "https://upstream1.com",
					Weight:  2,
					Enabled: true,
				},
			},
		},
		{
			name:        "upstreams with empty URL should be skipped",
			apiEndpoint: "",
			upstreams: []commontypes.UpstreamConfig{
				{
					URL:     "",
					Weight:  1,
					Enabled: true,
				},
				{
					URL:     "  ",
					Weight:  1,
					Enabled: true,
				},
				{
					URL:     "https://upstream1.com",
					Weight:  1,
					Enabled: true,
				},
			},
			expected: []commontypes.UpstreamConfig{
				{
					URL:     "https://upstream1.com",
					Weight:  1,
					Enabled: true,
				},
			},
		},
		{
			name:        "upstreams with zero or negative weight should default to 1",
			apiEndpoint: "",
			upstreams: []commontypes.UpstreamConfig{
				{
					URL:     "https://upstream1.com",
					Weight:  0,
					Enabled: true,
				},
				{
					URL:     "https://upstream2.com",
					Weight:  -5,
					Enabled: true,
				},
			},
			expected: []commontypes.UpstreamConfig{
				{
					URL:     "https://upstream1.com",
					Weight:  1,
					Enabled: true,
				},
				{
					URL:     "https://upstream2.com",
					Weight:  1,
					Enabled: true,
				},
			},
		},
		{
			name:        "explicitly disabled upstreams are preserved",
			apiEndpoint: "",
			upstreams: []commontypes.UpstreamConfig{
				{
					URL:     "https://upstream1.com",
					Weight:  1,
					Enabled: false,
				},
			},
			expected: []commontypes.UpstreamConfig{
				{
					URL:     "https://upstream1.com",
					Weight:  1,
					Enabled: false,
				},
			},
		},
		{
			name:        "URLs are trimmed of spaces",
			apiEndpoint: "",
			upstreams: []commontypes.UpstreamConfig{
				{
					URL:     "  https://upstream1.com  ",
					Weight:  2,
					Enabled: true,
				},
			},
			expected: []commontypes.UpstreamConfig{
				{
					URL:     "https://upstream1.com",
					Weight:  2,
					Enabled: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeUpstreamCatalog(tt.apiEndpoint, tt.upstreams)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFirstEnabledUpstream(t *testing.T) {
	tests := []struct {
		name      string
		upstreams []commontypes.UpstreamConfig
		expected  string
	}{
		{
			name:      "empty upstreams",
			upstreams: nil,
			expected:  "",
		},
		{
			name: "no enabled upstreams",
			upstreams: []commontypes.UpstreamConfig{
				{URL: "https://upstream1.com", Enabled: false},
				{URL: "https://upstream2.com", Enabled: false},
			},
			expected: "",
		},
		{
			name: "first enabled upstream has empty URL",
			upstreams: []commontypes.UpstreamConfig{
				{URL: "", Enabled: true},
				{URL: "  ", Enabled: true},
				{URL: "https://upstream3.com", Enabled: true},
			},
			expected: "https://upstream3.com",
		},
		{
			name: "multiple enabled upstreams, returns first",
			upstreams: []commontypes.UpstreamConfig{
				{URL: "https://upstream1.com", Enabled: false},
				{URL: "https://upstream2.com", Enabled: true},
				{URL: "https://upstream3.com", Enabled: true},
			},
			expected: "https://upstream2.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := firstEnabledUpstream(tt.upstreams)
			assert.Equal(t, tt.expected, result)
		})
	}
}
