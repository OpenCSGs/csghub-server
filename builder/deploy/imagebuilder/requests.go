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
		DriverVersion string `json:"driver_version,omitempty"`

		SpaceGitURL    string `json:"space_url"`
		GitRef         string `json:"git_ref"`
		GitUserID      string `json:"user_id"`
		GitAccessToken string `json:"git_access_token"`

		BuildID      string `json:"build_id"`
		FactoryBuild bool   `json:"factory_build"`
		ClusterID    string `json:"cluster_id"`

		LastCommitID string `json:"last_commit_id"`
	}
	BuildResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	StatusRequest struct {
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`
		BuildID   string `json:"build_id"`
		// for local builder test only
		CurrentStatus int
		ClusterID     string `json:"cluster_id"`
	}

	StatusResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		ImageID string `json:"image_id"`
	}

	LogsRequest struct {
		OrgName   string `json:"org_name"`
		SpaceName string `json:"name"`
		BuildID   string `json:"build_id"`
		ClusterID string `json:"cluster_id"`
	}

	StopRequest struct {
		ClusterID string `json:"cluster_id"`
		Namespace string `json:"namespace"`
		Workname  string `json:"workname"`
	}

	LogsResponse struct {
		SSEReadCloser io.ReadCloser `json:"sse_read_closer"`
	}
)

func (s *StatusResponse) Success() bool {
	return s.Code == 0
}

func (s *StatusResponse) Fail() bool {
	return s.Code == 1
}

func (s *StatusResponse) Inprogress() bool {
	return s.Code == 2
}
