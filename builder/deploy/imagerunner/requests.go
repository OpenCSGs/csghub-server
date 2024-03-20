package imagerunner

import "io"

type (
	RunRequest struct {
		UserName  string `json:"user_name"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`

		GitRef string `json:"git_ref"`

		Hardware string `json:"hardware"`
		Env      string `json:"env"`

		BuildID int64  `json:"build_id"`
		ImageID string `json:"image_id"`
	}

	RunResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	StopRequest struct {
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
		BuildID   int64  `json:"build_id"`
		ImageID   string `json:"image_id"`
	}

	StopResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	StatusRequest struct {
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
		BuildID   int64  `json:"build_id"`
		ImageID   string `json:"image_id"`
	}

	StatusResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	LogsRequest struct {
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
		BuildID   int64  `json:"build_id"`
		ImageID   string `json:"image_id"`
	}

	LogsResponse struct {
		SSEReadCloser io.ReadCloser `json:"sse_read_closer"`
	}
)
