package scheduler

type Config struct {
	ImageBuilderURL string `json:"image_builder_url"`
	ImageRunnerURL  string `json:"image_runner_url"`
}

type SDKConfig struct {
	name    string
	version string
	port    string
}

var GRADIO = SDKConfig{
	name:    "gradio",
	version: "3.37.0",
	port:    "7860",
}
var STREAMLIT = SDKConfig{
	name:    "streamlit",
	version: "1.33.0",
	port:    "8501",
}

type RepoInfo struct {
	DeployID     int64
	SpaceID      int64
	ModelID      int64
	RepoID       int64
	Path         string
	Name         string
	Sdk          string
	SdkVersion   string
	HTTPCloneURL string
	UserName     string
	RepoType     string
}
