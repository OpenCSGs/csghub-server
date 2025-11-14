package callback

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type GitCallbackComponent interface {
	SetRepoVisibility(yes bool)
	WatchSpaceChange(ctx context.Context, req *types.GiteaCallbackPushReq) error
	WatchRepoRelation(ctx context.Context, req *types.GiteaCallbackPushReq) error
	GenSyncVersion(ctx context.Context, req *types.GiteaCallbackPushReq) error
	SetRepoUpdateTime(ctx context.Context, req *types.GiteaCallbackPushReq) error
	UpdateRepoInfos(ctx context.Context, req *types.GiteaCallbackPushReq) error
	SensitiveCheck(ctx context.Context, req *types.GiteaCallbackPushReq) error
	MCPScan(ctx context.Context, req *types.GiteaCallbackPushReq) error
}

type gitCallbackComponentImpl struct {
	config                    *config.Config
	gitServer                 gitserver.GitServer
	tagComponent              component.TagComponent
	modSvcClient              rpc.ModerationSvcClient
	modelStore                database.ModelStore
	datasetStore              database.DatasetStore
	spaceComponent            component.SpaceComponent
	spaceStore                database.SpaceStore
	repoStore                 database.RepoStore
	repoRelationStore         database.RepoRelationsStore
	mirrorStore               database.MirrorStore
	syncVersionGenerator      SyncVersionGenerator
	repoRuntimeFrameworkStore database.RepositoriesRuntimeFrameworkStore
	runtimeArchComponent      component.RuntimeArchitectureComponent
	runtimeArchStore          database.RuntimeArchitecturesStore
	runtimeFrameworkStore     database.RuntimeFrameworksStore
	tagStore                  database.TagStore
	tagRuleStore              database.TagRuleStore
	llmConfigStore            database.LLMConfigStore
	promptPrefixStore         database.PromptPrefixStore
	mcpScanResultStore        database.MCPScanResultStore
	mcpScanner                component.MCPScannerComponent
	// set visibility if file content is sensitive
	setRepoVisibility bool
	maxPromptFS       int64
}

// new CallbackComponent
func NewGitCallback(config *config.Config) (*gitCallbackComponentImpl, error) {
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, err
	}
	tc, err := component.NewTagComponent(config)
	if err != nil {
		return nil, err
	}
	ms := database.NewModelStore()
	ds := database.NewDatasetStore()
	ss := database.NewSpaceStore()
	rs := database.NewRepoStore()
	rrs := database.NewRepoRelationsStore()
	mirrorStore := database.NewMirrorStore()
	sc, err := component.NewSpaceComponent(config)
	ras := database.NewRuntimeArchitecturesStore()
	if err != nil {
		return nil, err
	}
	svGen := NewSyncVersionGenerator()
	rrf := database.NewRepositoriesRuntimeFramework()
	rac, err := component.NewRuntimeArchitectureComponent(config)
	if err != nil {
		return nil, err
	}
	rfs := database.NewRuntimeFrameworksStore()
	ts := database.NewTagStore()
	var modSvcClient rpc.ModerationSvcClient
	if config.SensitiveCheck.Enable {
		modSvcClient = rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", config.Moderation.Host, config.Moderation.Port))
	}
	dt := database.NewTagRuleStore()
	llmConfigStore := database.NewLLMConfigStore(config)
	promptPrefixStore := database.NewPromptPrefixStore(config)
	mcpScanResultStore := database.NewMCPScanResultStore()
	mcpScanner := component.NewMCPScannerComponent(config)
	return &gitCallbackComponentImpl{
		config:                    config,
		gitServer:                 gs,
		tagComponent:              tc,
		modelStore:                ms,
		datasetStore:              ds,
		spaceStore:                ss,
		spaceComponent:            sc,
		repoStore:                 rs,
		repoRelationStore:         rrs,
		mirrorStore:               mirrorStore,
		modSvcClient:              modSvcClient,
		syncVersionGenerator:      svGen,
		repoRuntimeFrameworkStore: rrf,
		runtimeArchComponent:      rac,
		runtimeArchStore:          ras,
		runtimeFrameworkStore:     rfs,
		tagStore:                  ts,
		tagRuleStore:              dt,
		maxPromptFS:               config.Dataset.PromptMaxJsonlFileSize,
		llmConfigStore:            llmConfigStore,
		promptPrefixStore:         promptPrefixStore,
		mcpScanResultStore:        mcpScanResultStore,
		mcpScanner:                mcpScanner,
	}, nil
}

