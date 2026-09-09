package types

import "fmt"

type EvaluationDatasetsConfig struct {
	Version                     string                    `json:"version"`
	RuntimeFramework            string                    `json:"runtime_framework"`
	CompatibleRuntimeFrameworks []string                  `json:"compatible_runtime_frameworks,omitempty"`
	Prune                       bool                      `json:"prune"`
	Datasets                    []EvaluationDatasetConfig `json:"datasets"`
}

type EvaluationDatasetConfig struct {
	Namespace        string `json:"namespace"`
	RepoName         string `json:"repo_name"`
	Category         string `json:"category"`
	TagName          string `json:"tag_name"`
	RepoType         string `json:"repo_type"`
	RuntimeFramework string `json:"runtime_framework"`
	Source           string `json:"source"`
}

func (c EvaluationDatasetsConfig) Validate() error {
	if c.RuntimeFramework == "" {
		return fmt.Errorf("evaluation dataset runtime_framework is required")
	}
	if len(c.Datasets) == 0 {
		return fmt.Errorf("evaluation dataset config for %s must not be empty", c.RuntimeFramework)
	}
	frameworks := map[string]struct{}{c.RuntimeFramework: {}}
	for _, runtimeFramework := range c.CompatibleRuntimeFrameworks {
		if runtimeFramework == "" {
			return fmt.Errorf("compatible runtime_framework must not be empty")
		}
		if _, ok := frameworks[runtimeFramework]; ok {
			return fmt.Errorf("duplicate runtime_framework in evaluation dataset config: %s", runtimeFramework)
		}
		frameworks[runtimeFramework] = struct{}{}
	}
	seen := make(map[string]struct{}, len(c.Datasets))
	for _, dataset := range c.Datasets {
		if dataset.Namespace == "" || dataset.RepoName == "" {
			return fmt.Errorf("evaluation dataset namespace and repo_name are required")
		}
		if dataset.Category == "" || dataset.TagName == "" || dataset.RepoType == "" {
			return fmt.Errorf("evaluation dataset %s/%s tag rule fields are required", dataset.Namespace, dataset.RepoName)
		}
		if dataset.RuntimeFramework == "" || dataset.Source == "" {
			return fmt.Errorf("evaluation dataset %s/%s runtime fields are required", dataset.Namespace, dataset.RepoName)
		}
		if dataset.RuntimeFramework != c.RuntimeFramework {
			return fmt.Errorf(
				"evaluation dataset %s/%s runtime_framework %q does not match config %q",
				dataset.Namespace,
				dataset.RepoName,
				dataset.RuntimeFramework,
				c.RuntimeFramework,
			)
		}
		key := dataset.Namespace + "/" + dataset.RepoName + "/" + dataset.Category
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate evaluation dataset config: %s", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (c EvaluationDatasetsConfig) RuntimeFrameworkNames() []string {
	runtimeFrameworks := make([]string, 0, len(c.CompatibleRuntimeFrameworks)+1)
	runtimeFrameworks = append(runtimeFrameworks, c.RuntimeFramework)
	return append(runtimeFrameworks, c.CompatibleRuntimeFrameworks...)
}
