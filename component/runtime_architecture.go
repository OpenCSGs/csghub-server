package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	gguf "github.com/gpustack/gguf-parser-go"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

var (
	MainBranch            string = "main"
	MasterBranch          string = "master"
	ConfigFileName        string = "config.json"
	ModelIndexFileName    string = "model_index.json"
	ModelSafetensorsIndex string = "model.safetensors.index.json"
	ScanLock              sync.Mutex
)

type runtimeArchitectureComponentImpl struct {
	repoComponent             RepoComponent
	repoStore                 database.RepoStore
	repoRuntimeFrameworkStore database.RepositoriesRuntimeFrameworkStore
	runtimeArchStore          database.RuntimeArchitecturesStore
	runtimeFrameworksStore    database.RuntimeFrameworksStore
	tagStore                  database.TagStore
	resouceModelStore         database.ResourceModelStore
	metadataStore             database.MetadataStore
	gitServer                 gitserver.GitServer
	cache                     cache.RedisClient
	fileDownloadPath          string
	apiToken                  string
	config                    *config.Config
}

type RuntimeArchitectureComponent interface {
	ListByRuntimeFrameworkID(ctx context.Context, id int64) ([]database.RuntimeArchitecture, error)
	SetArchitectures(ctx context.Context, id int64, architectures []string) ([]string, error)
	DeleteArchitectures(ctx context.Context, id int64, architectures []string) ([]string, error)
	ScanAllModels(ctx context.Context, scanType int) error
	ScanModel(ctx context.Context, currentUser, namespace, name string) error
	UpdateModelMetadata(ctx context.Context, repo *database.Repository) (*types.ModelInfo, error)
	// add runtime_framework tag to model
	AddRuntimeFrameworkTag(ctx context.Context, rftags []*database.Tag, repoId, rfId int64) error
	// update runtime_framework tag to model
	UpdateRuntimeFrameworkTag(ctx context.Context, modelInfo *types.ModelInfo, repo *database.Repository) error
	// add resource tag to model
	AddResourceTag(ctx context.Context, rstags []*database.Tag, modelname string, repoId int64) error
	InitRuntimeFrameworkAndArchitectures() error
}

func NewRuntimeArchitectureComponent(config *config.Config) (RuntimeArchitectureComponent, error) {
	c := &runtimeArchitectureComponentImpl{}
	c.runtimeFrameworksStore = database.NewRuntimeFrameworksStore()
	c.runtimeArchStore = database.NewRuntimeArchitecturesStore()
	c.tagStore = database.NewTagStore()
	c.resouceModelStore = database.NewResourceModelStore()
	c.metadataStore = database.NewMetadataStore()
	repo, err := NewRepoComponentImpl(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create repo component, %w", err)
	}
	c.repoComponent = repo
	c.repoStore = database.NewRepoStore()
	c.repoRuntimeFrameworkStore = database.NewRepositoriesRuntimeFramework()
	c.gitServer, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	cache, err := cache.NewCache(context.Background(), cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create redis cache, error: %w", err)
	}
	c.fileDownloadPath = fmt.Sprintf("%s/%s", config.Model.DownloadEndpoint, "csg")
	c.apiToken = config.APIToken
	c.cache = cache
	c.config = config
	return c, nil
}

func (c *runtimeArchitectureComponentImpl) ListByRuntimeFrameworkID(ctx context.Context, id int64) ([]database.RuntimeArchitecture, error) {
	archs, err := c.runtimeArchStore.ListByRuntimeFrameworkID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list runtime arch failed, %w", err)
	}
	return archs, nil
}

func (c *runtimeArchitectureComponentImpl) SetArchitectures(ctx context.Context, id int64, architectures []string) ([]string, error) {
	_, err := c.runtimeFrameworksStore.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("invalid runtime framework id, %w", err)
	}
	var failedArchs []string
	for _, arch := range architectures {
		if len(strings.Trim(arch, " ")) < 1 {
			continue
		}
		err := c.runtimeArchStore.Add(ctx, database.RuntimeArchitecture{
			RuntimeFrameworkID: id,
			ArchitectureName:   strings.Trim(arch, " "),
		})
		if err != nil {
			failedArchs = append(failedArchs, arch)
		}
	}
	return failedArchs, nil
}

