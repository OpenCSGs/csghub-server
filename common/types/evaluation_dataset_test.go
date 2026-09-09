package types

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluationDatasetsConfigValidate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		config := EvaluationDatasetsConfig{
			RuntimeFramework: "evalscope",
			Datasets: []EvaluationDatasetConfig{{
				Namespace:        "evalscope",
				RepoName:         "aime25",
				Category:         "evaluation",
				TagName:          "examination",
				RepoType:         "dataset",
				RuntimeFramework: "evalscope",
				Source:           "ms",
			}},
		}
		require.NoError(t, config.Validate())
	})

	t.Run("missing fields", func(t *testing.T) {
		config := EvaluationDatasetsConfig{
			RuntimeFramework: "evalscope",
			Datasets:         []EvaluationDatasetConfig{{Namespace: "evalscope"}},
		}
		require.Error(t, config.Validate())
	})

	t.Run("empty datasets", func(t *testing.T) {
		config := EvaluationDatasetsConfig{RuntimeFramework: "evalscope", Prune: true}
		require.Error(t, config.Validate())
	})

	t.Run("duplicate", func(t *testing.T) {
		dataset := EvaluationDatasetConfig{
			Namespace:        "evalscope",
			RepoName:         "aime25",
			Category:         "evaluation",
			TagName:          "examination",
			RepoType:         "dataset",
			RuntimeFramework: "evalscope",
			Source:           "ms",
		}
		config := EvaluationDatasetsConfig{
			RuntimeFramework: "evalscope",
			Datasets:         []EvaluationDatasetConfig{dataset, dataset},
		}
		require.Error(t, config.Validate())
	})

	t.Run("duplicate compatible framework", func(t *testing.T) {
		config := EvaluationDatasetsConfig{
			RuntimeFramework:            "evalscope",
			CompatibleRuntimeFrameworks: []string{"evalscope"},
			Datasets: []EvaluationDatasetConfig{{
				Namespace:        "evalscope",
				RepoName:         "aime25",
				Category:         "evaluation",
				TagName:          "examination",
				RepoType:         "dataset",
				RuntimeFramework: "evalscope",
				Source:           "ms",
			}},
		}
		require.Error(t, config.Validate())
	})

	t.Run("runtime framework mismatch", func(t *testing.T) {
		config := EvaluationDatasetsConfig{
			RuntimeFramework: "opencompass",
			Datasets: []EvaluationDatasetConfig{{
				Namespace:        "evalscope",
				RepoName:         "aime25",
				Category:         "evaluation",
				TagName:          "examination",
				RepoType:         "dataset",
				RuntimeFramework: "evalscope",
				Source:           "ms",
			}},
		}
		require.Error(t, config.Validate())
	})
}

func TestEvaluationDatasetsConfigFile(t *testing.T) {
	data, err := os.ReadFile("../../configs/evaluation/datasets/evalscope-datasets.json")
	require.NoError(t, err)

	var config EvaluationDatasetsConfig
	require.NoError(t, json.Unmarshal(data, &config))
	require.Equal(t, "1.10.0", config.Version)
	require.Equal(t, "evalscope", config.RuntimeFramework)
	require.Equal(t, []string{"evalscope", "amd-evalscope"}, config.RuntimeFrameworkNames())
	require.True(t, config.Prune)
	require.NotEmpty(t, config.Datasets)
	require.Contains(t, config.Datasets, EvaluationDatasetConfig{
		Namespace:        "evalscope",
		RepoName:         "aime25",
		Category:         "evaluation",
		TagName:          "examination",
		RepoType:         "dataset",
		RuntimeFramework: "evalscope",
		Source:           "ms",
	})
	require.NoError(t, config.Validate())
}
