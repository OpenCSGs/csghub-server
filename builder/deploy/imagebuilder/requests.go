package imagebuilder

import "io"

type (
	BuildRequest struct {
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`

		Hardware      string `json:"hardware"`
		PythonVersion string `json:"python_version"`
		SDKType       string `json:"sdk"`
		SDKVersion    string `json:"sdk_version"`

		SpaceURL       string `json:"space_url"`
		GitRef         string `json:"git_ref"`
		GitUserID      string `json:"git_user_id"`
		GitAccessToken string `json:"git_access_token"`

		BuildID      int64 `json:"build_id"`
		FactoryBuild bool  `json:"factory_build"`
	}
	BuildResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	StatusRequest struct {
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
		BuildID   int64  `json:"build_id"`
		// for local builder test only
		CurrentStatus int
	}

	StatusResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		ImageID string `json:"image_id"`
	}

	LogsRequest struct {
		OrgName   string `json:"org_name"`
		SpaceName string `json:"name"`
		BuildID   int64  `json:"build_id"`
	}

	LogsResponse struct {
		SSEReadCloser io.ReadCloser `json:"sse_read_closer"`
	}
)
