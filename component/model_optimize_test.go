package component

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestVllmVersionGate(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"v0.10.0", true},
		{"v0.11.2", true},
		{"v0.9.2", false},
		{"v0.28.0", true},
		{"v0.24.0", true},
		{"25.11-py3", false},
		{"", false},
		{"abc", false},
	}
	for _, c := range cases {
		t.Run(c.version, func(t *testing.T) {
			require.Equal(t, c.want, vllmVersionGate(c.version))
		})
	}
}

func TestSglangVersionGate(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"v0.5.14", true},
		{"v0.5.12", true},
		{"v0.5.4", true},
		{"v0.5.3", true},
		{"v0.5.3rc0", true},
		{"v0.4.2", false},
		{"25.12-py3", false},
		{"", false},
		{"abc", false},
	}
	for _, c := range cases {
		t.Run(c.version, func(t *testing.T) {
			require.Equal(t, c.want, sglangVersionGate(c.version))
		})
	}
}

func TestGpuTier(t *testing.T) {
	cases := []struct {
		model string
		want  string
	}{
		{"NVIDIA-H100", "h100"},
		{"NVIDIA-H200", "h100"},
		{"NVIDIA-A100-SXM4-80GB", "a100"},
		{"NVIDIA-H20", "h20"},
		{"NVIDIA-RTX-4090", ""},
		{"", ""},
	}
	for _, c := range cases {
		t.Run(c.model, func(t *testing.T) {
			require.Equal(t, c.want, gpuTier(c.model))
		})
	}
}

func TestOptimizeVllmArgs(t *testing.T) {
	t.Run("H100 single node", func(t *testing.T) {
		got := optimizeVllmArgs("v0.10.0", "NVIDIA-H100", 8, false, false, 32768)
		require.Equal(t, "0.92", got["gpu-memory-utilization"])
		require.Equal(t, "8192", got["max-num-batched-tokens"])
		require.Equal(t, "32768", got["max-model-len"])
	})

	t.Run("H100 single node max_pos_emb larger than gpu limit", func(t *testing.T) {
		got := optimizeVllmArgs("v0.10.0", "NVIDIA-H100", 8, false, false, 200000)
		require.Equal(t, "131072", got["max-model-len"])
	})

	t.Run("H100 single node max_pos_emb unavailable", func(t *testing.T) {
		got := optimizeVllmArgs("v0.10.0", "NVIDIA-H100", 8, false, false, 0)
		require.Equal(t, "131072", got["max-model-len"])
	})

	t.Run("A100 single node", func(t *testing.T) {
		got := optimizeVllmArgs("v0.10.0", "NVIDIA-A100", 8, false, false, 32768)
		require.Equal(t, "0.9", got["gpu-memory-utilization"])
		require.Equal(t, "8192", got["max-num-batched-tokens"])
		require.Equal(t, "32768", got["max-model-len"])
	})

	t.Run("H20 single node", func(t *testing.T) {
		got := optimizeVllmArgs("v0.10.0", "NVIDIA-H20", 8, false, false, 32768)
		require.Equal(t, "0.9", got["gpu-memory-utilization"])
		require.Equal(t, "4096", got["max-num-batched-tokens"])
		_, ok := got["max-model-len"]
		require.False(t, ok)
	})

	t.Run("H100 multi node gpu limit below 9016 floor", func(t *testing.T) {
		got := optimizeVllmArgs("v0.10.0", "NVIDIA-H100", 1, true, false, 0)
		require.Equal(t, "16384", got["max-model-len"])
	})

	t.Run("H100 multi node gpu limit exceeds 9016", func(t *testing.T) {
		got := optimizeVllmArgs("v0.10.0", "NVIDIA-H100", 8, true, false, 0)
		require.Equal(t, "131072", got["max-model-len"])
	})

	t.Run("PD returns empty", func(t *testing.T) {
		got := optimizeVllmArgs("v0.10.0", "NVIDIA-H100", 8, false, true, 32768)
		require.Empty(t, got)
	})

	t.Run("old version returns empty", func(t *testing.T) {
		got := optimizeVllmArgs("v0.9.2", "NVIDIA-H100", 8, false, false, 32768)
		require.Empty(t, got)
	})

	t.Run("nvidia-vllm NGC version returns empty", func(t *testing.T) {
		got := optimizeVllmArgs("25.11-py3", "NVIDIA-H100", 8, false, false, 32768)
		require.Empty(t, got)
	})

	t.Run("unknown GPU returns empty", func(t *testing.T) {
		got := optimizeVllmArgs("v0.10.0", "Unknown-GPU", 8, false, false, 32768)
		require.Empty(t, got)
	})
}

