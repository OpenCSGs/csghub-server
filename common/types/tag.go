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
	Name  string `json:"name" binding:"required"`
	Scope string `json:"scope" binding:"required"`
}

type UpdateCategory CreateCategory
