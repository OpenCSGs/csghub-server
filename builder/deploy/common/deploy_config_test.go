package common

import (
	"testing"
	"time"

	"opencsg.com/csghub-server/common/config"

	"github.com/stretchr/testify/require"
)

func TestBuildDeployConfig(t *testing.T) {
	tests := []struct {
		name         string
		configSetup  func() *config.Config
		expectations func(*testing.T, DeployConfig)
	}{{
		name: "Normal configuration build",
		configSetup: func() *config.Config {
			cfg := &config.Config{}
			cfg.Space.BuilderEndpoint = "http://builder.example.com"
			cfg.Space.RunnerEndpoint = "http://runner.example.com"
			cfg.Space.InternalRootDomain = "internal.example.com"
			cfg.Space.DeployTimeoutInMin = 30
			cfg.Model.DeployTimeoutInMin = 60
			cfg.Space.BuildTimeoutInMin = 45
			cfg.Model.DownloadEndpoint = "https://download.example.com"
			cfg.Accounting.ChargingEnable = true
			cfg.Space.PublicRootDomain = "public.example.com"
			cfg.S3.InternalEndpoint = "http://internal-s3.example.com"
			cfg.UniqueServiceName = "test-service"
			cfg.APIToken = "test-api-token"
			cfg.Runner.HearBeatIntervalInSec = 15
			return cfg
		},
		expectations: func(t *testing.T, result DeployConfig) {
			require.Equal(t, "http://builder.example.com", result.ImageBuilderURL)
			require.Equal(t, "http://runner.example.com", result.ImageRunnerURL)
			require.Equal(t, 10*time.Second, result.MonitorInterval)
			require.Equal(t, "internal.example.com", result.InternalRootDomain)
			require.Equal(t, 30, result.SpaceDeployTimeoutInMin)
			require.Equal(t, 60, result.ModelDeployTimeoutInMin)
			require.Equal(t, 45, result.BuildTimeoutInMin)
			require.Equal(t, "https://download.example.com", result.ModelDownloadEndpoint)
			require.Equal(t, true, result.ChargingEnable)
			require.Equal(t, "public.example.com", result.PublicRootDomain)
			require.Equal(t, true, result.S3Internal) // Because InternalEndpoint is not empty
			require.Equal(t, "test-service", result.UniqueServiceName)
			require.Equal(t, "test-api-token", result.APIToken)
			require.Equal(t, "test-api-token", result.APIKey) // APIKey should equal APIToken
			require.Equal(t, 15, result.HeartBeatTimeInSec)
		},
	}, {
		name: "Empty S3InternalEndpoint case",
		configSetup: func() *config.Config {
			cfg := &config.Config{}
			// Keep most configurations empty, but set key fields
			cfg.S3.InternalEndpoint = ""
			return cfg
		},
		expectations: func(t *testing.T, result DeployConfig) {
			require.Equal(t, false, result.S3Internal) // Because InternalEndpoint is empty
			// Verify default values are correct
			require.Equal(t, 10*time.Second, result.MonitorInterval)
		},
	}, {
		name: "Partial empty configurations case",
		configSetup: func() *config.Config {
			cfg := &config.Config{}
			// Set only partial configurations
			cfg.Space.BuilderEndpoint = "http://builder.example.com"
			cfg.Model.DownloadEndpoint = "https://download.example.com"
			return cfg
		},
		expectations: func(t *testing.T, result DeployConfig) {
			require.Equal(t, "http://builder.example.com", result.ImageBuilderURL)
			require.Equal(t, "https://download.example.com", result.ModelDownloadEndpoint)
			// Verify default or empty values for other fields
			require.Equal(t, "", result.ImageRunnerURL)
			require.Equal(t, false, result.ChargingEnable)
		},
	}, {
		name: "Timeout settings validation",
		configSetup: func() *config.Config {
			cfg := &config.Config{}
			cfg.Space.DeployTimeoutInMin = 0
			cfg.Model.DeployTimeoutInMin = -1
			cfg.Space.BuildTimeoutInMin = 120
			return cfg
		},
		expectations: func(t *testing.T, result DeployConfig) {
			// Even with invalid values, the function should pass them through
			require.Equal(t, 0, result.SpaceDeployTimeoutInMin)
			require.Equal(t, -1, result.ModelDeployTimeoutInMin)
			require.Equal(t, 120, result.BuildTimeoutInMin)
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup configuration
			cfg := tt.configSetup()
			// Call the tested function
			result := BuildDeployConfig(cfg)
			// Verify results
			tt.expectations(t, result)
		})
	}
}