func TestOptimizeSglangArgs(t *testing.T) {
	t.Run("H100 single node", func(t *testing.T) {
		got := optimizeSglangArgs("v0.5.14", "NVIDIA-H100", 8, false, false, 32768)
		require.Equal(t, "0.92", got["mem-fraction-static"])
		require.Equal(t, "32768", got["context-length"])
	})

	t.Run("A100 single node", func(t *testing.T) {
		got := optimizeSglangArgs("v0.5.14", "NVIDIA-A100", 8, false, false, 32768)
		require.Equal(t, "0.9", got["mem-fraction-static"])
		require.Equal(t, "32768", got["context-length"])
	})

	t.Run("H20 returns empty", func(t *testing.T) {
		got := optimizeSglangArgs("v0.5.14", "NVIDIA-H20", 8, false, false, 32768)
		require.Empty(t, got)
	})

	t.Run("multi node no context-length", func(t *testing.T) {
		got := optimizeSglangArgs("v0.5.14", "NVIDIA-H100", 8, true, false, 32768)
		require.Equal(t, "0.92", got["mem-fraction-static"])
		_, ok := got["context-length"]
		require.False(t, ok)
	})

	t.Run("PD no context-length", func(t *testing.T) {
		got := optimizeSglangArgs("v0.5.14", "NVIDIA-H100", 8, false, true, 32768)
		require.Equal(t, "0.92", got["mem-fraction-static"])
		_, ok := got["context-length"]
		require.False(t, ok)
	})

	t.Run("old version returns empty", func(t *testing.T) {
		got := optimizeSglangArgs("v0.4.2", "NVIDIA-H100", 8, false, false, 32768)
		require.Empty(t, got)
	})

	t.Run("nvidia-sglang NGC version returns empty", func(t *testing.T) {
		got := optimizeSglangArgs("25.12-py3", "NVIDIA-H100", 8, false, false, 32768)
		require.Empty(t, got)
	})

	t.Run("unknown GPU returns empty", func(t *testing.T) {
		got := optimizeSglangArgs("v0.5.14", "Unknown-GPU", 8, false, false, 32768)
		require.Empty(t, got)
	})
}

func TestOptimizeEngineArgs(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	hw := types.HardWare{
		Gpu: types.Processor{Type: "NVIDIA-H100", Num: "8"},
	}
	hwJSON, _ := json.Marshal(hw)

	mc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(1)).Return(
		&database.RuntimeFramework{
			ID:           1,
			FrameName:    "vllm",
			FrameVersion: "v0.10.0",
		}, nil,
	)
	mc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.SpaceResource{ID: 1, Resources: string(hwJSON)}, nil,
	)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "model").Return(
		&database.Repository{ID: 1, DefaultBranch: "main"}, nil,
	)
	mc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "model",
		Ref:       "main",
		Path:      ConfigFileName,
		RepoType:  types.ModelRepo,
	}).Return(`{"max_position_embeddings": 32768}`, nil)

	req := types.OptimizeEngineArgsReq{
		RuntimeFrameworkID: 1,
		ResourceID:         1,
		EngineArgs:         `{"gpu-memory-utilization":"0.85"}`,
		EnablePD:           false,
	}

	resp, err := mc.OptimizeEngineArgs(ctx, "ns", "model", "user", req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "0.92", resp.EngineArgs["gpu-memory-utilization"])
	require.Equal(t, "8192", resp.EngineArgs["max-num-batched-tokens"])
	require.Equal(t, "32768", resp.EngineArgs["max-model-len"])
}

func TestOptimizeEngineArgs_UserValueMatchesOmit(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	hw := types.HardWare{
		Gpu: types.Processor{Type: "NVIDIA-H100", Num: "8"},
	}
	hwJSON, _ := json.Marshal(hw)

	mc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(1)).Return(
		&database.RuntimeFramework{ID: 1, FrameName: "vllm", FrameVersion: "v0.10.0"}, nil,
	)
	mc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.SpaceResource{ID: 1, Resources: string(hwJSON)}, nil,
	)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "model").Return(
		&database.Repository{ID: 1, DefaultBranch: "main"}, nil,
	)
	mc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "model",
		Ref:       "main",
		Path:      ConfigFileName,
		RepoType:  types.ModelRepo,
	}).Return(`{"max_position_embeddings": 32768}`, nil)

	req := types.OptimizeEngineArgsReq{
		RuntimeFrameworkID: 1,
		ResourceID:         1,
		EngineArgs:         `{"gpu-memory-utilization":"0.92","max-num-batched-tokens":"8192","max-model-len":"32768"}`,
		EnablePD:           false,
	}

	resp, err := mc.OptimizeEngineArgs(ctx, "ns", "model", "user", req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.EngineArgs)
}

func TestOptimizeEngineArgs_UnknownFrameReturnsEmpty(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	hw := types.HardWare{
		Gpu: types.Processor{Type: "NVIDIA-H100", Num: "8"},
	}
	hwJSON, _ := json.Marshal(hw)

	mc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(1)).Return(
		&database.RuntimeFramework{ID: 1, FrameName: "tgi", FrameVersion: "1.0"}, nil,
	)
	mc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.SpaceResource{ID: 1, Resources: string(hwJSON)}, nil,
	)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "model").Return(
		&database.Repository{ID: 1, DefaultBranch: "main"}, nil,
	)
	mc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "model",
		Ref:       "main",
		Path:      ConfigFileName,
		RepoType:  types.ModelRepo,
	}).Return(`{"max_position_embeddings": 32768}`, nil)

	req := types.OptimizeEngineArgsReq{
		RuntimeFrameworkID: 1,
		ResourceID:         1,
		EngineArgs:         `{}`,
		EnablePD:           false,
	}

	resp, err := mc.OptimizeEngineArgs(ctx, "ns", "model", "user", req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.EngineArgs)
}
