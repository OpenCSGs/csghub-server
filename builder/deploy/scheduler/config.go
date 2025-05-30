package scheduler

type Config struct {
	ImageBuilderURL string `json:"image_builder_url"`
	ImageRunnerURL  string `json:"image_runner_url"`
}

type RepoInfo struct {
	DeployID      int64
	SpaceID       int64
	ModelID       int64
	RepoID        int64
	Path          string
	Name          string
	Sdk           string
	SdkVersion    string
	DriverVersion string
	HTTPCloneURL  string
	UserName      string
	RepoType      string
}
