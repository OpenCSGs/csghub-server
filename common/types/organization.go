package types

type CreateOrgReq struct {
	// Org unique identifier
	Name string `json:"name" example:"org_name_1"`
	// Display name
	Nickname    string `json:"nickname" example:"org_display_name"`
	Description string `json:"description" example:"org description"`
	Username    string `json:"-"`
	Homepage    string `json:"homepage,omitempty" example:"https://www.example.com"`
	Logo        string `json:"logo,omitempty" example:"https://www.example.com/logo.png"`
	Verified    bool   `json:"verified" example:"false"`
	OrgType     string `json:"org_type" example:"company or school etc"`
}

type EditOrgReq struct {
	// Display name
	Nickname    *string `json:"nickname" example:"org display name"`
	Description *string `json:"description" example:"org description"`
	// TODO:rename json field name to 'name", need to negotiate with Portal engineer
	// Org unique identifier
	Name        string  `json:"-"`
	Homepage    *string `json:"homepage,omitempty" example:"https://www.example.com"`
	Logo        *string `json:"logo,omitempty" example:"https://www.example.com/logo.png"`
	Verified    *bool   `json:"verified" example:"false"`
	OrgType     *string `json:"org_type" example:"company or school etc"`
	CurrentUser string  `json:"-"`
}

type DeleteOrgReq struct {
	Name        string `json:"-"`
	CurrentUser string `json:"-"`
}

type OrgDatasetsReq struct {
	// org name of dataset
	Namespace   string `json:"namespace"`
	CurrentUser string `json:"-"`
	PageOpts
}

type (
	OrgModelsReq = OrgDatasetsReq
	OrgCodesReq  = OrgDatasetsReq
	OrgSpacesReq = OrgDatasetsReq
)

type Organization struct {
	// unique name of the organization
	Name     string `json:"path"`
	Nickname string `json:"name,omitempty"`
	Homepage string `json:"homepage,omitempty"`
	Logo     string `json:"logo,omitempty"`
	OrgType  string `json:"org_type,omitempty"`
	Verified bool   `json:"verified"`
}

type Member struct {
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	UUID        string `json:"uuid"`
	Avatar      string `json:"avatar,omitempty"`
	Role        string `json:"role,omitempty"`
	LastLoginAt string `json:"last_login_at,omitempty"`
}
