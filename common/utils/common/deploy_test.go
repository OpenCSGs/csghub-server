package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestExtractDeployTargetAndHost_EmptyAppEndpoint(t *testing.T) {
	ctx := context.Background()
	cluster := &database.ClusterInfo{
		ClusterID:   "cluster-1",
		AppEndpoint: "",
	}
	req := types.EndpointReq{
		Target:   "http://svc.default.svc.cluster.local",
		Host:     "",
		SvcName:  "test-svc",
		Endpoint: "http://test-endpoint",
	}

	target, host, err := ExtractDeployTargetAndHost(ctx, cluster, req)
	require.Nil(t, err)
	require.Equal(t, "http://svc.default.svc.cluster.local", target)
	require.Equal(t, "", host)
}

func TestExtractDeployTargetAndHost_WithAppEndpoint_EmptyEndpoint(t *testing.T) {
	ctx := context.Background()
	cluster := &database.ClusterInfo{
		ClusterID:   "cluster-1",
		AppEndpoint: "remote.app.internal",
	}
	req := types.EndpointReq{
		Target:   "http://svc.default.svc.cluster.local",
		Host:     "",
		SvcName:  "test-svc",
		Endpoint: "",
	}

	_, _, err := ExtractDeployTargetAndHost(ctx, cluster, req)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "endpoint of deploy")
}

func TestExtractDeployTargetAndHost_WithAppEndpoint_InvalidEndpoint(t *testing.T) {
	ctx := context.Background()
	cluster := &database.ClusterInfo{
		ClusterID:   "cluster-1",
		AppEndpoint: "remote.app.internal",
	}
	req := types.EndpointReq{
		Target:   "http://svc.default.svc.cluster.local",
		Host:     "",
		SvcName:  "test-svc",
		Endpoint: ":::",
	}

	_, _, err := ExtractDeployTargetAndHost(ctx, cluster, req)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to parse endpoint")
}

func TestExtractDeployTargetAndHost_WithAppEndpoint_Success(t *testing.T) {
	ctx := context.Background()
	cluster := &database.ClusterInfo{
		ClusterID:   "cluster-1",
		AppEndpoint: "remote.app.internal",
	}
	req := types.EndpointReq{
		Target:   "http://svc.default.svc.cluster.local",
		Host:     "",
		SvcName:  "test-svc",
		Endpoint: "http://svc.example.svc.cluster.local",
	}

	target, host, err := ExtractDeployTargetAndHost(ctx, cluster, req)
	require.Nil(t, err)
	require.Equal(t, "remote.app.internal", target)
	require.Equal(t, "svc.example.svc.cluster.local", host)
}

func TestExtractDeployTargetAndHost_WithAppEndpoint_HttpEndpoint(t *testing.T) {
	ctx := context.Background()
	cluster := &database.ClusterInfo{
		ClusterID:   "cluster-1",
		AppEndpoint: "spaces.app.opencsg.com",
	}
	req := types.EndpointReq{
		Target:   "http://svc.default.svc.cluster.local",
		Host:     "",
		SvcName:  "u-user-space",
		Endpoint: "http://u-user-space.spaces.app.opencsg.com",
	}

	target, host, err := ExtractDeployTargetAndHost(ctx, cluster, req)
	require.Nil(t, err)
	require.Equal(t, "spaces.app.opencsg.com", target)
	require.Equal(t, "u-user-space.spaces.app.opencsg.com", host)
}

func TestExtractHostFromEndpoint_ValidURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		expected string
	}{
		{
			name:     "simple http url",
			endpoint: "http://example.com",
			expected: "example.com",
		},
		{
			name:     "url with port",
			endpoint: "http://example.com:8080",
			expected: "example.com",
		},
		{
			name:     "url with path",
			endpoint: "http://example.com/path/to/resource",
			expected: "example.com",
		},
		{
			name:     "url with subdomain",
			endpoint: "http://api.example.com",
			expected: "api.example.com",
		},
		{
			name:     "url with hyphen in subdomain",
			endpoint: "http://my-service.example.com",
			expected: "my-service.example.com",
		},
		{
			name:     "https url",
			endpoint: "https://secure.example.com",
			expected: "secure.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := extractHostFromEndpoint(tt.endpoint)
			require.Nil(t, err)
			require.Equal(t, tt.expected, host)
		})
	}
}

func TestExtractHostFromEndpoint_InvalidURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{
			name:     "malformed url",
			endpoint: ":::",
		},
		{
			name:     "invalid scheme",
			endpoint: "not-a-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := extractHostFromEndpoint(tt.endpoint)
			require.NotNil(t, err)
			require.Empty(t, host)
		})
	}
}
