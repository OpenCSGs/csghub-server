package types

type SpaceTemplateReq struct {
	Type        string `json:"type" binding:"required"`
	Name        string `json:"name" binding:"required"`
	ShowName    string `json:"show_name" binding:"required"`
	Enable      bool   `json:"enable"`
	Path        string `json:"path" binding:"required"`
	DevMode     bool   `json:"dev_mode"`
	Port        int    `json:"port"`
	Secrets     string `json:"secrets"`
	Variables   string `json:"variables"`
	Description string `json:"description"`
}

type UpdateSpaceTemplateReq struct {
	ID          int64   `json:"-"`
	Type        *string `json:"type"`
	Name        *string `json:"name"`
	ShowName    *string `json:"show_name"`
	Enable      *bool   `json:"enable"`
	Path        *string `json:"path"`
	DevMode     *bool   `json:"dev_mode"`
	Port        *int    `json:"port"`
	Secrets     *string `json:"secrets"`
	Variables   *string `json:"variables"`
	Description *string `json:"description"`
}
