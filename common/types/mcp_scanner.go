package types

// risk level
type Level string

const (
	LevelLow      Level = "low"
	LevelMedium   Level = "medium"
	LevelHigh     Level = "high"
	LevelCritical Level = "critical"
)

func (l Level) String() string {
	return string(l)
}

// Issue security problem
type ScannerIssue struct {
	Title       string `json:"title"`
	FilePath    string `json:"file_path"`
	RiskType    string `json:"risk_type"`
	Description string `json:"description"`
	Level       Level  `json:"level"`
	Suggestion  string `json:"suggestion"`
}

var TextFileExtensions = []string{
	// C and C++
	".c", ".cpp", ".cc", ".h", ".hpp",

	// Web
	".html", ".htm", ".css", ".js", ".ts", ".jsx", ".tsx",

	// Scripting languages
	".php", ".py", ".rb", ".pl", ".sh", ".bash", ".ps1",

	// JVM languages
	".java", ".kt", ".scala", ".groovy",

	// .NET languages
	".cs", ".vb", ".fs",

	// Other programming languages
	".go", ".rs", ".swift", ".dart", ".lua",

	// Data formats
	".json", ".xml", ".yaml", ".yml", ".toml", ".ini", ".sql",

	// Plain text
	".txt", ".md", ".log", ".csv", ".tsv", ".conf", ".cfg", ".text", ".asc", ".rtf",
}

type PluginName string

const (
	ToolPoisoningPluginName PluginName = "tool_poisoning"
)
