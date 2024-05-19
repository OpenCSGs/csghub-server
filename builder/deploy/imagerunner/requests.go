package imagerunner

import (
	"io"

	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/types"
)

type (
	RunRequest struct {
		SpaceID   int64  `json:"space_id"`
		UserName  string `json:"user_name"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`

		GitRef string `json:"git_ref"`

		Hardware types.HardWare    `json:"hardware,omitempty"`
		Env      map[string]string `json:"env,omitempty"`

		ImageID   string `json:"image_id"`
		DeployID  int64  `json:"deploy_id"`
		ClusterId string `json:"cluster_id"`
	}

	RunResponse struct {
		DeployID int64  `json:"deploy_id"`
		Code     int    `json:"code"`
		Message  string `json:"message"`
	}

	StopRequest struct {
		SpaceID   int64  `json:"space_id"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
	}

	StopResponse struct {
		DeployID int64  `json:"deploy_id"`
		Code     int    `json:"code"`
		Message  string `json:"message"`
	}

	StatusRequest struct {
		SpaceID   int64  `json:"space_id"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
	}

	StatusResponse struct {
		DeployID int64  `json:"deploy_id"`
		Code     int    `json:"code"`
		Message  string `json:"message"`
	}

	LogsRequest struct {
		SpaceID   int64  `json:"space_id"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
		DeployID  int64  `json:"deploy_id"`
	}

	LogsResponse struct {
		SSEReadCloser io.ReadCloser `json:"sse_read_closer"`
	}

	CluserInfo struct {
		ClusterID     string                              `json:"cluster_id"`
		ClusterName   string                              `json:"cluster_name"`
		ClusterRegion string                              `json:"cluster_region"`
		Nodes         map[string]cluster.NodeResourceInfo `json:"nodes"`
	}
)