func (c *runtimeArchitectureComponentImpl) DeleteArchitectures(ctx context.Context, id int64, architectures []string) ([]string, error) {
	_, err := c.runtimeFrameworksStore.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("invalid runtime framework id, %w", err)
	}
	var failedDeletes []string
	for _, arch := range architectures {
		if len(strings.Trim(arch, " ")) < 1 {
			continue
		}
		err := c.runtimeArchStore.DeleteByRuntimeIDAndArchName(ctx, id, strings.Trim(arch, " "))
		if err != nil {
			failedDeletes = append(failedDeletes, arch)
		}
	}
	return failedDeletes, nil
}

func (c *runtimeArchitectureComponentImpl) ScanModel(ctx context.Context, currentUser, namespace, name string) error {
	repo, err := c.repoStore.FindByPath(ctx, types.ModelRepo, namespace, name)
	if err != nil {
		return fmt.Errorf("fail to find repository by namespace and name, %w", err)
	}
	permission, err := c.repoComponent.GetUserRepoPermission(ctx, currentUser, repo)
	if err != nil {
		return fmt.Errorf("fail to get user permission for repository, %w", err)
	}
	if !permission.CanWrite {
		return errorx.ErrForbiddenMsg(fmt.Sprintf("user %s does not have permission to update metadata for %s ", currentUser, repo.Path))
	}
	modelInfo, err := c.UpdateModelMetadata(ctx, repo)
	if err != nil {
		return fmt.Errorf("fail to update model metadata, %w", err)
	}
	err = c.UpdateRuntimeFrameworkTag(ctx, modelInfo, repo)
	if err != nil {
		return fmt.Errorf("fail to update runtime framework tag, %w", err)
	}
	return nil
}

// scanType: 0-all, 1-new
func (c *runtimeArchitectureComponentImpl) ScanAllModels(ctx context.Context, scanType int) error {
	deadline, ok := ctx.Deadline()
	durationUntilDeadline := 2 * time.Hour
	if ok {
		durationUntilDeadline = time.Until(deadline)
	}
	err := c.cache.RunWhileLocked(ctx, "runtime_architecture_scan_lock", durationUntilDeadline, func(ctxLock context.Context) error {
		var i int
		for {
			ctxBatch, cancel := context.WithTimeout(ctxLock, 1*time.Hour)
			repos, err := c.repoStore.FindWithBatch(ctxBatch, 500, i, types.ModelRepo)
			i += 1
			if err != nil {
				slog.Error("fail to batch get repositories for scanning all models", slog.Any("err", err))
			}
			if len(repos) == 0 {
				cancel()
				break
			}
			for _, repo := range repos {
				if repo.Metadata.TensorType != "" && scanType == 1 {
					// skip scanned models
					continue
				}
				modelInfo, err := c.UpdateModelMetadata(ctxBatch, &repo)
				if err != nil {
					slog.Error("fail to update model metadata", slog.Any("err", err), slog.Any("repo path", repo.Path))
					continue
				}
				err = c.UpdateRuntimeFrameworkTag(ctxBatch, modelInfo, &repo)
				if err != nil {
					slog.Error("fail to update runtime framework tag", slog.Any("err", err), slog.Any("repo path", repo.Path))
				}
			}
			slog.Info("scanned repositories for model metadata", slog.Any("count", len(repos)))
			cancel()
		}
		slog.Info("scanned all repositories for model metadata")
		return nil
	})
	if err != nil {
		slog.Warn("architecture scan is already in progress")
		return fmt.Errorf("architecture scan is already in progress")
	}
	return nil
}

