package types

import (
	"time"
)

type RepoTag struct {
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Group     string    `json:"group"`
	BuiltIn   bool      `json:"built_in"`
	ShowName  string    `json:"show_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TagCategory string

const (
	TaskCategory       TagCategory = "task"
	LicenseCategory    TagCategory = "license"
	FrameworkCategory  TagCategory = "framework"
	SizeCategory       TagCategory = "size"
	LanguageCategory   TagCategory = "language"
	EvaluationCategory TagCategory = "evaluation"
)

type CreateTag struct {
	Name     string `json:"name" binding:"required"`
	Category string `json:"category" binding:"required"`
	Group    string `json:"group"`
	Scope    string `json:"scope" binding:"required"`
	BuiltIn  bool   `json:"built_in"`
	ShowName string `json:"show_name"`
}

type UpdateTag CreateTag

type CreateCategory struct {
	Name     string `json:"name" binding:"required"`
	Scope    string `json:"scope" binding:"required"`
	ShowName string `json:"show_name"`
	Enabled  bool   `json:"enabled"`
}

type UpdateCategory CreateCategory

type TagScope string

const (
	ModelTagScope   TagScope = "model"
	DatasetTagScope TagScope = "dataset"
	CodeTagScope    TagScope = "code"
	SpaceTagScope   TagScope = "space"
	PromptTagScope  TagScope = "prompt"
	MCPTagScope     TagScope = "mcp"
	UnknownScope    TagScope = "unknown"
)

type TagFilter struct {
	Scopes     []TagScope `form:"scope" binding:"omitempty,dive,eq=model|eq=dataset|eq=code|eq=space|eq=prompt|eq=mcp"`
	Categories []string   `form:"category" binding:"omitempty,dive"`
	BuiltIn    *bool      `form:"built_in" binding:"omitnil"`
}
