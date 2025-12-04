package imagerunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// mockHTTPClient for intercepting HTTP requests
type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestRemoteRunner_insideCluster_Status(t *testing.T) {
	// Replace "Status" with the correct field name from types.StatusResponse, e.g., "State"

	req := &types.StatusRequest{
		OrgName:   "testOrg",
		RepoName:  "testRepo",
		ClusterID: "cluster1",
		SvcName:   "testSvc",
	}

	expectedStatus := &types.StatusResponse{
		DeployID: 1,
		Code:     200,
		Message:  "Running",
	}

	mockHTTPClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			resp := expectedStatus

			statusBytes, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(statusBytes)),
			}, nil
		},
	}

	mockClusterStore := mockdb.NewMockClusterInfoStore(t)
	mockClusterStore.EXPECT().ByClusterID(context.Background(), req.ClusterID).Return(database.ClusterInfo{
		ClusterID:      req.ClusterID,
		Mode:           types.ConnectModeInCluster,
		RunnerEndpoint: "in-cluster-endpoint",
	}, nil)

	runner := &RemoteRunner{
		clusterStore: mockClusterStore,
		client:       mockHTTPClient,
	}

	got, err := runner.Status(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, expectedStatus) {
		t.Errorf("expected %v, got %v", expectedStatus, got)
	}

}

func TestRemoteRunner_outsideCluster_Status(t *testing.T) {

	req := &types.StatusRequest{
		OrgName:   "testOrg",
		RepoName:  "testRepo",
		ClusterID: "cluster1",
		SvcName:   "testSvc",
	}

	expectedStatus := &types.StatusResponse{
		DeployID: 1,
		Code:     200,
		Message:  "Running",
	}

	mockHTTPClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			resp := expectedStatus

			statusBytes, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(statusBytes)),
			}, nil
		},
	}

	mockClusterStore := mockdb.NewMockClusterInfoStore(t)
	mockClusterStore.EXPECT().ByClusterID(context.Background(), req.ClusterID).Return(database.ClusterInfo{
		ClusterID:      req.ClusterID,
		Mode:           types.ConnectModeKubeConfig,
		RunnerEndpoint: "kubeconfig-cluster-endpoint",
	}, nil)

	runner := &RemoteRunner{
		remote:       &url.URL{Scheme: "http", Host: "remote-endpoint"},
		clusterStore: mockClusterStore,
		client:       mockHTTPClient,
	}

	got, err := runner.Status(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, expectedStatus) {
		t.Errorf("expected %v, got %v", expectedStatus, got)
	}
}

func TestRemoteRunner_GetRemoteRunnerHost(t *testing.T) {
	mockClusterStore := mockdb.NewMockClusterInfoStore(t)
	defaultRemoteURL, _ := url.Parse("http://default.runner")
	runner := &RemoteRunner{
		remote:       defaultRemoteURL,
		clusterStore: mockClusterStore,
	}

	t.Run("empty clusterID", func(t *testing.T) {
		host, err := runner.GetRemoteRunnerHost(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedHost := "http://default.runner"
		if host != expectedHost {
			t.Errorf("expected host %s, got %s", expectedHost, host)
		}
	})

	t.Run("in-cluster mode", func(t *testing.T) {
		clusterID := "in-cluster"
		expectedHost := "http://in-cluster.endpoint"
		mockClusterStore.EXPECT().ByClusterID(context.Background(), clusterID).Return(database.ClusterInfo{
			Mode:           types.ConnectModeInCluster,
			RunnerEndpoint: expectedHost,
		}, nil).Once()

		host, err := runner.GetRemoteRunnerHost(context.Background(), clusterID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if host != expectedHost {
			t.Errorf("expected host %s, got %s", expectedHost, host)
		}
	})

	t.Run("kubeconfig mode", func(t *testing.T) {
		clusterID := "kubeconfig-cluster"
		mockClusterStore.EXPECT().ByClusterID(context.Background(), clusterID).Return(database.ClusterInfo{
			Mode: types.ConnectModeKubeConfig,
		}, nil).Once()

		host, err := runner.GetRemoteRunnerHost(context.Background(), clusterID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedHost := "http://default.runner"
		if host != expectedHost {
			t.Errorf("expected host %s, got %s", expectedHost, host)
		}
	})

	t.Run("store error", func(t *testing.T) {
		clusterID := "error-cluster"
		expectedErr := errors.New("database error")
		mockClusterStore.EXPECT().ByClusterID(context.Background(), clusterID).Return(database.ClusterInfo{}, expectedErr).Once()

		_, err := runner.GetRemoteRunnerHost(context.Background(), clusterID)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestRemoteRunner_GetClusterById(t *testing.T) {
	clusterID := "test-cluster"
	expectedCluster := &types.ClusterResponse{
		ClusterID: clusterID,
		Region:    "test-region",
		Zone:      "test-zone",
		Provider:  "test-provider",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/cluster/"+clusterID {
			t.Errorf("expected path /api/v1/cluster/%s, got %s", clusterID, r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(expectedCluster)
		require.Nil(t, err)
	}))
	defer server.Close()

	mockClusterStore := mockdb.NewMockClusterInfoStore(t)
	mockClusterStore.EXPECT().ByClusterID(mock.Anything, clusterID).Return(database.ClusterInfo{
		Mode:           types.ConnectModeInCluster,
		RunnerEndpoint: server.URL,
	}, nil).Once()

	remoteURL, _ := url.Parse(server.URL)
	runner := &RemoteRunner{
		remote:       remoteURL,
		client:       server.Client(),
		clusterStore: mockClusterStore,
	}

	got, err := runner.GetClusterById(context.Background(), clusterID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, expectedCluster) {
		t.Errorf("expected cluster %v, got %v", expectedCluster, got)
	}
}

func TestRemoteRunner_GetClusterById_ServerError(t *testing.T) {
	clusterID := "test-cluster"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	mockClusterStore := mockdb.NewMockClusterInfoStore(t)
	mockClusterStore.EXPECT().ByClusterID(mock.Anything, clusterID).Return(database.ClusterInfo{
		Mode:           types.ConnectModeInCluster,
		RunnerEndpoint: server.URL,
	}, nil).Once()

	remoteURL, _ := url.Parse(server.URL)
	runner := &RemoteRunner{
		remote:       remoteURL,
		client:       server.Client(),
		clusterStore: mockClusterStore,
	}

	_, err := runner.GetClusterById(context.Background(), clusterID)
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
}

func TestRemoteRunner_GetClusterById_OutsideCluster(t *testing.T) {
	clusterID := "test-cluster"
	expectedCluster := &types.ClusterResponse{
		ClusterID: clusterID,
		Region:    "test-region",
		Zone:      "test-zone",
		Provider:  "test-provider",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/cluster/"+clusterID {
			t.Errorf("expected path /api/v1/cluster/%s, got %s", clusterID, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(expectedCluster)
		require.Nil(t, err)
	}))
	defer server.Close()

	mockClusterStore := mockdb.NewMockClusterInfoStore(t)
	mockClusterStore.EXPECT().ByClusterID(mock.Anything, clusterID).Return(database.ClusterInfo{
		Mode: types.ConnectModeKubeConfig, // Not in-cluster
	}, nil).Once()

	remoteURL, _ := url.Parse(server.URL)
	runner := &RemoteRunner{
		remote:       remoteURL,
		client:       server.Client(),
		clusterStore: mockClusterStore,
	}

	got, err := runner.GetClusterById(context.Background(), clusterID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, expectedCluster) {
		t.Errorf("expected cluster %v, got %v", expectedCluster, got)
	}
}
