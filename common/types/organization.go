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

// CreateOrgReq implements SensitiveRequestV2
var _ SensitiveRequestV2 = (*CreateOrgReq)(nil)

func (c *CreateOrgReq) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name:     "name",
			Value:    func() string { return c.Name },
			Scenario: "nickname_detection",
		},
		{
			Name:     "nickname",
			Value:    func() string { return c.Nickname },
			Scenario: "nickname_detection",
		},
		{
			Name:     "description",
			Value:    func() string { return c.Description },
			Scenario: "comment_detection",
		},
		{
			Name:     "homepage",
			Value:    func() string { return c.Homepage },
			Scenario: "chat_detection",
		},
	}
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

// EditOrgReq implements SensitiveRequestV2
var _ SensitiveRequestV2 = (*EditOrgReq)(nil)

func (e *EditOrgReq) GetSensitiveFields() []SensitiveField {
	var fields []SensitiveField
	if e.Nickname != nil {
		fields = append(fields, SensitiveField{
			Name: "nickname",
			Value: func() string {
				return *e.Nickname
			},
			Scenario: "nickname_detection",
		})
	}
	if e.Description != nil {
		fields = append(fields, SensitiveField{
			Name: "description",
			Value: func() string {
				return *e.Description
			},
			Scenario: "comment_detection",
		})
	}
	if e.Homepage != nil {
		fields = append(fields, SensitiveField{
			Name: "homepage",
			Value: func() string {
				return *e.Homepage
			},
			Scenario: "chat_detection",
		})
	}
	return fields
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
	OrgModelsReq      = OrgDatasetsReq
	OrgCodesReq       = OrgDatasetsReq
	OrgSpacesReq      = OrgDatasetsReq
	OrgCollectionsReq = OrgDatasetsReq
	OrgPromptsReq     = OrgDatasetsReq
	OrgMCPsReq        = OrgDatasetsReq
)

type Organization struct {
	// unique name of the organization
	Name     string `json:"path"`
	Nickname string `json:"name,omitempty"`
	Homepage string `json:"homepage,omitempty"`
	Logo     string `json:"logo,omitempty"`
	OrgType  string `json:"org_type,omitempty"`
	Verified bool   `json:"verified"`
	UserID   int64  `json:"user_id,omitempty"`
}

type Member struct {
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	UUID        string `json:"uuid"`
	Avatar      string `json:"avatar,omitempty"`
	Role        string `json:"role,omitempty"`
	LastLoginAt string `json:"last_login_at,omitempty"`
}