// update model metadata
func (c *runtimeArchitectureComponentImpl) UpdateModelMetadata(ctx context.Context, repo *database.Repository) (*types.ModelInfo, error) {
	var modelInfo *types.ModelInfo
	var err error
	modelFormat := repo.Format()

	switch modelFormat {
	case string(types.Safetensors):
		modelInfo, err = c.GetMetadataFromSafetensors(ctx, repo)
	case string(types.GGUF):
		modelInfo, err = c.GetMetadataFromGGUF(ctx, repo)
	default:
		return nil, fmt.Errorf("unsupported model format %s", modelFormat)
	}
	if err != nil {
		return nil, fmt.Errorf("fail to get model metadata from %s, %w", modelFormat, err)
	}
	metadata := &database.Metadata{
		RepositoryID:      repo.ID,
		ModelParams:       modelInfo.ParamsBillions,
		TensorType:        modelInfo.TensorType,
		MiniGPUMemoryGB:   modelInfo.MiniGPUMemoryGB,
		MiniGPUFinetuneGB: modelInfo.MiniGPUFinetuneGB,
		Architecture:      modelInfo.Architecture,
		ModelType:         modelInfo.ModelType,
		ClassName:         modelInfo.ClassName,
		Quantizations:     modelInfo.Quantizations,
	}
	err = c.metadataStore.Upsert(ctx, metadata)
	if err != nil {
		return nil, fmt.Errorf("fail to update model metadata in db, %w", err)
	}
	slog.Info("updated model metadata successfully", slog.Any("repo path", repo.Path), slog.Any("model info", metadata))
	return modelInfo, nil
}

// UpdateRuntimeFrameworkTag
func (c *runtimeArchitectureComponentImpl) UpdateRuntimeFrameworkTag(ctx context.Context, modelInfo *types.ModelInfo, repo *database.Repository) error {
	filter := &types.TagFilter{
		Scopes:     []types.TagScope{types.ModelTagScope},
		Categories: []string{"runtime_framework", "resource"},
	}
	runtime_framework_tags, _ := c.tagStore.AllTags(ctx, filter)
	var archs []string
	if modelInfo.Architecture != "" {
		archs = append(archs, modelInfo.Architecture)
	}
	if modelInfo.ClassName != "" {
		archs = append(archs, modelInfo.ClassName)
	}
	if modelInfo.ModelType != "" {
		archs = append(archs, modelInfo.ModelType)
	}
	if len(archs) == 0 {
		return fmt.Errorf("fail to get architecture from model info")
	}
	modelFormat := repo.Format()
	newFrames, err := c.getRuntimeFrameworks(ctx, archs, *repo, types.ModelType(modelFormat))
	if err != nil || len(newFrames) == 0 {
		return fmt.Errorf("fail to get runtime frameworks for %s, %w", archs, err)
	}
	// clean old runtime tags
	err = c.tagStore.RemoveRepoTagsByCategory(ctx, repo.ID, []string{"runtime_framework", "resource"})
	if err != nil {
		return fmt.Errorf("fail to remove old runtime framework tags, %w", err)
	}
	// add new tags
	for _, new := range newFrames {
		// add runtime framework and resource tags
		err = c.AddRuntimeFrameworkTag(ctx, runtime_framework_tags, repo.ID, new.ID)
		if err != nil {
			slog.Warn("fail to add runtime framework tag", slog.Any("repo.ID", repo.ID), slog.Any("runtime framework name", new.FrameName), slog.Any("error", err))
		}
	}
	return nil
}

func (c *runtimeArchitectureComponentImpl) getRuntimeFrameworks(ctx context.Context, arch []string, repo database.Repository, modelType types.ModelType) ([]database.RuntimeFramework, error) {
	repo.NamespaceAndName()
	oriName := repo.OriginName()
	runtimes, err := c.runtimeArchStore.ListByArchNameAndModel(ctx, arch, oriName)
	// to do check resource models
	if err != nil {
		slog.Warn("fail to get runtime frameworks for git callback", slog.Any("arch", arch), slog.Any("error", err))
		return nil, err
	}
	var frameIDs []int64
	for _, runtime := range runtimes {
		frameIDs = append(frameIDs, runtime.RuntimeFrameworkID)
	}

	newFrames, err := c.runtimeFrameworksStore.ListByIDs(ctx, frameIDs)
	if err != nil {
		slog.Warn("fail to get runtime frameworks for git callback", slog.Any("arch", arch), slog.Any("error", err))
		return nil, err
	}
	var frames []database.RuntimeFramework
	for _, frame := range newFrames {
		supportedFormat := string(types.Safetensors)
		if frame.ModelFormat != "" {
			supportedFormat = frame.ModelFormat
		}
		if !strings.Contains(supportedFormat, string(modelType)) {
			continue
		}
		frames = append(frames, frame)

	}
	return frames, nil
}

// check model framework
func isGGUFModel(repo *database.Repository) bool {
	for _, tag := range repo.Tags {
		if tag.Name == "gguf" {
			return true
		}
	}
	return strings.Contains(strings.ToLower(repo.Name), "gguf")
}

