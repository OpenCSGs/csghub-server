package types

// AgentTemplate represents the template for an agent
type AgentTemplate struct {
	ID          int64   `json:"id"`
	Type        *string `json:"type" binding:"required"`                 // Possible values: langflow, agno, code, etc.
	UserUUID    *string `json:"-"`                                       // Will be set from HTTP header using httpbase.GetCurrentUserUUID
	Name        *string `json:"name" binding:"required,max=255"`         // Agent template name
	Description *string `json:"description" binding:"omitempty,max=500"` // Agent template description
	Content     *string `json:"content" binding:"required"`              // Used to store the complete content of the template
	Public      bool    `json:"public"`                                  // Whether the template is public
}

// AgentInstance represents an instance created from an agent template
type AgentInstance struct {
	ID          int64   `json:"id"`
	TemplateID  *int64  `json:"template_id" binding:"omitempty,gte=1"` // Associated with the id in the template table
	UserUUID    *string `json:"-"`                                     // Will be set from HTTP header using httpbase.GetCurrentUserUUID
	Name        *string `json:"name" binding:"required"`               // Instance name
	Description *string `json:"description" binding:"omitempty"`       // Instance description
	Type        *string `json:"type" binding:"required"`               // Possible values: langflow, agno, code, etc.
	ContentID   *string `json:"content_id" binding:"omitempty"`        // Used to specify the unique id of the instance resource
	Public      bool    `json:"public"`                                // Whether the instance is public
	Editable    bool    `json:"editable"`                              // Whether the instance is editable
}
