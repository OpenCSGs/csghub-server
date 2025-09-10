package types

import "time"

type SensitiveWordSet struct {
	ID        int64                     `json:"id"`
	Name      string                    `json:"name"`
	ShowName  string                    `json:"show_name"`
	Words     []string                  `json:"words"`
	Enabled   bool                      `json:"enabled"`
	CreatedAt time.Time                 `json:"created_at"`
	UpdatedAt time.Time                 `json:"updated_at"`
	Category  *SensitiveWordSetCategory `json:"category"`
}

type SensitiveWordSetCategory struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ShowName string `json:"show_name"`
}

type CreateSensitiveWordSetReq struct {
	Name       string   `json:"name"`
	ShowName   string   `json:"show_name"`
	Words      []string `json:"words"`
	Enabled    bool     `json:"enabled"`
	CategoryID int64    `json:"category_id"`
}

type UpdateSensitiveWordSetReq struct {
	ID         int64    `json:"_"`
	Name       *string  `json:"name"`
	ShowName   *string  `json:"show_name"`
	Words      []string `json:"words"`
	Enabled    *bool    `json:"enabled"`
	CategoryID *int64   `json:"category_id"`
}
