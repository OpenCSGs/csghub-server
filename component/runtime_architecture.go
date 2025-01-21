package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var (
	MainBranch         string = "main"
	MasterBranch       string = "master"
	ConfigFileName     string = "config.json"
	ModelIndexFileName string = "model_index.json"
	ScanLock           sync.Mutex
)

type runtimeArchitectureComponentImpl struct {
	repoComponent             RepoComponent
	repoStore                 database.RepoStore
	repoRuntimeFrameworkStore database.RepositoriesRuntimeFrameworkStore
	runtimeArchStore          database.RuntimeArchitecturesStore
	runtimeFrameworksStore    database.RuntimeFrameworksStore
	tagStore                  database.TagStore
	resouceModelStore         database.ResourceModelStore
	gitServer                 gitserver.GitServer
	cache                     cache.RedisClient
}

type RuntimeArchitectureComponent interface {
	ListByRuntimeFrameworkID(ctx context.Context, id int64) ([]database.RuntimeArchitecture, error)
	SetArchitectures(ctx context.Context, id int64, architectures []string) ([]string, error)
	DeleteArchitectures(ctx context.Context, id int64, architectures []string) ([]string, error)
	ScanArchitecture(ctx context.Context, id int64, scanType int, task types.PipelineTask, models []string) error
	// check if it's supported model resource by name
	IsSupportedModelResource(ctx context.Context, modelName string, rfm *database.RuntimeFramework, id int64) (bool, error)
	GetArchitecture(ctx context.Context, task types.PipelineTask, repo *database.Repository) (string, error)
	// remove runtime_framework tag from model
	RemoveRuntimeFrameworkTag(ctx context.Context, rftags []*database.Tag, repoId, rfId int64)
	// add runtime_framework tag to model
	AddRuntimeFrameworkTag(ctx context.Context, rftags []*database.Tag, repoId, rfId int64) error
	// add resource tag to model
	AddResourceTag(ctx context.Context, rstags []*database.Tag, modelname string, repoId int64) error
}

func NewRuntimeArchitectureComponent(config *config.Config) (RuntimeArchitectureComponent, error) {
	c := &runtimeArchitectureComponentImpl{}
	c.runtimeFrameworksStore = database.NewRuntimeFrameworksStore()
	c.runtimeArchStore = database.NewRuntimeArchitecturesStore()
	c.tagStore = database.NewTagStore()
	c.resouceModelStore = database.NewResourceModelStore()
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
	c.cache = cache
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

func (c *runtimeArchitectureComponentImpl) ScanArchitecture(ctx context.Context, id int64, scanType int, task types.PipelineTask, models []string) error {
	frame, err := c.runtimeFrameworksStore.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invalid runtime framework id, %w", err)
	}
	archs, err := c.runtimeArchStore.ListByRuntimeFrameworkID(ctx, id)
	if err != nil {
		return fmt.Errorf("list runtime arch failed, %w", err)
	}
	var archMap = make(map[string]string)
	for _, arch := range archs {
		archMap[arch.ArchitectureName] = arch.ArchitectureName
	}
	if task == "" {
		task = types.TextGeneration
	}

	deadline, ok := ctx.Deadline()
	durationUntilDeadline := 2 * time.Hour
	if ok {
		durationUntilDeadline = time.Until(deadline)
	}
	err = c.cache.RunWhileLocked(ctx, "runtime_architecture_scan_lock", durationUntilDeadline, func(ctx context.Context) error {
		if scanType == 0 || scanType == 2 {
			err := c.scanExistModels(ctx, types.ScanReq{
				FrameID:   id,
				FrameType: frame.Type,
				ArchMap:   archMap,
				Models:    models,
				Task:      task,
			})
			if err != nil {
				slog.Error("scan old models failed", slog.Any("error", err))
			}
		}

		if scanType == 0 || scanType == 1 {
			err := c.scanNewModels(ctx, types.ScanReq{
				FrameID:   id,
				FrameType: frame.Type,
				ArchMap:   archMap,
				Models:    models,
				Task:      task,
			})
			if err != nil {
				slog.Error("scan new models failed", slog.Any("error", err))
			}
		}
		slog.Info("scan models to update runtime frameworks done")
		return nil
	})
	if err != nil {
		slog.Warn("architecture scan is already in progress")
		return fmt.Errorf("architecture scan is already in progress")
	}
	return nil
}

func (c *runtimeArchitectureComponentImpl) scanNewModels(ctx context.Context, req types.ScanReq) error {
	var i int
	for {
		repos, err := c.repoStore.GetRepoWithoutRuntimeByID(ctx, req.FrameID, req.Models, 1000, i)
		i += 1
		if err != nil {
			return fmt.Errorf("failed to get repos without runtime by ID, %w", err)
		}
		if len(repos) == 0 {
			break
		}
		runtime_framework, err := c.runtimeFrameworksStore.FindByID(ctx, req.FrameID)
		if err != nil {
			return fmt.Errorf("failed to get runtime framework by ID, %w", err)
		}
		runtime_framework_tags, _ := c.tagStore.GetTagsByScopeAndCategories(ctx, "model", []string{"runtime_framework", "resource"})
		for _, repo := range repos {
			arch, err := c.GetArchitecture(ctx, req.Task, &repo)
			if err != nil {
				slog.Warn("did not to get arch for create relation", slog.Any("ConfigFileName", ConfigFileName), slog.Any("repo", repo.Path), slog.Any("error", err))
				continue
			}
			if len(arch) < 1 {
				continue
			}
			_, name := repo.NamespaceAndName()
			// check if model is in resource model table but not in runtime framework repo
			isSupportedRM, err := c.IsSupportedModelResource(ctx, name, runtime_framework, repo.ID)
			if err != nil {
				slog.Debug("fail to check model name in runtime framework repo", slog.Any("repo", repo.Path), slog.Any("error", err))
			}
			_, exist := req.ArchMap[arch]
			if !exist && !isSupportedRM {
				continue
			}
			err = c.repoRuntimeFrameworkStore.Add(ctx, req.FrameID, repo.ID, req.FrameType)
			if err != nil {
				slog.Warn("fail to create relation", slog.Any("repo", repo.Path), slog.Any("frameid", req.FrameID), slog.Any("error", err))
			}
			// add runtime framework and resource tag to model
			err = c.AddRuntimeFrameworkTag(ctx, runtime_framework_tags, repo.ID, req.FrameID)
			if err != nil {
				slog.Warn("fail to add runtime framework tag", slog.Any("repo", repo.Path), slog.Any("frameid", req.FrameID), slog.Any("error", err))
			}
			err = c.AddResourceTag(ctx, runtime_framework_tags, name, repo.ID)
			if err != nil {
				slog.Warn("fail to add resource tag", slog.Any("repo", repo.Path), slog.Any("frameid", req.FrameID), slog.Any("error", err))
			}
		}
	}
	return nil
}

