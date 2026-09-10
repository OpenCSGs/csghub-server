package types

import "time"

type PromptVersionReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	CurrentUser string `json:"current_user"`
	FilePath    string `json:"file_path"`
	Version     string `json:"version"`
}

type CreatePromptVersionReq struct {
	Version       string `json:"version" binding:"required"`
	SourceVersion string `json:"source_version"`
	Changelog     string `json:"changelog"`
}

type PromptVersion struct {
	ID        int64     `json:"id"`
	Version   string    `json:"version"`
	FilePath  string    `json:"file_path"`
	Commit    string    `json:"commit"`
	Changelog string    `json:"changelog"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PromptVersionDetail struct {
	PromptVersion
	Prompt PromptOutput `json:"prompt"`
}
