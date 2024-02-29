package types

import "time"

type RepoTag struct {
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Group     string    `json:"group"`
	BuiltIn   bool      `json:"built_in"`
	ShowName  string    `json:"show_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
