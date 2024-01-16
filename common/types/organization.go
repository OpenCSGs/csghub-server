package types

type CreateOrgReq struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Username    string `json:"username"`
}

type EditOrgReq struct {
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

type OrgDatasetsReq struct {
	Namespace   string `json:"namespace"`
	CurrentUser string `json:"current_user"`
	PageOpts
}

type OrgModelsReq = OrgDatasetsReq
