package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

const (
	gpuMemUtilH100    = 0.92
	gpuMemUtilA100    = 0.90
	maxBatchedH100    = 8192
	maxBatchedA100    = 8192
	maxBatchedH20     = 4096
	maxModelLenH100   = 16384
	maxModelLenA100   = 10240
	maxModelLenBase   = 5120
	memFractionH100   = 0.92
	memFractionA100   = 0.90
	multiNodeMinToken = 9016
)

var (
	vllmVerRegex   = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
	sglangVerRegex = regexp.MustCompile(`^[^0-9]*([0-9]+)\.([0-9]+)\.([0-9]+)`)
)

func (c *modelComponentImpl) OptimizeEngineArgs(ctx context.Context, namespace, name, currentUser string, req types.OptimizeEngineArgsReq) (*types.OptimizeEngineArgsResp, error) {
	frame, err := c.runtimeFrameworksStore.FindEnabledByID(ctx, req.RuntimeFrameworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to find runtime framework: %w", err)
	}

	resource, err := c.spaceResourceStore.FindByID(ctx, req.ResourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to find resource: %w", err)
	}

	var hardware types.HardWare
	if err := json.Unmarshal([]byte(resource.Resources), &hardware); err != nil {
		return nil, fmt.Errorf("failed to parse hardware resources: %w", err)
	}

	xpuModel := hardware.GetResXPUMode()
	gpuNum, err := strconv.Atoi(strings.TrimSpace(hardware.Gpu.Num))
	if err != nil || gpuNum < 1 {
		gpuNum = 1
	}
	multiNode := hardware.Replicas > 1
	gpuCount := gpuNum
	if multiNode {
		gpuCount = gpuNum * hardware.Replicas
	}

	repo, err := c.repoStore.FindByPath(ctx, types.ModelRepo, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model repo: %w", err)
	}
	maxPosEmb := c.readMaxPositionEmbeddings(ctx, namespace, name, repo.DefaultBranch)

	userArgs, err := common.JsonStrToMap(req.EngineArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse engine_args: %w", err)
	}

	frameNameLower := strings.ToLower(frame.FrameName)
	var optimized map[string]string

	switch {
	case strings.Contains(frameNameLower, "vllm"):
		optimized = optimizeVllmArgs(frame.FrameVersion, xpuModel, gpuCount, multiNode, req.EnablePD, maxPosEmb)
	case strings.Contains(frameNameLower, "sglang"):
		optimized = optimizeSglangArgs(frame.FrameVersion, xpuModel, gpuCount, multiNode, req.EnablePD, maxPosEmb)
	default:
		optimized = map[string]string{}
	}

	changed := make(map[string]string)
	for k, v := range optimized {
		userVal, exists := userArgs[k]
		if !exists || userVal != v {
			changed[k] = v
		}
	}

	return &types.OptimizeEngineArgsResp{EngineArgs: changed}, nil
}

func (c *modelComponentImpl) readMaxPositionEmbeddings(ctx context.Context, namespace, name, ref string) int {
	if ref == "" {
		ref = types.MainBranch
	}
	content, err := c.gitServer.GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: namespace,
		Name:      name,
		Ref:       ref,
		Path:      ConfigFileName,
		RepoType:  types.ModelRepo,
	})
	if err != nil {
		slog.Debug("failed to read config.json for max_position_embeddings",
			slog.String("namespace", namespace), slog.String("name", name), slog.Any("error", err))
		return 0
	}
	var cfg types.ModelConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		slog.Debug("failed to unmarshal config.json",
			slog.String("namespace", namespace), slog.String("name", name), slog.Any("error", err))
		return 0
	}
	return cfg.MaxPositionEmbeddings
}

func gpuTier(xpuModel string) string {
	lower := strings.ToLower(xpuModel)
	switch {
	case strings.Contains(lower, "h100") || strings.Contains(lower, "h200"):
		return "h100"
	case strings.Contains(lower, "a100"):
		return "a100"
	case strings.Contains(lower, "h20"):
		return "h20"
	default:
		return ""
	}
}

func vllmVersionGate(version string) bool {
	s := strings.TrimLeftFunc(version, func(r rune) bool {
		return r < '0' || r > '9'
	})
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		parts = parts[:2]
	}
	verNum := strings.Join(parts, ".")
	if !vllmVerRegex.MatchString(verNum) {
		return false
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	return major > 0 || (major == 0 && minor >= 10)
}

func sglangVersionGate(version string) bool {
	matches := sglangVerRegex.FindStringSubmatch(version)
	if matches == nil {
		return false
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	return major > 0 || minor > 4 || (minor == 4 && patch >= 4)
}

func optimizeVllmArgs(version, xpuModel string, gpuCount int, multiNode, isPD bool, maxPosEmb int) map[string]string {
	result := map[string]string{}

	if isPD {
		return result
	}

	if !vllmVersionGate(version) {
		return result
	}

	tier := gpuTier(xpuModel)
	if tier == "" {
		return result
	}

	switch tier {
	case "h100":
		result["gpu-memory-utilization"] = strconv.FormatFloat(gpuMemUtilH100, 'f', -1, 64)
		result["max-num-batched-tokens"] = strconv.Itoa(maxBatchedH100)
	case "a100":
		result["gpu-memory-utilization"] = strconv.FormatFloat(gpuMemUtilA100, 'f', -1, 64)
		result["max-num-batched-tokens"] = strconv.Itoa(maxBatchedA100)
	case "h20":
		result["gpu-memory-utilization"] = strconv.FormatFloat(gpuMemUtilA100, 'f', -1, 64)
		result["max-num-batched-tokens"] = strconv.Itoa(maxBatchedH20)
	}

	multiplier := maxModelLenBase
	switch tier {
	case "h100":
		multiplier = maxModelLenH100
	case "a100":
		multiplier = maxModelLenA100
	}

	if multiplier > maxModelLenBase {
		gpuLimit := gpuCount * multiplier
		if multiNode && gpuLimit < multiNodeMinToken {
			gpuLimit = multiNodeMinToken
		}
		if maxPosEmb > 0 && maxPosEmb < gpuLimit {
			result["max-model-len"] = strconv.Itoa(maxPosEmb)
		} else {
			result["max-model-len"] = strconv.Itoa(gpuLimit)
		}
	}

	return result
}

func optimizeSglangArgs(version, xpuModel string, gpuCount int, multiNode, isPD bool, maxPosEmb int) map[string]string {
	result := map[string]string{}

	if !sglangVersionGate(version) {
		return result
	}

	tier := gpuTier(xpuModel)
	if tier == "h20" || tier == "" {
		return result
	}

	switch tier {
	case "h100":
		result["mem-fraction-static"] = strconv.FormatFloat(memFractionH100, 'f', -1, 64)
	case "a100":
		result["mem-fraction-static"] = strconv.FormatFloat(memFractionA100, 'f', -1, 64)
	}

	if !multiNode && !isPD {
		multiplier := maxModelLenBase
		switch tier {
		case "h100":
			multiplier = maxModelLenH100
		case "a100":
			multiplier = maxModelLenA100
		}

		if multiplier > maxModelLenBase {
			gpuLimit := gpuCount * multiplier
			if maxPosEmb > 0 && maxPosEmb < gpuLimit {
				result["context-length"] = strconv.Itoa(maxPosEmb)
			} else {
				result["context-length"] = strconv.Itoa(gpuLimit)
			}
		}
	}

	return result
}