// for text-generation
func (c *runtimeArchitectureComponentImpl) GetArchitectureFromConfig(ctx context.Context, repo *database.Repository) (*types.ModelConfig, error) {
	content, err := c.getConfigContent(ctx, ConfigFileName, repo)
	if err != nil {
		return nil, fmt.Errorf("fail to read config.json for relation, %w", err)
	}
	config := &types.ModelConfig{}
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("fail to unmarshal config, %w", err)
	}
	slog.Debug("unmarshal config", slog.Any("config", config))
	if config.Architectures == nil {
		return nil, nil
	}
	if len(config.Architectures) < 1 {
		return nil, nil
	}
	return config, nil
}

func (c *runtimeArchitectureComponentImpl) GetModelTypeFromConfig(ctx context.Context, repo *database.Repository) (string, error) {
	content, err := c.getConfigContent(ctx, ConfigFileName, repo)
	if err != nil {
		return "", fmt.Errorf("fail to read config.json for relation, %w", err)
	}
	var config struct {
		ModelType string `json:"model_type"`
	}
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		return "", fmt.Errorf("fail to unmarshal config, %w", err)
	}
	slog.Debug("unmarshal config", slog.Any("config", config))
	if len(config.ModelType) < 1 {
		return "", nil
	}
	slog.Debug("model type of config", slog.Any("Architectures", config.ModelType))
	return config.ModelType, nil
}

// for text-to-image
func (c *runtimeArchitectureComponentImpl) GetClassNameFromConfig(ctx context.Context, repo *database.Repository) (string, error) {
	content, err := c.getConfigContent(ctx, ModelIndexFileName, repo)
	if err != nil {
		return "", fmt.Errorf("fail to read model_index.json for relation, %w", err)
	}
	var config struct {
		ClassName string `json:"_class_name"`
	}
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		return "", fmt.Errorf("fail to unmarshal config, %w", err)
	}
	slog.Debug("unmarshal config", slog.Any("config", config))
	if config.ClassName == "" {
		return "", nil
	}
	slog.Debug("ClassName of mode index json", slog.Any("ClassName", config.ClassName))
	return config.ClassName, nil
}

func (c *runtimeArchitectureComponentImpl) GetMetadataFromGGUF(ctx context.Context, repo *database.Repository) (*types.ModelInfo, error) {
	namespace, name := repo.NamespaceAndName()
	files, err := getAllFiles(ctx, namespace, name, "", types.ModelRepo, repo.DefaultBranch, c.gitServer.GetTree)
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("get RepoFileTree for relation, %w", err)
	}
	modelInfo := &types.ModelInfo{}
	hasGGuf := false
	for _, file := range files {
		ext := filepath.Ext(file.Name)
		if ext == ".gguf" {
			if strings.Contains(file.Name, "00001-of-") || !strings.Contains(file.Name, "-of-") {
				hasGGuf = true
				fs, err := c.GetGGUFContent(ctx, file.Path, repo)
				if err != nil {
					return nil, fmt.Errorf("fail to get main gguf content, %w", err)
				}
				metadata := fs.Metadata()
				var opts []gguf.GGUFRunEstimateOption
				opts = append(opts, gguf.WithLLaMACppContextSize(int32(c.config.Model.MinContextForEstimation)))
				opts = append(opts, gguf.WithParallelSize(int32(1)))
				lme := fs.EstimateLLaMACppRun(opts...)
				emi := lme.SummarizeItem(true, 2*1024*1024, 2*1024*1024)
				modelInfo.ParamsBillions = float32(math.Round(float64(metadata.Parameters)/1e9*100) / 100)
				modelInfo.Architecture = metadata.Architecture
				quantization := types.Quantization{
					VERSION: c.GetBitFromFileType(metadata.FileType.String()),
					TYPE:    metadata.FileTypeDescriptor,
				}
				if len(emi.VRAMs) > 0 {
					quantization.MiniGPUMemoryGB = max(float32(emi.VRAMs[0].NonUMA/(1024*1024*1024)), 1)
				}
				modelInfo.Quantizations = append(modelInfo.Quantizations, quantization)
			}
		}
	}
	if !hasGGuf {
		return nil, fmt.Errorf("no gguf file found")
	}
	return modelInfo, nil
}

