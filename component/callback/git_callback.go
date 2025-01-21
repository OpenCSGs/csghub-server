package callback

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"strconv"
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
	SetRepoUpdateTime(ctx context.Context, req *types.GiteaCallbackPushReq) error
	UpdateRepoInfos(ctx context.Context, req *types.GiteaCallbackPushReq) error
	SensitiveCheck(ctx context.Context, req *types.GiteaCallbackPushReq) error
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
	repoRuntimeFrameworkStore database.RepositoriesRuntimeFrameworkStore
	runtimeArchComponent      component.RuntimeArchitectureComponent
	runtimeArchStore          database.RuntimeArchitecturesStore
	runtimeFrameworkStore     database.RuntimeFrameworksStore
	tagStore                  database.TagStore
	tagRuleStore              database.TagRuleStore
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
		repoRuntimeFrameworkStore: rrf,
		runtimeArchComponent:      rac,
		runtimeArchStore:          ras,
		runtimeFrameworkStore:     rfs,
		tagStore:                  ts,
		tagRuleStore:              dt,
		maxPromptFS:               config.Dataset.PromptMaxJsonlFileSize,
	}, nil
}

// SetRepoVisibility sets a flag whether change repo's visibility if file content is sensitive
func (c *gitCallbackComponentImpl) SetRepoVisibility(yes bool) {
	c.setRepoVisibility = yes
}

func (c *gitCallbackComponentImpl) WatchSpaceChange(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	err := WatchSpaceChange(req, c.spaceStore, c.spaceComponent).Run()
	if err != nil {
		slog.Error("watch space change failed", slog.Any("error", err))
		return err
	}
	return nil
}