// SetRepoVisibility sets a flag whether change repo's visibility if file content is sensitive
func (c *gitCallbackComponentImpl) SetRepoVisibility(yes bool) {
	c.setRepoVisibility = yes
}

func (c *gitCallbackComponentImpl) WatchSpaceChange(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	err := WatchSpaceChange(req, c.spaceStore, c.spaceComponent).Run()
	if err != nil {
		slog.Error("[git_callback] watch space change failed", slog.Any("error", err))
		return err
	}
	return nil
}

func (c *gitCallbackComponentImpl) WatchRepoRelation(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	err := WatchRepoRelation(req, c.repoStore, c.repoRelationStore, c.gitServer).Run()
	if err != nil {
		slog.Error("[git_callback] watch repo relation failed", slog.Any("error", err))
		return err
	}
	return nil
}

func (c *gitCallbackComponentImpl) GenSyncVersion(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	if !req.Repository.Private {
		err := c.syncVersionGenerator.GenSyncVersion(req)
		if err != nil {
			slog.Error("generate sync version failed", slog.Any("error", err))
			return err
		}
	}
	return nil
}

func (c *gitCallbackComponentImpl) SetRepoUpdateTime(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	// split req.Repository.FullName by '/'
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")
	adjustedRepoType := types.RepositoryType(strings.TrimRight(repoType, "s"))
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	isMirrorRepo, err := c.repoStore.IsMirrorRepo(ctx, adjustedRepoType, namespace, repoName)
	if err != nil {
		slog.Error("failed to check if a mirror repo", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
		return err
	}
	if isMirrorRepo {
		updated, err := time.Parse(time.RFC3339, req.HeadCommit.Timestamp)
		if err != nil {
			slog.Error("Error parsing time:", slog.Any("error", err), slog.String("timestamp", req.HeadCommit.Timestamp))
			updated = time.Now() // set updated to current time if parsing fails
		}
		err = c.repoStore.SetUpdateTimeByPath(ctx, adjustedRepoType, namespace, repoName, updated)
		if err != nil {
			slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			return err
		}
		mirror, err := c.mirrorStore.FindByRepoPath(ctx, adjustedRepoType, namespace, repoName)
		if err != nil {
			slog.Error("failed to find repo mirror", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			return err
		}
		mirror.LastUpdatedAt = time.Now()
		err = c.mirrorStore.Update(ctx, mirror)
		if err != nil {
			slog.Error("failed to update repo mirror last_updated_at", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			return err
		}
	} else {
		err := c.repoStore.SetUpdateTimeByPath(ctx, adjustedRepoType, namespace, repoName, time.Now())
		if err != nil {
			slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			return err
		}
	}
	return nil
}

func (c *gitCallbackComponentImpl) UpdateRepoInfos(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	commits := req.Commits
	ref := req.Ref
	// split req.Repository.FullName by '/'
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")

	var err error
	for _, commit := range commits {
		err = errors.Join(err, c.modifyFiles(ctx, repoType, namespace, repoName, ref, commit.Modified))
		err = errors.Join(err, c.removeFiles(ctx, repoType, namespace, repoName, ref, commit.Removed))
		err = errors.Join(err, c.addFiles(ctx, repoType, namespace, repoName, ref, commit.Added))
	}
	err = c.updateDescriptionFromReadme(ctx, repoType, namespace, repoName, ref)
	if err != nil {
		return err
	}

	return err
}

func (c *gitCallbackComponentImpl) SensitiveCheck(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	// split req.Repository.FullName by '/'
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")
	adjustedRepoType := types.RepositoryType(strings.TrimRight(repoType, "s"))

	var err error
	if c.modSvcClient != nil {
		err = c.modSvcClient.SubmitRepoCheck(ctx, adjustedRepoType, namespace, repoName)
	}
	if err != nil {
		slog.Error("fail to submit repo sensitive check", slog.Any("error", err), slog.Any("repo_type", adjustedRepoType), slog.String("namespace", namespace), slog.String("name", repoName))
		return err
	}

	return nil
}

// modifyFiles method handles modified files, skip if not modify README.md
func (c *gitCallbackComponentImpl) modifyFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	//update repo tags firstly
	if slices.Contains(fileNames, types.ReadmeFileName) {
		content, err := c.getFileRaw(repoType, namespace, repoName, ref, types.ReadmeFileName)
		if err != nil {
			return err
		}
		// should be only one README.md
		err = c.updateMetaTags(ctx, repoType, namespace, repoName, ref, content)
		if err != nil {
			slog.Error("failed to update meta tags", slog.String("content", content))
		}
	}
	// update model runtime
	return c.updateRepoRelations(ctx, repoType, namespace, repoName, ref, false, fileNames)
}

func (c *gitCallbackComponentImpl) removeFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	// handle removed files
	// delete tags
	// update model runtime
	err := c.updateRepoRelations(ctx, repoType, namespace, repoName, ref, true, fileNames)
	if err != nil {
		slog.Warn("failed to update repo relations", slog.Any("error", err))
	}

	for _, fileName := range fileNames {
		slog.Debug("remove file", slog.String("file", fileName))
		// only care about readme file under root directory
		if fileName == types.ReadmeFileName {
			// use empty content to clear all the meta tags
			const content string = ""
			adjustedRepoType := types.RepositoryType(strings.TrimSuffix(repoType, "s"))
			err := c.tagComponent.ClearMetaTags(ctx, adjustedRepoType, namespace, repoName)
			if err != nil {
				slog.Error("failed to clear meta tags", slog.String("content", content),
					slog.String("repo", path.Join(namespace, repoName)), slog.String("ref", ref),
					slog.Any("error", err))
				return fmt.Errorf("failed to clear met tags,cause: %w", err)
			}
		} else {
			tagScope, err := getTagScopeByRepoType(repoType)
			if err != nil {
				slog.Error("failed to get tag scope for remove repo library file",
					slog.Any("namespace", namespace), slog.Any("reponame", repoName),
					slog.Any("file", fileName), slog.Any("err", err))
				return fmt.Errorf("failed to get tag scope for remove repo %s/%s library file %s, error: %w",
					namespace, repoName, fileName, err)
			}
			err = c.tagComponent.UpdateLibraryTags(ctx, tagScope, namespace, repoName, fileName, "")
			if err != nil {
				slog.Error("failed to remove Library tag", slog.String("namespace", namespace),
					slog.String("name", repoName), slog.String("ref", ref), slog.String("fileName", fileName),
					slog.Any("error", err))
				return fmt.Errorf("failed to remove Library tag, cause: %w", err)
			}
		}
	}
	return nil
}

func (c *gitCallbackComponentImpl) addFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	if len(fileNames) == 0 {
		return nil
	}
	// update tag firstly
	err := c.updateRepoTags(ctx, repoType, namespace, repoName, ref, fileNames)
	if err != nil {
		return err
	}
	// update model runtime
	err = c.updateRepoRelations(ctx, repoType, namespace, repoName, ref, false, fileNames)

	return err
}