func (c *runtimeArchitectureComponentImpl) GetMetadataFromSafetensors(ctx context.Context, repo *database.Repository) (*types.ModelInfo, error) {
	namespace, name := repo.NamespaceAndName()
	files, err := getAllFiles(ctx, namespace, name, "", types.ModelRepo, repo.DefaultBranch, c.gitServer.GetTree)
	if err != nil {
		return nil, fmt.Errorf("get RepoFileTree for relation, %w", err)
	}
	fileUrls := make([]string, 0)
	var hasConfig bool
	var hasModelIndex bool
	for _, file := range files {
		if file.Path == ConfigFileName {
			hasConfig = true
		}
		if file.Path == ModelIndexFileName {
			hasModelIndex = true
		}
		ext := filepath.Ext(file.Name)
		if ext == ".safetensors" {
			req := types.GetFileReq{
				Lfs:       true,
				Namespace: namespace,
				Name:      name,
				Path:      file.Path,
				Ref:       repo.DefaultBranch,
				RepoType:  types.ModelRepo,
			}
			_, _, url, err := c.repoComponent.InternalDownloadFile(ctx, &req)
			if err != nil {
				return nil, fmt.Errorf("fail to get safetensors file url, %w", err)
			}
			fileUrls = append(fileUrls, url)
		}
	}
	if len(fileUrls) == 0 {
		return nil, fmt.Errorf("no safetensors file found")
	}
	modelInfo := &types.ModelInfo{}
	//check files contains config.json
	if hasConfig {
		modelInfo, err := common.GetModelInfo(fileUrls, c.config.Model.MinContextForEstimation)
		if err != nil {
			slog.Error("fail to get model info from safetensors", slog.Any("err", err), slog.Any("repo", repo.Path))
			return nil, fmt.Errorf("fail to get model info from safetensors, %w, repo: %v", err, repo.Path)
		}
		config, err := c.GetArchitectureFromConfig(ctx, repo)
		if err != nil || config == nil {
			slog.Error("fail to get architecture from config", slog.Any("err", err))
			return nil, fmt.Errorf("fail to get architecture from config, %w", err)
		}
		if len(config.Architectures) > 0 {
			modelInfo.Architecture = config.Architectures[0]
		}
		modelInfo.ModelType = config.ModelType
		modelInfo.NumHiddenLayers = config.NumHiddenLayers
		modelInfo.HiddenSize = config.HiddenSize
		modelInfo.NumAttentionHeads = config.NumAttentionHeads
		if modelInfo.HiddenSize != 0 {
			kvcacheSize := common.GetKvCacheSize(modelInfo.ContextSize, modelInfo.BatchSize, modelInfo.HiddenSize, modelInfo.NumHiddenLayers, modelInfo.BytesPerParam)
			activateMemory := common.GetActivationMemory(modelInfo.BatchSize, modelInfo.ContextSize, modelInfo.NumHiddenLayers, modelInfo.HiddenSize, modelInfo.NumAttentionHeads, modelInfo.BytesPerParam)
			modelInfo.MiniGPUMemoryGB = float32(math.Round(float64(kvcacheSize+modelInfo.ModelWeightsGB+activateMemory)*100)) / 100

			defaultLoRArank := 16
			defaultBatchSize := 16
			modelInfo.MiniGPUFinetuneGB = common.GetLoRAFinetuneMemory(
				modelInfo.ModelWeightsGB,
				modelInfo.ParamsBillions*1e9,
				defaultBatchSize,
				c.config.Model.MinContextForFinetune,
				modelInfo.HiddenSize,
				modelInfo.NumHiddenLayers,
				modelInfo.NumAttentionHeads,
				modelInfo.BytesPerParam,
				defaultLoRArank,
			)
		}
		return modelInfo, nil
	}
	if hasModelIndex {
		className, err := c.GetClassNameFromConfig(ctx, repo)
		if err != nil {
			slog.Error("fail to get class name from model_index.json", slog.Any("err", err))
			return nil, fmt.Errorf("fail to get class name from model_index.json, %w", err)
		}
		modelInfo.ClassName = className
		return modelInfo, nil
	}

	return nil, fmt.Errorf("no model_index.json or config.json found, repo: %v", repo.Path)
}