func (c *gitCallbackComponentImpl) WatchRepoRelation(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	err := WatchRepoRelation(req, c.repoStore, c.repoRelationStore, c.gitServer).Run()
	if err != nil {
		slog.Error("watch repo relation failed", slog.Any("error", err))
		return err
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
			return err
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
	for _, fileName := range fileNames {
		slog.Debug("modify file", slog.String("file", fileName))
		// update model runtime
		c.updateRepoRelations(ctx, repoType, namespace, repoName, ref, fileName, false, fileNames)
		// only care about readme file under root directory
		if fileName != types.ReadmeFileName {
			continue
		}

		content, err := c.getFileRaw(repoType, namespace, repoName, ref, fileName)
		if err != nil {
			return err
		}
		// should be only one README.md
		return c.updateMetaTags(ctx, repoType, namespace, repoName, ref, content)
	}
	return nil
}

func (c *gitCallbackComponentImpl) removeFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	// handle removed files
	// delete tags
	for _, fileName := range fileNames {
		slog.Debug("remove file", slog.String("file", fileName))
		// update model runtime
		c.updateRepoRelations(ctx, repoType, namespace, repoName, ref, fileName, true, fileNames)
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
			var tagScope database.TagScope
			switch repoType {
			case fmt.Sprintf("%ss", types.DatasetRepo):
				tagScope = database.DatasetTagScope
			case fmt.Sprintf("%ss", types.ModelRepo):
				tagScope = database.ModelTagScope
			case fmt.Sprintf("%ss", types.PromptRepo):
				tagScope = database.PromptTagScope
			default:
				return nil
				// case CodeRepoType:
				// 	tagScope = database.CodeTagScope
				// case SpaceRepoType:
				// 	tagScope = database.SpaceTagScope
			}
			err := c.tagComponent.UpdateLibraryTags(ctx, tagScope, namespace, repoName, fileName, "")
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
	for _, fileName := range fileNames {
		slog.Debug("add file", slog.String("file", fileName))
		// update model runtime
		c.updateRepoRelations(ctx, repoType, namespace, repoName, ref, fileName, false, fileNames)
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
			var tagScope database.TagScope
			switch repoType {
			case fmt.Sprintf("%ss", types.DatasetRepo):
				tagScope = database.DatasetTagScope
			case fmt.Sprintf("%ss", types.ModelRepo):
				tagScope = database.ModelTagScope
			case fmt.Sprintf("%ss", types.PromptRepo):
				tagScope = database.PromptTagScope
			default:
				return nil
				// case CodeRepoType:
				// 	tagScope = database.CodeTagScope
				// case SpaceRepoType:
				// 	tagScope = database.SpaceTagScope
			}
			err := c.tagComponent.UpdateLibraryTags(ctx, tagScope, namespace, repoName, "", fileName)
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
		tagScope database.TagScope
	)
	switch repoType {
	case fmt.Sprintf("%ss", types.DatasetRepo):
		tagScope = database.DatasetTagScope
	case fmt.Sprintf("%ss", types.ModelRepo):
		tagScope = database.ModelTagScope
	case fmt.Sprintf("%ss", types.PromptRepo):
		tagScope = database.PromptTagScope
	default:
		return nil
		// TODO: support code and space
		// case CodeRepoType:
		// 	tagScope = database.CodeTagScope
		// case SpaceRepoType:
		// 	tagScope = database.SpaceTagScope
	}
	_, err = c.tagComponent.UpdateMetaTags(ctx, tagScope, namespace, repoName, content)
	if err != nil {
		slog.Error("failed to update meta tags", slog.String("namespace", namespace),
			slog.String("content", content), slog.String("repo", repoName), slog.String("ref", ref),
			slog.Any("error", err))
		return fmt.Errorf("failed to update met tags,cause: %w", err)
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
func (c *gitCallbackComponentImpl) updateRepoRelations(ctx context.Context, repoType, namespace, repoName, ref, fileName string, deleteAction bool, fileNames []string) {
	slog.Debug("update model relation for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("repoType", repoType), slog.Any("fileName", fileName), slog.Any("branch", ref))
	if repoType == fmt.Sprintf("%ss", types.ModelRepo) {
		c.updateModelRuntimeFrameworks(ctx, repoType, namespace, repoName, ref, fileName, deleteAction)
	}
	if repoType == fmt.Sprintf("%ss", types.DatasetRepo) {
		c.updateDatasetTags(ctx, namespace, repoName, fileNames)
	}
}

// update dataset tags for evaluation
func (c *gitCallbackComponentImpl) updateDatasetTags(ctx context.Context, namespace, repoName string, fileNames []string) {
	// script dataset repo was not supported so far
	scriptName := fmt.Sprintf("%s.py", repoName)
	if slices.Contains(fileNames, scriptName) {
		return
	}
	repo, err := c.repoStore.FindByPath(ctx, types.DatasetRepo, namespace, repoName)
	if err != nil || repo == nil {
		slog.Warn("fail to query repo for in callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return
	}
	// check if it's evaluation dataset
	evalDataset, err := c.tagRuleStore.FindByRepo(ctx, string(types.EvaluationCategory), namespace, repoName, string(types.DatasetRepo))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// check if it's a mirror repo
			mirror, err := c.mirrorStore.FindByRepoPath(ctx, types.DatasetRepo, namespace, repoName)
			if err != nil || mirror == nil {
				slog.Debug("fail to query mirror dataset for in callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
				return
			}
			namespace := strings.Split(mirror.SourceRepoPath, "/")[0]
			name := strings.Split(mirror.SourceRepoPath, "/")[1]
			// use mirror namespace and name to find dataset
			evalDataset, err = c.tagRuleStore.FindByRepo(ctx, string(types.EvaluationCategory), namespace, name, string(types.DatasetRepo))
			if err != nil {
				slog.Debug("not an evaluation dataset, ignore it", slog.Any("repo id", repo.Path))
				return
			}
		} else {
			slog.Error("failed to query evaluation dataset", slog.Any("repo id", repo.Path), slog.Any("error", err))
			return
		}

	}
	tagIds := []int64{}
	tagIds = append(tagIds, evalDataset.Tag.ID)
	if evalDataset.RuntimeFramework != "" {
		rTag, _ := c.tagStore.FindTag(ctx, evalDataset.RuntimeFramework, string(types.DatasetRepo), "runtime_framework")
		if rTag != nil {
			tagIds = append(tagIds, rTag.ID)
		}
	}

	err = c.tagStore.UpsertRepoTags(ctx, repo.ID, []int64{}, tagIds)
	if err != nil {
		slog.Warn("fail to add dataset tag", slog.Any("repoId", repo.ID), slog.Any("tag id", tagIds), slog.Any("error", err))
	}

}

// update model runtime frameworks
func (c *gitCallbackComponentImpl) updateModelRuntimeFrameworks(ctx context.Context, repoType, namespace, repoName, ref, fileName string, deleteAction bool) {
	// must be model repo and config.json
	if repoType != fmt.Sprintf("%ss", types.ModelRepo) || fileName != component.ConfigFileName || (ref != ("refs/heads/"+component.MainBranch) && ref != ("refs/heads/"+component.MasterBranch)) {
		return
	}
	repo, err := c.repoStore.FindByPath(ctx, types.ModelRepo, namespace, repoName)
	if err != nil || repo == nil {
		slog.Warn("fail to query repo for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return
	}
	// delete event
	if deleteAction {
		err := c.repoRuntimeFrameworkStore.DeleteByRepoID(ctx, repo.ID)
		if err != nil {
			slog.Warn("fail to remove repo runtimes for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("repoid", repo.ID), slog.Any("error", err))
		}
		return
	}
	arch, err := c.runtimeArchComponent.GetArchitecture(ctx, types.TaskAutoDetection, repo)
	if err != nil {
		slog.Warn("fail to get config.json content for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return
	}
	slog.Debug("get arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("arch", arch))
	//add resource tag, like ascend
	runtime_framework_tags, _ := c.tagStore.GetTagsByScopeAndCategories(ctx, "model", []string{"runtime_framework", "resource"})
	fields := strings.Split(repo.Path, "/")
	err = c.runtimeArchComponent.AddResourceTag(ctx, runtime_framework_tags, fields[1], repo.ID)
	if err != nil {
		slog.Warn("fail to add resource tag", slog.Any("error", err))
		return
	}
	runtimes, err := c.runtimeArchStore.ListByRArchNameAndModel(ctx, arch, fields[1])
	// to do check resource models
	if err != nil {
		slog.Warn("fail to get runtime ids by arch for git callback", slog.Any("arch", arch), slog.Any("error", err))
		return
	}
	slog.Debug("get runtimes by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("arch", arch), slog.Any("runtimes", runtimes))
	var frameIDs []int64
	for _, runtime := range runtimes {
		frameIDs = append(frameIDs, runtime.RuntimeFrameworkID)
	}
	slog.Debug("get new frame ids for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("frameIDs", frameIDs))
	newFrames, err := c.runtimeFrameworkStore.ListByIDs(ctx, frameIDs)
	if err != nil {
		slog.Warn("fail to get runtime frameworks for git callback", slog.Any("arch", arch), slog.Any("error", err))
		return
	}
	slog.Debug("get new frames by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("newFrames", newFrames))
	var newFrameMap = make(map[string]string)
	for _, frame := range newFrames {
		newFrameMap[strconv.FormatInt(frame.ID, 10)] = strconv.FormatInt(frame.ID, 10)
	}
	slog.Debug("get new frame map by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("newFrameMap", newFrameMap))
	oldRepoRuntimes, err := c.repoRuntimeFrameworkStore.GetByRepoIDs(ctx, repo.ID)
	if err != nil {
		slog.Warn("fail to get repo runtimes for git callback", slog.Any("repo.ID", repo.ID), slog.Any("error", err))
		return
	}
	slog.Debug("get old frames by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("oldRepoRuntimes", oldRepoRuntimes))
	var oldFrameMap = make(map[string]string)
	// get map
	for _, runtime := range oldRepoRuntimes {
		oldFrameMap[strconv.FormatInt(runtime.RuntimeFrameworkID, 10)] = strconv.FormatInt(runtime.RuntimeFrameworkID, 10)
	}
	slog.Debug("get old frame map by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("oldFrameMap", oldFrameMap))
	// remove incorrect relation
	for _, old := range oldRepoRuntimes {
		// check if it need remove
		_, exist := newFrameMap[strconv.FormatInt(old.RuntimeFrameworkID, 10)]
		if !exist {
			// remove incorrect relations
			err := c.repoRuntimeFrameworkStore.Delete(ctx, old.RuntimeFrameworkID, repo.ID, old.Type)
			if err != nil {
				slog.Warn("fail to delete old repo runtimes for git callback", slog.Any("repo.ID", repo.ID), slog.Any("runtime framework id", old.RuntimeFrameworkID), slog.Any("error", err))
			}
			// remove runtime framework tags
			c.runtimeArchComponent.RemoveRuntimeFrameworkTag(ctx, runtime_framework_tags, repo.ID, old.RuntimeFrameworkID)
		}
	}

	// add new relation
	for _, new := range newFrames {
		// check if it need add
		_, exist := oldFrameMap[strconv.FormatInt(new.ID, 10)]
		if !exist {
			// add new relations
			err := c.repoRuntimeFrameworkStore.Add(ctx, new.ID, repo.ID, new.Type)
			if err != nil {
				slog.Warn("fail to add new repo runtimes for git callback", slog.Any("repo.ID", repo.ID), slog.Any("runtime framework id", new.ID), slog.Any("error", err))
			}
			// add runtime framework and resource tags
			err = c.runtimeArchComponent.AddRuntimeFrameworkTag(ctx, runtime_framework_tags, repo.ID, new.ID)
			if err != nil {
				slog.Warn("fail to add runtime framework tag for git callback", slog.Any("repo.ID", repo.ID), slog.Any("runtime framework id", new.ID), slog.Any("error", err))
			}
		}
	}

}

// check if the repo is valid for runtime framework
func (c *gitCallbackComponentImpl) isValidForRuntime(repoType, ref, fileName string) bool {
	if repoType != fmt.Sprintf("%ss", types.ModelRepo) {
		return false
	}
	if fileName != component.ConfigFileName && fileName != component.ModelIndexFileName {
		return false
	}

	if !strings.Contains(ref, component.MainBranch) && !strings.Contains(ref, component.MasterBranch) {
		return false
	}

	return true
}
