package scheduler

type Config struct {
	ImageBuilderURL string `json:"image_builder_url"`
	ImageRunnerURL  string `json:"image_runner_url"`
}

type SDKConfig struct {
	Name    string
	Version string
	Port    int
	Image   string
}

const DefaultContainerPort = 8080

var (
	GRADIO = SDKConfig{
		Name:    "gradio",
		Version: "3.37.0",
		Port:    7860,
		Image:   "",
	}
	STREAMLIT = SDKConfig{
		Name:    "streamlit",
		Version: "1.33.0",
		Port:    8501,
		Image:   "",
	}
	NGINX = SDKConfig{
		Name:    "nginx",
		Version: "1.25.0",
		Port:    8000,
		Image:   "csg-nginx:1.2",
	}
	DOCKER = SDKConfig{
		Name:    "docker",
		Version: "",
		Port:    8080,
		Image:   "",
	}
)

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
