package types

type CreateOrgReq struct {
	// Org unique identifier
	Name string `json:"name" example:"org_name_1"`
	// Display name
	FullName    string `json:"full_name" example:"org display name"`
	Description string `json:"description" example:"org description"`
	Username    string `json:"username" example:"user_name_1"`
}

type EditOrgReq struct {
	// Display name
	FullName    string `json:"full_name" example:"org display name"`
	Description string `json:"description" example:"org description"`
	// TODO:rename json field name to 'name", need to negotiate with Portal engineer
	// Org unique identifier
	Name        string `json:"-"`
	CurrentUser string `json:"current_user"`
}

type DeleteOrgReq struct {
	Name        string `json:"-"`
	CurrentUser string `json:"current_user"`
}

type OrgDatasetsReq struct {
	// org name of dataset
	Namespace   string `json:"namespace"`
	CurrentUser string `json:"current_user"`
	PageOpts
}

type (
	OrgModelsReq = OrgDatasetsReq
	OrgCodesReq  = OrgDatasetsReq
	OrgSpacesReq = OrgDatasetsReq
)
