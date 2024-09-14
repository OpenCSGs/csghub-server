package rpc

type Namespace struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Avatar string `json:"avatar,omitempty"`
}

type User struct {
	ID                int64          `json:"id,omitempty"`
	Username          string         `json:"username"`
	Nickname          string         `json:"nickname"`
	Phone             string         `json:"phone,omitempty"`
	Email             string         `json:"email,omitempty"`
	UUID              string         `json:"uuid,omitempty"`
	Avatar            string         `json:"avatar,omitempty"`
	Bio               string         `json:"bio,omitempty"`
	Homepage          string         `json:"homepage,omitempty"`
	Roles             []string       `json:"roles,omitempty"`
	LastLoginAt       string         `json:"last_login_at,omitempty"`
	Orgs              []Organization `json:"orgs,omitempty"`
	CanChangeUserName bool           `json:"can_change_username,omitempty"`
}

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
