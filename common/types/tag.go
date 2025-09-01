package types

import (
	"time"
)

type RepoTag struct {
	ID        int64     `json:"id,omitempty"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Group     string    `json:"group"`
	BuiltIn   bool      `json:"built_in"`
	Scope     TagScope  `json:"scope,omitempty"`
	ShowName  string    `json:"show_name" i18n:"Tag.I18nKey"`
	I18nKey   string    `json:"i18n_key,omitempty"`
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

type RepoTagCategory struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	ShowName string   `json:"show_name" i18n:"tag_category.name"`
	Scope    TagScope `json:"scope"`
	Enabled  bool     `json:"enabled"`
}

type CreateTag struct {
	Name     string `json:"name" binding:"required"`
	Category string `json:"category" binding:"required"`
	Group    string `json:"group"`
	Scope    string `json:"scope" binding:"required"`
	BuiltIn  bool   `json:"built_in"`
	// deprecated: use I18nKey instead
	ShowName string `json:"show_name"`
	I18nKey  string `json:"i18n_key"`
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
	Search     string     `form:"search" binding:"omitempty"`
}
