package types

type CreateOrgReq struct {
	Name string `json:"name"`
	//Display name
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Username    string `json:"username"`
}

type EditOrgReq struct {
	//Display name
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Name        string `json:"path"`
}

type OrgDatasetsReq struct {
	//org name of dataset
	Namespace   string `json:"namespace"`
	CurrentUser string `json:"current_user"`
	PageOpts
}

type OrgModelsReq = OrgDatasetsReq
