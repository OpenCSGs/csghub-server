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

		ImageID string `json:"image_id"`
	}

	RunResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	StopRequest struct {
		SpaceID   int64  `json:"space_id"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
	}

	StopResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	StatusRequest struct {
		SpaceID   int64  `json:"space_id"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
	}

	StatusResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	LogsRequest struct {
		SpaceID   int64  `json:"space_id"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
	}

	LogsResponse struct {
		SSEReadCloser io.ReadCloser `json:"sse_read_closer"`
	}
)