func (c *runtimeArchitectureComponentImpl) getConfigContent(ctx context.Context, configFileName string, repo *database.Repository) (string, error) {
	namespace, name := repo.NamespaceAndName()
	content, err := c.gitServer.GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: namespace,
		Name:      name,
		Ref:       repo.DefaultBranch,
		Path:      configFileName,
		RepoType:  types.ModelRepo,
	})
	if err != nil {
		return "", fmt.Errorf("get RepoFileRaw for relation, %w", err)
	}
	return content, nil
}

// get gguf content
func (c *runtimeArchitectureComponentImpl) GetGGUFContent(ctx context.Context, filename string, repo *database.Repository) (*gguf.GGUFFile, error) {
	var options []gguf.GGUFReadOption
	options = append(options, gguf.SkipRangeDownloadDetection(), gguf.SkipTLSVerification())
	namespace, name := repo.NamespaceAndName()
	req := types.GetFileReq{
		Lfs:       true,
		Namespace: namespace,
		Name:      name,
		Path:      filename,
		Ref:       repo.DefaultBranch,
		RepoType:  types.ModelRepo,
	}
	_, _, url, err := c.repoComponent.InternalDownloadFile(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("fail to get gguf file url, %w", err)
	}

	f, err := gguf.ParseGGUFFileRemote(ctx, url, options...)
	if err != nil {
		return nil, fmt.Errorf("fail to parse gguf file, %w, url: %v", err, url)
	}
	return f, nil
}

// add runtime_framework tag to model
func (c *runtimeArchitectureComponentImpl) AddRuntimeFrameworkTag(ctx context.Context, rftags []*database.Tag, repoId, rfId int64) error {
	rfw, err := c.runtimeFrameworksStore.FindByID(ctx, rfId)
	if err != nil {
		return err
	}
	for _, tag := range rftags {
		if checkTagName(rfw, tag.Name) {
			err := c.tagStore.UpsertRepoTags(ctx, repoId, []int64{}, []int64{tag.ID})
			if err != nil {
				slog.Error("fail to add runtime_framework tag to model repo", slog.Any("repoId", repoId), slog.Any("runtime_framework_id", rfId), slog.Any("error", err))
			}
		}
	}
	return nil
}

// add resource tag to model
func (c *runtimeArchitectureComponentImpl) AddResourceTag(ctx context.Context, rstags []*database.Tag, modelname string, repoId int64) error {
	rms, err := c.resouceModelStore.FindByModelName(ctx, modelname)
	if err != nil {
		return err
	}
	for _, rm := range rms {
		for _, tag := range rstags {
			if strings.Contains(rm.ResourceName, tag.Name) {
				err := c.tagStore.UpsertRepoTags(ctx, repoId, []int64{}, []int64{tag.ID})
				if err != nil {
					slog.Warn("fail to add resource tag to model repo", slog.Any("repoId", repoId), slog.Any("error", err))
				}
			}
		}

	}
	return nil
}

// get bit version from file type
func (c *runtimeArchitectureComponentImpl) GetBitFromFileType(fileType string) string {
	if strings.Contains(fileType, "/") {
		fileType = strings.Split(fileType, "/")[1]
	}
	switch {
	case strings.Contains(fileType, "Q1_"):
		return "1-bit"
	case strings.Contains(fileType, "Q2_"):
		return "2-bit"
	case strings.Contains(fileType, "Q3_"):
		return "3-bit"
	case strings.Contains(fileType, "Q4_"):
		return "4-bit"
	case strings.Contains(fileType, "Q5_"):
		return "5-bit"
	case strings.Contains(fileType, "Q6_"):
		return "6-bit"
	case strings.Contains(fileType, "Q8_"):
		return "8-bit"
	case strings.Contains(fileType, "16"):
		return "16-bit"
	case strings.Contains(fileType, "32"):
		return "32-bit"
	case strings.Contains(fileType, "64"):
		return "64-bit"
	default:
		return "unknown"
	}
}

