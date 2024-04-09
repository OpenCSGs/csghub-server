package imagerunner

import "io"

type (
	RunRequest struct {
		SpaceID   int64  `json:"space_id"`
		UserName  string `json:"user_name"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`

		GitRef string `json:"git_ref"`

		Hardware string `json:"hardware"`
		Env      string `json:"env"`

		ImageID  string `json:"image_id"`
		DeployID int64  `json:"deploy_id"`
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
)
