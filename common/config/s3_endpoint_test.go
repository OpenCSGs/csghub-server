package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NormalizeS3Endpoint(t *testing.T) {
	tests := []struct {
		name            string
		endpoint        string
		enableDualstack bool
		region          string
		expected        string
	}{
		{
			name:            "standard amazon endpoint to dualstack",
			endpoint:        "s3.us-east-2.amazonaws.com",
			enableDualstack: true,
			expected:        "s3.dualstack.us-east-2.amazonaws.com",
		},
		{
			name:            "already dualstack is idempotent",
			endpoint:        "s3.dualstack.us-east-2.amazonaws.com",
			enableDualstack: true,
			expected:        "s3.dualstack.us-east-2.amazonaws.com",
		},
		{
			name:            "dualstack stripped when disabled",
			endpoint:        "s3.dualstack.us-east-2.amazonaws.com",
			enableDualstack: false,
			expected:        "s3.us-east-2.amazonaws.com",
		},
		{
			name:            "standard endpoint unchanged when disabled",
			endpoint:        "s3.us-east-2.amazonaws.com",
			enableDualstack: false,
			expected:        "s3.us-east-2.amazonaws.com",
		},
		{
			name:            "hyphen-style legacy endpoint to dualstack",
			endpoint:        "s3-us-west-2.amazonaws.com",
			enableDualstack: true,
			expected:        "s3.dualstack.us-west-2.amazonaws.com",
		},
		{
			name:            "china endpoint to dualstack",
			endpoint:        "s3.cn-north-1.amazonaws.com.cn",
			enableDualstack: true,
			expected:        "s3.dualstack.cn-north-1.amazonaws.com.cn",
		},
		{
			name:            "endpoint with scheme to dualstack",
			endpoint:        "https://s3.us-east-2.amazonaws.com",
			enableDualstack: true,
			expected:        "https://s3.dualstack.us-east-2.amazonaws.com",
		},
		{
			name:            "fips endpoint unchanged",
			endpoint:        "s3-fips.us-east-2.amazonaws.com",
			enableDualstack: true,
			expected:        "s3-fips.us-east-2.amazonaws.com",
		},
		{
			name:            "non-amazon endpoint unchanged",
			endpoint:        "localhost:9000",
			enableDualstack: true,
			expected:        "localhost:9000",
		},
		{
			name:            "minio-like custom domain unchanged",
			endpoint:        "minio.example.com",
			enableDualstack: true,
			expected:        "minio.example.com",
		},
		{
			name:            "aliyun oss endpoint unchanged",
			endpoint:        "oss-cn-beijing.aliyuncs.com",
			enableDualstack: true,
			expected:        "oss-cn-beijing.aliyuncs.com",
		},
		{
			name:            "empty endpoint unchanged",
			endpoint:        "",
			enableDualstack: true,
			expected:        "",
		},
		{
			name:            "global endpoint without region unchanged",
			endpoint:        "s3.amazonaws.com",
			enableDualstack: true,
			expected:        "s3.amazonaws.com",
		},
		{
			name:            "global endpoint with region to dualstack regional",
			endpoint:        "s3.amazonaws.com",
			enableDualstack: true,
			region:          "us-east-2",
			expected:        "s3.dualstack.us-east-2.amazonaws.com",
		},
		{
			name:            "global endpoint with region to standard regional when disabled",
			endpoint:        "s3.amazonaws.com",
			enableDualstack: false,
			region:          "us-east-2",
			expected:        "s3.us-east-2.amazonaws.com",
		},
		{
			name:            "global endpoint with region unchanged when disabled already standard",
			endpoint:        "s3.us-east-2.amazonaws.com",
			enableDualstack: false,
			region:          "us-east-2",
			expected:        "s3.us-east-2.amazonaws.com",
		},
		{
			name:            "hyphen-style legacy endpoint canonicalized when disabled",
			endpoint:        "s3-us-west-2.amazonaws.com",
			enableDualstack: false,
			region:          "us-west-2",
			expected:        "s3.us-west-2.amazonaws.com",
		},
		{
			name:            "china endpoint unchanged when disabled",
			endpoint:        "s3.cn-north-1.amazonaws.com.cn",
			enableDualstack: false,
			expected:        "s3.cn-north-1.amazonaws.com.cn",
		},
		{
			name:            "s3 express control endpoint unchanged when enabled",
			endpoint:        "s3express-control.us-east-2.amazonaws.com",
			enableDualstack: true,
			region:          "us-east-2",
			expected:        "s3express-control.us-east-2.amazonaws.com",
		},
		{
			name:            "s3 express control endpoint unchanged when disabled",
			endpoint:        "s3express-control.us-east-2.amazonaws.com",
			enableDualstack: false,
			region:          "us-east-2",
			expected:        "s3express-control.us-east-2.amazonaws.com",
		},
		{
			name:            "s3 express zonal endpoint unchanged when enabled",
			endpoint:        "s3express-usw2-az3.us-west-2.amazonaws.com",
			enableDualstack: true,
			region:          "us-west-2",
			expected:        "s3express-usw2-az3.us-west-2.amazonaws.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizeS3Endpoint(tt.endpoint, tt.enableDualstack, tt.region))
		})
	}
}
