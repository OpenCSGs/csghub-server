package types

type File struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Lfs         bool   `json:"lfs"`
	Size        int    `json:"size"`
	Commit      Commit `json:"commit"`
	Path        string `json:"path"`
	Mode        string `json:"mode"`
	SHA         string `json:"sha"`
	DownloadURL string `json:"download_url"`
	Content     string `json:"content"`
}