// Init runtimeFramework and architecture, triggered on startup
// if the updated time is different, update the database
func (c *runtimeArchitectureComponentImpl) InitRuntimeFrameworkAndArchitectures() error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	err := c.UpdateRuntimeFrameworkByType(ctx, types.InferenceType)
	if err != nil {
		return fmt.Errorf("failed to update inference runtime_framework: %w", err)
	}
	err = c.UpdateRuntimeFrameworkByType(ctx, types.FinetuneType)
	if err != nil {
		return fmt.Errorf("failed to update inference runtime_framework: %w", err)
	}
	err = c.UpdateRuntimeFrameworkByType(ctx, types.EvaluationType)
	if err != nil {
		return fmt.Errorf("failed to update inference runtime_framework: %w", err)
	}
	err = c.UpdateRuntimeFrameworkByType(ctx, types.NotebookType)
	if err != nil {
		return fmt.Errorf("failed to update inference runtime_framework: %w", err)
	}
	return nil
}

// update by engine type
func (c *runtimeArchitectureComponentImpl) UpdateRuntimeFrameworkByType(ctx context.Context, engineType int) error {
	var jsonFiles []string
	var err error
	switch engineType {
	case types.InferenceType:
		jsonFiles, err = getJsonfiles("inference")
		if err != nil {
			return fmt.Errorf("failed to get json files: %w", err)
		}
	case types.FinetuneType:
		jsonFiles, err = getJsonfiles("finetune")
		if err != nil {
			return fmt.Errorf("failed to get json files: %w", err)
		}
	case types.EvaluationType:
		jsonFiles, err = getJsonfiles("evaluation")
		if err != nil {
			return fmt.Errorf("failed to get json files: %w", err)
		}
	case types.NotebookType:
		jsonFiles, err = getJsonfiles("notebook")
		if err != nil {
			return fmt.Errorf("failed to get json files: %w", err)
		}
	}

	for _, filePath := range jsonFiles {
		// Read the JSON file
		jsonData, err := os.ReadFile(filePath)
		if err != nil {
			slog.Error("failed to read json file", slog.Any("file", filePath), slog.Any("error", err))
		}
		// Parse the JSON data into the EngineConfig struct
		var engineConfig types.EngineConfig
		err = json.Unmarshal(jsonData, &engineConfig)
		if err != nil {
			slog.Error("failed to unmarshal json file", slog.Any("file", filePath), slog.Any("error", err))
			continue
		}
		//get file modified time
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			slog.Error("failed to get file info", slog.Any("file", filePath), slog.Any("error", err))
			continue
		}
		if engineConfig.MinVersion != "" {
			err = c.removeFrameAndArchByVersion(ctx, engineConfig.EngineName, engineConfig.MinVersion)
			if err != nil {
				slog.Error("failed to remove runtime_framework and archs", slog.Any("file", filePath), slog.Any("error", err))
				continue
			}
		}
		engineConfig.UpdatedAt = fileInfo.ModTime()
		err = c.UpdateRuntimeFrameworkAndArch(ctx, engineType, engineConfig)
		if err != nil {
			slog.Error("failed to update runtime_framework and archs", slog.Any("file", filePath), slog.Any("error", err))
		}

	}
	return nil
}

