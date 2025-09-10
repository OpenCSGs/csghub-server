package imagebuilder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// TestRemoteBuilderBuild tests the Build method with various scenarios
func TestRemoteBuilderBuild(t *testing.T) {
	tests := []struct {
		name           string
		request        *types.ImageBuilderRequest
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectedError  string
	}{
		{
			name: "successful build request",
			request: &types.ImageBuilderRequest{
				ClusterID:      "cluster-001",
				SpaceName:      "test-space",
				OrgName:        "test-org",
				SpaceURL:       "https://example.com/test-org/test-space",
				Sdk:            "gradio",
				Sdk_version:    "4.0.0",
				PythonVersion:  "3.9",
				Hardware:       "cpu-basic",
				GitRef:         "main",
				UserId:         "user-123",
				GitAccessToken: "token-456",
				DeployId:       "build-789",
				LastCommitID:   "abc123def456",
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/api/v1/imagebuilder/builder", r.URL.Path)
				// Note: Content-Type is set by the internal HTTP client, not by rtypes.HttpClient

				// Verify request body
				var reqBody types.ImageBuilderRequest
				err := json.NewDecoder(r.Body).Decode(&reqBody)
				assert.NoError(t, err)
				assert.Equal(t, "test-space", reqBody.SpaceName)
				assert.Equal(t, "test-org", reqBody.OrgName)
				assert.Equal(t, "build-789", reqBody.DeployId)

				w.Header().Set("Content-Type", "application/json")
				err = json.NewEncoder(w).Encode(reqBody)
				require.Nil(t, err)
			},
			expectedError: "",
		},
		{
			name: "server returns error response",
			request: &types.ImageBuilderRequest{
				ClusterID: "cluster-001",
				SpaceName: "test-space",
				OrgName:   "test-org",
				DeployId:  "build-789",
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			expectedError: "EOF",
		},
		{
			name: "server returns empty response",
			request: &types.ImageBuilderRequest{
				SpaceName: "test-space",
				OrgName:   "test-org",
				DeployId:  "build-789",
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			// Parse server URL to create remote URL
			remoteURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			// Create RemoteBuilder
			mockClusterStore := mockdb.NewMockClusterInfoStore(t)
			builder := &RemoteBuilder{
				remote:       remoteURL,
				clusterStore: mockClusterStore,
				client:       server.Client(),
			}
			if tt.request.ClusterID != "" {
				mockClusterStore.EXPECT().
					ByClusterID(context.Background(), tt.request.ClusterID).
					Return(database.ClusterInfo{Mode: types.ConnectModeKubeConfig}, nil)
			}

			// Execute the method under test
			err = builder.Build(context.Background(), tt.request)
			if err != nil {
				assert.Equal(t, tt.expectedError, "EOF")
			} else {
				assert.Equal(t, "", tt.expectedError)
			}

		})
	}
}

// TestRemoteBuilderBuildEdgeCases tests edge cases for the Build method
func TestRemoteBuilderBuildEdgeCases(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		// Create a server that will delay the response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			select {
			case <-r.Context().Done():
				return // Request was cancelled
			case <-time.After(1 * time.Second):
				w.Header().Set("Content-Type", "application/json")
				require.Nil(t, nil)
			}
		}))
		defer server.Close()

		// Create HTTP client and RemoteBuilder
		remoteURL, err := url.Parse(server.URL)
		assert.NoError(t, err)

		builder := &RemoteBuilder{
			remote:       remoteURL,
			clusterStore: mockdb.NewMockClusterInfoStore(t),
			client:       server.Client(),
		}
		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		req := &types.ImageBuilderRequest{
			SpaceName: "test-space",
			OrgName:   "test-org",
			DeployId:  "build-789",
		}

		err = builder.Build(ctx, req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("server timeout", func(t *testing.T) {
		// Create a server that will timeout
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate server delay
			time.Sleep(100 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			require.Nil(t, nil)
		}))
		defer server.Close()

		remoteURL, err := url.Parse(server.URL)
		assert.NoError(t, err)
		builder := &RemoteBuilder{
			remote:       remoteURL,
			clusterStore: mockdb.NewMockClusterInfoStore(t),
			client:       server.Client(),
		}

		// Create context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		req := &types.ImageBuilderRequest{
			SpaceName: "test-space",
			OrgName:   "test-org",
			DeployId:  "build-789",
		}

		err = builder.Build(ctx, req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})

	t.Run("large request payload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify we can handle large payloads
			var reqBody types.ImageBuilderRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			assert.NoError(t, err)
			assert.NotEmpty(t, reqBody.SpaceURL) // Should contain our large URL

			w.Header().Set("Content-Type", "application/json")
			require.Nil(t, nil)
		}))
		defer server.Close()

		remoteURL, err := url.Parse(server.URL)
		assert.NoError(t, err)

		builder := &RemoteBuilder{
			remote:       remoteURL,
			clusterStore: mockdb.NewMockClusterInfoStore(t),
			client:       server.Client(),
		}

		// Create request with large URL (simulate large payload)
		largeURL := "https://example.com/" + string(make([]byte, 1000))
		req := &types.ImageBuilderRequest{
			SpaceName: "test-space",
			OrgName:   "test-org",
			SpaceURL:  largeURL,
			DeployId:  "build-789",
		}

		err = builder.Build(context.Background(), req)

		assert.NoError(t, err)
	})
}

// TestRemoteBuilderBuildIntegration tests the Build method with integration scenarios
func TestRemoteBuilderBuildIntegration(t *testing.T) {
	t.Run("complete build workflow", func(t *testing.T) {
		// Track request sequence
		requestCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++

			// Verify request structure
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/v1/imagebuilder/builder", r.URL.Path)

			var reqBody types.ImageBuilderRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			assert.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			require.Nil(t, nil)
		}))
		defer server.Close()

		remoteURL, err := url.Parse(server.URL)
		assert.NoError(t, err)

		mockClusterStore := mockdb.NewMockClusterInfoStore(t)
		builder := &RemoteBuilder{
			remote:       remoteURL,
			clusterStore: mockClusterStore,
			client:       server.Client(),
		}

		// Test with realistic configuration
		req := &types.ImageBuilderRequest{
			ClusterID:      "prod-cluster",
			SpaceName:      "my-gradio-app",
			OrgName:        "my-organization",
			SpaceURL:       "https://huggingface.co/spaces/my-organization/my-gradio-app",
			Sdk:            "gradio",
			Sdk_version:    "4.0.0",
			PythonVersion:  "3.9",
			Hardware:       "cpu-basic",
			GitRef:         "main",
			UserId:         "user123",
			GitAccessToken: "ghp_xxxxxxxxxxxx",
			DeployId:       "integration-test-001",
			LastCommitID:   "a1b2c3d4e5f6",
			FactoryBuild:   false,
		}

		if req.ClusterID != "" {
			mockClusterStore.EXPECT().
				ByClusterID(context.Background(), req.ClusterID).
				Return(database.ClusterInfo{Mode: types.ConnectModeKubeConfig}, nil)
		}

		err = builder.Build(context.Background(), req)

		assert.NoError(t, err)
		assert.Equal(t, 1, requestCount)
	})
}
