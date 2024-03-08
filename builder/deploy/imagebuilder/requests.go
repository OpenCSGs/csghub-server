package imagebuilder

type (
	BuildRequest struct {
		UserName  string `json:"user_name"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`

		Hardware      string `json:"hardware"`
		PythonVersion string `json:"python_version"`
		SDKType       string `json:"sdk_type"`
		SDKVersion    string `json:"sdk_version"`

		GitRef         string `json:"git_ref"`
		GitUserID      string `json:"git_user_id"`
		GitAccessToken string `json:"git_access_token"`
	}
	BuildResponse struct{}

	StatusRequest struct {
		OrgName string `json:"org_name"`
		Name    string `json:"name"`
		Ref     string `json:"ref"`
	}

	StatusResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	LogsRequest struct {
		OrgName string `json:"org_name"`
		Name    string `json:"name"`
		Ref     string `json:"ref"`
	}

	LogsResponse struct{}
)
