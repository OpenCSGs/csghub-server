package scheduler

type Config struct {
	ImageBuilderURL string `json:"image_builder_url"`
	ImageRunnerURL  string `json:"image_runner_url"`
}
