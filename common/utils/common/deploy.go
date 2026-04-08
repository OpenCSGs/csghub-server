package common

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func ExtractDeployTargetAndHost(ctx context.Context, cluster *database.ClusterInfo, req types.EndpointReq) (string, string, error) {
	target := req.Target
	host := req.Host
	appSvcName := req.SvcName

	if len(cluster.AppEndpoint) < 1 {
		slog.Warn("app endpoint of cluster is empty", slog.Any("clusterID", cluster.ClusterID))
		return target, host, nil
	}

	target = cluster.AppEndpoint
	if len(req.Endpoint) < 1 {
		return "", "", fmt.Errorf("endpoint of deploy %s is empty", appSvcName)
	}

	host, err := extractHostFromEndpoint(req.Endpoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract host from endpoint %s, error: %w", req.Endpoint, err)
	}

	return target, host, nil
}

func extractHostFromEndpoint(endpoint string) (string, error) {
	// http://u-neo888-test0922-2-lv.spaces.app.internal
	// extract host from url
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse endpoint url %s, error: %w", endpoint, err)
	}
	host := u.Hostname()
	if len(host) < 1 {
		return "", fmt.Errorf("extract host of endpoint %s is empty", endpoint)
	}
	return host, nil
}