// check if it's supported model resource by name
func (c *runtimeArchitectureComponentImpl) IsSupportedModelResource(ctx context.Context, modelName string, rf *database.RuntimeFramework, id int64) (bool, error) {
	trimModel := strings.Replace(strings.ToLower(modelName), "meta-", "", 1)
	rm, err := c.resouceModelStore.CheckModelNameNotInRFRepo(ctx, trimModel, id)
	if err != nil || rm == nil {
		return false, err
	}
	image := strings.ToLower(rf.FrameImage)
	if strings.Contains(image, "/") {
		parts := strings.Split(image, "/")
		image = parts[len(parts)-1]
	}

	if strings.Contains(image, rm.EngineName) {
		return true, nil
	}
	if matchRuntimeFrameworkWithEngineEE(rf, rm.EngineName) {
		return true, nil
	}

	// special handling for nim models
	nimImage := strings.ReplaceAll(image, "-", "")
	nimMatchModel := strings.ReplaceAll(trimModel, "-", "")
	if strings.Contains(nimImage, nimMatchModel) {
		return true, nil
	}
	return false, nil
}

func (c *runtimeArchitectureComponentImpl) scanExistModels(ctx context.Context, req types.ScanReq) error {
	repos, err := c.repoStore.GetRepoWithRuntimeByID(ctx, req.FrameID, req.Models)
	if err != nil {
		return fmt.Errorf("fail to get repos with runtime by ID, %w", err)
	}
	if repos == nil {
		return nil
	}
	for _, repo := range repos {
		arch, err := c.GetArchitecture(ctx, req.Task, &repo)
		if err != nil {
			slog.Warn("did not to get arch for remove relation", slog.Any("ConfigFileName", ConfigFileName), slog.Any("repo", repo.Path), slog.Any("error", err))
			continue
		}
		if len(arch) < 1 {
			continue
		}
		_, exist := req.ArchMap[arch]
		if exist {
			continue
		}
		err = c.repoRuntimeFrameworkStore.Delete(ctx, req.FrameID, repo.ID, req.FrameType)
		if err != nil {
			slog.Warn("fail to remove relation", slog.Any("repo", repo.Path), slog.Any("frameid", req.FrameID), slog.Any("error", err))
		}
	}
	return nil
}
func (c *runtimeArchitectureComponentImpl) GetArchitecture(ctx context.Context, task types.PipelineTask, repo *database.Repository) (string, error) {
	if task == types.TextGeneration {
		return c.GetArchitectureFromConfig(ctx, repo)
	} else if task == types.Text2Image {
		return c.GetClassNameFromConfig(ctx, repo)
	} else if task == types.TaskAutoDetection {
		arch, err := c.GetArchitectureFromConfig(ctx, repo)
		if err == nil {
			return arch, err
		}
		return c.GetClassNameFromConfig(ctx, repo)
	} else {
		return "", fmt.Errorf("task type %s not supported", task)
	}
}

// for text-generation
func (c *runtimeArchitectureComponentImpl) GetArchitectureFromConfig(ctx context.Context, repo *database.Repository) (string, error) {
	content, err := c.getConfigContent(ctx, ConfigFileName, repo)
	if err != nil {
		return "", fmt.Errorf("fail to read config.json for relation, %w", err)
	}
	var config struct {
		Architectures []string `json:"architectures"`
	}
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		return "", fmt.Errorf("fail to unmarshal config, %w", err)
	}
	slog.Debug("unmarshal config", slog.Any("config", config))
	if config.Architectures == nil {
		return "", nil
	}
	if len(config.Architectures) < 1 {
		return "", nil
	}
	slog.Debug("architectures of config", slog.Any("Architectures", config.Architectures))
	return config.Architectures[0], nil
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

// remove runtime_framework tag from model
func (c *runtimeArchitectureComponentImpl) RemoveRuntimeFrameworkTag(ctx context.Context, rftags []*database.Tag, repoId, rfId int64) {
	rfw, _ := c.runtimeFrameworksStore.FindByID(ctx, rfId)
	for _, tag := range rftags {
		if checkTagName(rfw, tag.Name) {
			err := c.tagStore.RemoveRepoTags(ctx, repoId, []int64{tag.ID})
			if err != nil {
				slog.Warn("fail to remove runtime_framework tag from model repo", slog.Any("repoId", repoId), slog.Any("runtime_framework_id", rfId), slog.Any("error", err))
			}
		}
	}
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
				slog.Warn("fail to add runtime_framework tag to model repo", slog.Any("repoId", repoId), slog.Any("runtime_framework_id", rfId), slog.Any("error", err))
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