// update Repo tags
func (c *gitCallbackComponentImpl) updateRepoTags(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	for _, fileName := range fileNames {
		slog.Debug("add file", slog.String("file", fileName))
		// only care about readme file under root directory
		if fileName == types.ReadmeFileName {
			content, err := c.getFileRaw(repoType, namespace, repoName, ref, fileName)
			if err != nil {
				return err
			}
			err = c.updateMetaTags(ctx, repoType, namespace, repoName, ref, content)
			if err != nil {
				return err
			}
		} else {
			tagScope, err := getTagScopeByRepoType(repoType)
			if err != nil {
				slog.Error("failed to get tag scope for update repo library file",
					slog.Any("namespace", namespace), slog.Any("reponame", repoName),
					slog.Any("file", fileName), slog.Any("err", err))
				return fmt.Errorf("failed to get tag scope for update repo %s/%s library file %s, error: %w",
					namespace, repoName, fileName, err)
			}
			err = c.tagComponent.UpdateLibraryTags(ctx, tagScope, namespace, repoName, "", fileName)
			if err != nil {
				slog.Error("failed to add Library tag", slog.String("namespace", namespace),
					slog.String("name", repoName), slog.String("ref", ref), slog.String("fileName", fileName),
					slog.Any("error", err))
				return fmt.Errorf("failed to add Library tag, cause: %w", err)
			}
		}
	}
	return nil
}

func (c *gitCallbackComponentImpl) updateMetaTags(ctx context.Context, repoType, namespace, repoName, ref, content string) error {
	var (
		err      error
		tagScope types.TagScope
	)
	tagScope, err = getTagScopeByRepoType(repoType)
	if err != nil {
		slog.Error("failed to get tag scope for update meta tags",
			slog.Any("namespace", namespace), slog.Any("reponame", repoName), slog.Any("err", err))
		return fmt.Errorf("failed to get tag scope for update repo %s/%s meta tags, error: %w", namespace, repoName, err)
	}
	_, err = c.tagComponent.UpdateMetaTags(ctx, tagScope, namespace, repoName, content)
	if err != nil {
		slog.Error("failed to update meta tags", slog.String("namespace", namespace),
			slog.String("content", content), slog.String("repo", repoName), slog.String("ref", ref),
			slog.Any("error", err))
		return fmt.Errorf("failed to update met tags, cause: %w", err)
	}
	slog.Info("update meta tags success", slog.String("repo", path.Join(namespace, repoName)), slog.String("type", repoType))
	return nil
}

