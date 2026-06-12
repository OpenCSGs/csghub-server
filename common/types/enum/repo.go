package enum

type SourceType string

const (
	HFSource     string = "huggingface"
	MSSource     string = "modelscope"
	CSGSource    string = "opencsg"
	GitHubSource string = "github"
	// OtherSource identifies mirror sources that do not map to a dedicated repository source path column.
	OtherSource string = "other"
)
