package imagerunner

type (
	RunRequest struct {
		UserName  string `json:"user_name"`
		OrgName   string `json:"org_name"`
		SpaceName string `json:"space_name"`

		GitRef string `json:"git_ref"`

		Hardware string `json:"hardware"`
		Env      string `json:"env"`
	}
	RunResponse struct{}

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