func (c *gitCallbackComponentImpl) getFileRaw(repoType, namespace, repoName, ref, fileName string) (string, error) {
	var (
		content string
		err     error
	)
	repoType = strings.TrimRight(repoType, "s")
	getFileRawReq := gitserver.GetRepoInfoByPathReq{
		Namespace: namespace,
		Name:      repoName,
		Ref:       ref,
		Path:      fileName,
		RepoType:  types.RepositoryType(repoType),
	}
	content, err = c.gitServer.GetRepoFileRaw(context.Background(), getFileRawReq)
	if err != nil {
		slog.Error("failed to get file content", slog.String("namespace", namespace),
			slog.String("file", fileName), slog.String("repo", repoName), slog.String("ref", ref),
			slog.Any("error", err))
		return "", fmt.Errorf("failed to get file content,cause: %w", err)
	}
	slog.Debug("get file content success", slog.String("repoType", repoType), slog.String("namespace", namespace),
		slog.String("file", fileName), slog.String("repo", repoName), slog.String("ref", ref))

	return content, nil
}

// update repo relations
func (c *gitCallbackComponentImpl) updateRepoRelations(ctx context.Context, repoType, namespace, repoName, ref string, deleteAction bool, fileNames []string) error {
	slog.Debug("update model relation for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("repoType", repoType), slog.Any("branch", ref))
	if repoType == fmt.Sprintf("%ss", types.ModelRepo) {
		return c.updateModelInfo(ctx, repoType, namespace, repoName, fileNames)
	}
	if repoType == fmt.Sprintf("%ss", types.DatasetRepo) {
		return c.updateDatasetTags(ctx, namespace, repoName, fileNames)
	}
	return nil
}

// update dataset tags for evaluation
func (c *gitCallbackComponentImpl) updateDatasetTags(ctx context.Context, namespace, repoName string, fileNames []string) error {
	// script dataset repo was not supported so far
	scriptName := fmt.Sprintf("%s.py", repoName)
	if slices.Contains(fileNames, scriptName) {
		return nil
	}
	repo, err := c.repoStore.FindByPath(ctx, types.DatasetRepo, namespace, repoName)
	if err != nil || repo == nil {
		slog.Warn("fail to query repo for in callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return fmt.Errorf("fail to query repo for in callback, cause: %w", err)
	}
	// check if it's evaluation dataset
	evalDataset, err := c.tagRuleStore.FindByRepo(ctx, string(types.EvaluationCategory), namespace, repoName, string(types.DatasetRepo))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// check if it's a mirror repo
			namespace, name := repo.OriginNamespaceAndName()
			if namespace == "" || name == "" {
				slog.Debug("not an evaluation dataset, ignore it", slog.Any("repo id", repo.Path))
			}
			// use mirror namespace and name to find dataset
			evalDataset, err = c.tagRuleStore.FindByRepo(ctx, string(types.EvaluationCategory), namespace, name, string(types.DatasetRepo))
			if err != nil {
				slog.Debug("not an evaluation dataset, ignore it", slog.Any("repo id", repo.Path))
			}
		} else {
			slog.Error("failed to query evaluation dataset", slog.Any("repo id", repo.Path), slog.Any("error", err))
			return fmt.Errorf("failed to query evaluation dataset, cause: %w", err)
		}

	}

	// check if it's a task dataset (e.g., ms-swift)
	taskDataset, err := c.tagRuleStore.FindByRepo(ctx, "task", namespace, repoName, string(types.DatasetRepo))
	slog.Info("taskDataset", slog.Any("taskDataset", taskDataset))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("failed to query task dataset", slog.Any("repo id", repo.Path), slog.Any("error", err))
	}
	// if not found, check with mirror namespace and name
	if errors.Is(err, sql.ErrNoRows) {
		mirrorNamespace, mirrorName := repo.OriginNamespaceAndName()
		if mirrorNamespace != "" && mirrorName != "" {
			taskDataset, err = c.tagRuleStore.FindByRepo(ctx, "task", mirrorNamespace, mirrorName, string(types.DatasetRepo))
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				slog.Error("failed to query task dataset with mirror path", slog.Any("repo id", repo.Path), slog.Any("error", err))
			}
		}
	}

	tagIds := []int64{}

	if evalDataset != nil && evalDataset.RuntimeFramework != "" {
		tagIds = append(tagIds, evalDataset.Tag.ID)
		rTag, _ := c.tagStore.FindTag(ctx, evalDataset.RuntimeFramework, string(types.DatasetRepo), "runtime_framework")
		if rTag != nil {
			tagIds = append(tagIds, rTag.ID)
		}
	}
	// add task tag if found
	if taskDataset != nil && taskDataset.Tag.ID > 0 {
		tagIds = append(tagIds, taskDataset.Tag.ID)
		if taskDataset.RuntimeFramework != "" {
			taskRTag, _ := c.tagStore.FindTag(ctx, taskDataset.RuntimeFramework, string(types.DatasetRepo), "runtime_framework")
			if taskRTag != nil {
				tagIds = append(tagIds, taskRTag.ID)
			}
		}
	}
	if len(tagIds) == 0 {
		return nil
	}

	err = c.tagStore.UpsertRepoTags(ctx, repo.ID, []int64{}, tagIds)
	if err != nil {
		slog.Warn("fail to add dataset tag", slog.Any("repoId", repo.ID), slog.Any("tag id", tagIds), slog.Any("error", err))
	}
	return err
}

