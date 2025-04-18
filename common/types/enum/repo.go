package enum

type SourceType string

const (
	HFSource     string = "huggingface"
	MSSource     string = "modelscope"
	CSGSource    string = "opencsg"
	GitHubSource string = "github"
)