// update runtime_framework if the updated time is different
func (c *runtimeArchitectureComponentImpl) UpdateRuntimeFrameworkAndArch(ctx context.Context, engineType int, engineConfig types.EngineConfig) error {
	for _, image := range engineConfig.EngineImages {
		rf, err := c.runtimeFrameworksStore.FindByFrameImageAndComputeType(ctx, image.Image, string(image.ComputeType))
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				slog.Error("failed to find runtime_framework", slog.Any("error", err))
				continue
			}
		}
		//check update time
		if !engineConfig.UpdatedAt.After(rf.UpdatedAt) {
			continue
		}
		rf.FrameName = engineConfig.EngineName
		rf.ComputeType = string(image.ComputeType)
		rf.ContainerPort = engineConfig.ContainerPort
		rf.FrameVersion = image.EngineVersion
		if engineConfig.EngineArgs != nil {
			args, err := json.Marshal(engineConfig.EngineArgs)
			if err != nil {
				return fmt.Errorf("failed to marshal engine args: %w", err)
			}
			rf.EngineArgs = string(args)
		}
		rf.Description = engineConfig.Description
		rf.ModelFormat = engineConfig.ModelFormat
		rf.Type = engineType
		rf.Enabled = engineConfig.Enabled
		rf.FrameImage = image.Image
		rf.DriverVersion = image.DriverVersion
		rf.ModelFormat = engineConfig.ModelFormat

		if rf.ID == 0 {
			nf, err := c.runtimeFrameworksStore.Add(ctx, *rf)
			if err != nil {
				slog.Error("failed to add runtime_framework", slog.Any("error", err), slog.String("engine", engineConfig.EngineName))
				continue
			}
			rf.ID = nf.ID
		} else {
			_, err = c.runtimeFrameworksStore.Update(ctx, *rf)
			if err != nil {
				slog.Error("failed to update runtime_framework", slog.Any("error", err), slog.String("engine", engineConfig.EngineName))
				continue
			}
			//update architectures
			err = c.runtimeArchStore.DeleteByRuntimeID(ctx, rf.ID)
			if err != nil {
				slog.Error("failed to delete runtime_architectures", slog.Any("error", err), slog.String("engine", engineConfig.EngineName))
				continue
			}

		}
		var archs []database.RuntimeArchitecture
		archMap := make(map[string]bool)
		engineArchs := append(engineConfig.SupportedArchs, image.ExtraArchs...)
		for _, arch := range engineArchs {
			//check duplicate arch in archs
			if _, exists := archMap[arch]; !exists {
				// If it doesn't exist, add it to the slice and the map
				archs = append(archs, database.RuntimeArchitecture{
					RuntimeFrameworkID: rf.ID,
					ArchitectureName:   arch,
				})
				archMap[arch] = true
			}
		}
		engineModels := append(engineConfig.SupportedModels, image.ExtraModels...)
		for _, name := range engineModels {
			if _, exists := archMap[name]; !exists {
				// If it doesn't exist, add it to the slice and the map
				archs = append(archs, database.RuntimeArchitecture{
					RuntimeFrameworkID: rf.ID,
					ModelName:          name,
				})
				archMap[name] = true
			}
		}
		if len(archs) == 0 {
			slog.Warn("no architectures found for runtime_framework", slog.String("engine", engineConfig.EngineName), slog.String("image", image.Image))
			continue
		}
		err = c.runtimeArchStore.BatchAdd(ctx, archs)
		if err != nil {
			slog.Error("failed to add runtime_architectures", slog.Any("error", err))
			continue
		}
		slog.Info("successfully updated runtime_framework", slog.String("engine", engineConfig.EngineName), slog.String("image", image.Image))

	}

	return nil
}

func (c *runtimeArchitectureComponentImpl) removeFrameAndArchByVersion(ctx context.Context, frameName, minVersion string) error {
	frames, err := c.runtimeFrameworksStore.FindByFrameName(ctx, frameName)
	if err != nil {
		return err
	}
	for _, frame := range frames {
		frameVer, err := semver.Make(frame.FrameVersion)
		if err != nil {
			slog.Error("failed to make semver", slog.Any("version", frameVer), slog.Any("error", err))
			continue
		}
		minVer, err := semver.Make(minVersion)
		if err != nil {
			slog.Error("failed to make semver", slog.Any("version", minVer), slog.Any("error", err))
			continue
		}
		if frameVer.LT(minVer) {
			err = c.runtimeFrameworksStore.RemoveRuntimeFrameworkAndArch(ctx, frame.ID)
			if err != nil {
				return err
			}
			slog.Info("removed runtime_framework and arch as the version is not the minimal version", slog.Int64("runtime_framework_id", frame.ID))

		}
	}
	return nil
}

func getJsonfiles(subPath string) (list []string, err error) {
	currentDir, err := filepath.Abs(filepath.Dir("."))
	if err != nil {
		return nil, fmt.Errorf("getting current directory error: %w", err)
	}
	// replace cmd/csghub-server
	currentDir = strings.Replace(currentDir, "cmd/csghub-server", "", 1)
	enginePath := filepath.Join(currentDir, "configs", subPath)
	_, err = os.Stat(enginePath)
	if err != nil {
		return nil, fmt.Errorf("get engine path %s error: %w", enginePath, err)
	}
	// get all json files in enginePath
	entries, err := os.ReadDir(enginePath)
	if err != nil {
		return nil, fmt.Errorf("read dir %s error: %w", enginePath, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".json" {
			list = append(list, filepath.Join(enginePath, entry.Name()))
		}
	}

	return list, nil
}
