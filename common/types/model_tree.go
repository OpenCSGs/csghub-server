package types

type ModelRelation string
type RelationOperation string

const (
	RelationBase                 ModelRelation     = "base"
	RelationFinetune             ModelRelation     = "finetune"
	RelationAdapter              ModelRelation     = "adapter"
	RelationMerge                ModelRelation     = "merge"
	RelationQuantized            ModelRelation     = "quantized"
	RelationAdd                  RelationOperation = "add"
	RelationDelete               RelationOperation = "delete"
	MetaDataKeyTag               string            = "task"
	MetaDataKeyQuantized         string            = "quantized_by"
	MetaDataKeyAdapter1          string            = "instance_prompt"
	MetaDataKeyBaseModel         string            = "base_model"
	MetaDataKeyLibrary           string            = "library_name"
	MetaDataKeyBaseModelRelation string            = "base_model_relation"
	AdapterConfigFileName        string            = "adapter_config.json"
	QuantizedConfigFileName      string            = "quantize_config.json"
	ModelConfigFileName          string            = "config.json"
	GGUFExtension                string            = ".gguf"
	ONNXExtension                string            = ".onnx"
	QuantizationConfigKey        string            = "quantization_config"
	DiffusersLibraryName         string            = "diffusers"
)

var AdapterLibraryNames = []string{"peft", "adapter-transformers"}

var ModelPathMapping = map[string]string{
	"xlm-roberta-large": "FacebookAI/xlm-roberta-large",
	"bert-base-uncased": "google-bert/bert-base-uncased",
	"roberta-base":      "FacebookAI/roberta-base",
}

type ModelNode struct {
	ID        int64             `json:"id"`
	Path      string            `json:"path"`
	Relation  ModelRelation     `json:"relation"`
	Children  []*ModelNode      `json:"children,omitempty"`
	Brothers  int               `json:"brothers,omitempty"`
	Operation RelationOperation `json:"-"`
}

type ModelTree struct {
	ParentNodes []*ModelNode   `json:"parent_nodes"`
	CurrentNode *ModelNode     `json:"current_node"`
	SubNodeInfo map[string]int `json:"sub_node_info"`
}
type ModelTreeReq struct {
	SourceRepoID int64         `json:"source_repo_id"`
	SourcePath   string        `json:"source_path"`
	TargetRepoID int64         `json:"target_repo_id"`
	TargetPath   string        `json:"target_path"`
	Relation     ModelRelation `json:"relation"`
}

type ScanModels struct {
	Models []string `json:"models"`
}