// update model runtime frameworks
func (c *gitCallbackComponentImpl) updateModelInfo(ctx context.Context, repoType, namespace, repoName string, fileNames []string) error {
	//check file contains
	if len(fileNames) == 0 {
		return nil
	}
	repo, err := c.repoStore.FindByPath(ctx, types.ModelRepo, namespace, repoName)
	if err != nil || repo == nil {
		slog.Warn("fail to query repo for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return err
	}
	return c.updateModelMetadata(ctx, fileNames, repo)
}

func (c *gitCallbackComponentImpl) updateModelMetadata(ctx context.Context, fileNames []string, repo *database.Repository) error {
	// must be model repo and config.json
	valid := c.isValidForRuntime(fileNames)
	if !valid {
		slog.Warn("ignore model metadata update for the given files", slog.Any("fileNames", fileNames))
		return nil
	}
	modelInfo, err := c.runtimeArchComponent.UpdateModelMetadata(ctx, repo)
	if err != nil {
		slog.Warn("fail to update model metadata", slog.Any("error", err), slog.Any("repo path", repo.Path))
		return err
	}
	err = c.runtimeArchComponent.UpdateRuntimeFrameworkTag(ctx, modelInfo, repo)
	if err != nil {
		slog.Warn("fail to update runtime framework tag", slog.Any("error", err), slog.Any("repo path", repo.Path))
	}
	return err
}

// check if the repo is valid for runtime framework
func (c *gitCallbackComponentImpl) isValidForRuntime(fileNames []string) bool {
	for _, fileName := range fileNames {
		if strings.Contains(fileName, component.ConfigFileName) {
			return true
		}
		if strings.Contains(fileName, component.ModelIndexFileName) {
			return true
		}
		if strings.Contains(fileName, types.ReadmeFileName) {
			return true
		}
		if strings.Contains(fileName, string(types.Safetensors)) {
			return true
		}
		if strings.Contains(fileName, string(types.GGUF)) {
			return true
		}
	}

	return false
}

func GetPipelineTaskFromTags(tags []database.Tag) types.PipelineTask {
	for _, tag := range tags {
		if tag.Name == string(types.TextGeneration) {
			return types.TextGeneration
		}
		if tag.Name == string(types.Text2Image) {
			return types.Text2Image
		}
	}
	return ""
}

func getTagScopeByRepoType(repoType string) (types.TagScope, error) {
	var tagScope types.TagScope
	switch repoType {
	case fmt.Sprintf("%ss", types.DatasetRepo):
		tagScope = types.DatasetTagScope
	case fmt.Sprintf("%ss", types.ModelRepo):
		tagScope = types.ModelTagScope
	case fmt.Sprintf("%ss", types.PromptRepo):
		tagScope = types.PromptTagScope
	case fmt.Sprintf("%ss", types.MCPServerRepo):
		tagScope = types.MCPTagScope
	case fmt.Sprintf("%ss", types.CodeRepo):
		tagScope = types.CodeTagScope
	default:
		return types.UnknownScope, fmt.Errorf("get tag scope by invalid repo type %s", repoType)
		// TODO: support space
		// case SpaceRepoType:
		// 	tagScope = types.SpaceTagScope
	}

	return tagScope, nil

}
