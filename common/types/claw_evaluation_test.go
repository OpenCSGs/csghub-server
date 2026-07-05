package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluationReq_ToClawEvaluationReq(t *testing.T) {
	req := EvaluationReq{
		Username:           "user1",
		OwnerNamespace:     "org1",
		TaskName:           "claw-job",
		TaskDesc:           "desc",
		RuntimeFrameworkId: 3,
		ResourceId:         4,
		Command:            "batch",
		Model:              "glm-5.1",
		BaseURL:            "http://localhost/v1",
		ApiKey:             "sk-test",
		Config:             "cfg",
		Tasks:              "1-9",
		Trials:             2,
		Parallel:           4,
		JudgeModel:         "judge",
		NoJudge:            false,
		TraceDir:           "/tmp/trace",
		Proxy:              "http://proxy",
		ClusterID:          "cluster1",
		Image:              "opencsghq/claw-eval:1.0.0",
		RepoType:           "model",
		TaskType:           TaskTypeClawEval,
		ResourceName:       "cpu-small",
	}

	clawReq := req.ToClawEvaluationReq()
	require.Equal(t, req.Username, clawReq.Username)
	require.Equal(t, req.OwnerNamespace, clawReq.OwnerNamespace)
	require.Equal(t, req.TaskName, clawReq.TaskName)
	require.Equal(t, req.Model, clawReq.Model)
	require.Equal(t, req.BaseURL, clawReq.BaseURL)
	require.Equal(t, req.ApiKey, clawReq.ApiKey)
	require.Equal(t, req.Tasks, clawReq.Tasks)
	require.Equal(t, req.TaskType, clawReq.TaskType)
	require.Equal(t, req.Image, clawReq.Image)
}

func TestEvaluationReq_GetSensitiveFieldsExcludesAPIKey(t *testing.T) {
	req := EvaluationReq{
		TaskName: "claw-job",
		TaskDesc: "desc",
		ApiKey:   "sk-test",
	}

	fields := req.GetSensitiveFields()
	require.Len(t, fields, 2)
	for _, field := range fields {
		require.NotEqual(t, "api_key", field.Name)
	}
}
