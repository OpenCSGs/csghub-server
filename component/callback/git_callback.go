package callback

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

const (
	DatasetRepoType = "datasets"
	ModelRepoType   = "models"
	CodeRepoType    = "codes"
	SpaceRepoType   = "spaces"
	ReadmeFileName  = "README.md"
)

// define GitCallbackComponent struct
type GitCallbackComponent struct {
	config      *config.Config
	gs          gitserver.GitServer
	tc          *component.TagComponent
	checker     component.SensitiveChecker
	ms          *database.ModelStore
	ds          *database.DatasetStore
	sc          *component.SpaceComponent
	ss          *database.SpaceStore
	rs          *database.RepoStore
	rrs         *database.RepoRelationsStore
	mirrorStore *database.MirrorStore
	svGen       *SyncVersionGenerator
	rrf         *database.RepositoriesRuntimeFrameworkStore
	rac         *component.RuntimeArchitectureComponent
	ras         *database.RuntimeArchitecturesStore
	rfs         *database.RuntimeFrameworksStore
	ts          *database.TagStore
	// set visibility if file content is sensitive
	setRepoVisibility bool
	pp                *component.PromptComponent
	maxPromptFS       int64
}

// new CallbackComponent
func NewGitCallback(config *config.Config) (*GitCallbackComponent, error) {
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
	checker := component.NewSensitiveComponent(config)
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
	pp, err := component.NewPromptComponent(config)
	if err != nil {
		return nil, err
	}
	return &GitCallbackComponent{
		config:      config,
		gs:          gs,
		tc:          tc,
		ms:          ms,
		ds:          ds,
		ss:          ss,
		sc:          sc,
		rs:          rs,
		rrs:         rrs,
		mirrorStore: mirrorStore,
		checker:     checker,
		svGen:       svGen,
		rrf:         rrf,
		rac:         rac,
		ras:         ras,
		rfs:         rfs,
		ts:          ts,
		pp:          pp,
		maxPromptFS: config.Dataset.PromptMaxJsonlFileSize,
	}, nil
}

// SetRepoVisibility sets a flag whether change repo's visibility if file content is sensitive
func (c *GitCallbackComponent) SetRepoVisibility(yes bool) {
	c.setRepoVisibility = yes
}

func (c *GitCallbackComponent) HandlePush(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	go func() {
		err := WatchSpaceChange(req, c.ss, c.sc).Run()
		if err != nil {
			slog.Error("watch space change failed", slog.Any("error", err))
		}
	}()

	go func() {
		err := WatchRepoRelation(req, c.rs, c.rrs, c.gs).Run()
		if err != nil {
			slog.Error("watch repo relation failed", slog.Any("error", err))
		}
	}()

	if !req.Repository.Private {
		go func() {
			err := c.svGen.GenSyncVersion(req)
			if err != nil {
				slog.Error("generate sync version failed", slog.Any("error", err))
			}
		}()
	}

	commits := req.Commits
	ref := req.Ref
	// split req.Repository.FullName by '/'
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		adjustedRepoType := types.RepositoryType(strings.TrimRight(repoType, "s"))

		isMirrorRepo, err := c.rs.IsMirrorRepo(ctx, adjustedRepoType, namespace, repoName)
		if err != nil {
			slog.Error("failed to check if a mirror repo", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
		}
		if isMirrorRepo {
			updated, err := time.Parse(time.RFC3339, req.HeadCommit.Timestamp)
			if err != nil {
				slog.Error("Error parsing time:", slog.Any("error", err), slog.String("timestamp", req.HeadCommit.Timestamp))
				return
			}
			err = c.rs.SetUpdateTimeByPath(ctx, adjustedRepoType, namespace, repoName, updated)
			if err != nil {
				slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			}
			mirror, err := c.mirrorStore.FindByRepoPath(ctx, adjustedRepoType, namespace, repoName)
			if err != nil {
				slog.Error("failed to find repo mirror", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			}
			mirror.LastUpdatedAt = time.Now()
			err = c.mirrorStore.Update(ctx, mirror)
			if err != nil {
				slog.Error("failed to update repo mirror last_updated_at", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			}
		} else {
			err := c.rs.SetUpdateTimeByPath(ctx, adjustedRepoType, namespace, repoName, time.Now())
			if err != nil {
				slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			}
		}
	}()

	var err error
	for _, commit := range commits {
		err = errors.Join(err, c.modifyFiles(ctx, repoType, namespace, repoName, ref, commit.Modified))
		err = errors.Join(err, c.removeFiles(ctx, repoType, namespace, repoName, ref, commit.Removed))
		err = errors.Join(err, c.addFiles(ctx, repoType, namespace, repoName, ref, commit.Added))
	}

	if err != nil {
		slog.Error("git callback push has error", slog.Any("error", err))
		return err
	}

	return nil
}

// modifyFiles method handles modified files, skip if not modify README.md
func (c *GitCallbackComponent) modifyFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	for _, fileName := range fileNames {
		slog.Debug("modify file", slog.String("file", fileName))
		// update model runtime
		c.updateModelRuntimeFrameworks(ctx, repoType, namespace, repoName, ref, fileName, false)
		// only care about readme file under root directory
		if fileName != ReadmeFileName {
			continue
		}

		content, err := c.getFileRaw(repoType, namespace, repoName, ref, fileName)
		if err != nil {
			return err
		}
		if c.setRepoVisibility {
			go func(content string) {
				ok, err := c.checkFileContent(ctx, repoType, namespace, repoName, content)
				if err != nil {
					slog.Error("callback check file failed", slog.String("repo", path.Join(namespace, repoName)), slog.String("file", fileName),
						slog.String("error", err.Error()))
					return
				}
				if !ok {
					err := fmt.Errorf("sensitie context detected. Set %s %s/%s to private", repoType, namespace, repoName)
					slog.Error(err.Error())
					return
				}
			}(content)
		}
		// should be only one README.md
		return c.updateMetaTags(ctx, repoType, namespace, repoName, ref, content)
	}
	c.setPromptDatasetType(ctx, repoType, namespace, repoName, ref, fileNames)
	return nil
}

func (c *GitCallbackComponent) removeFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	// handle removed files
	// delete tags
	for _, fileName := range fileNames {
		slog.Debug("remove file", slog.String("file", fileName))
		// update model runtime
		c.updateModelRuntimeFrameworks(ctx, repoType, namespace, repoName, ref, fileName, true)
		// only care about readme file under root directory
		if fileName == ReadmeFileName {
			// use empty content to clear all the meta tags
			const content string = ""
			adjustedRepoType := types.RepositoryType(strings.TrimSuffix(repoType, "s"))
			err := c.tc.ClearMetaTags(ctx, adjustedRepoType, namespace, repoName)
			if err != nil {
				slog.Error("failed to clear meta tags", slog.String("content", content),
					slog.String("repo", path.Join(namespace, repoName)), slog.String("ref", ref),
					slog.Any("error", err))
				return fmt.Errorf("failed to clear met tags,cause: %w", err)
			}
		} else {
			var tagScope database.TagScope
			switch repoType {
			case DatasetRepoType:
				tagScope = database.DatasetTagScope
			case ModelRepoType:
				tagScope = database.ModelTagScope
			default:
				return nil
				// case CodeRepoType:
				// 	tagScope = database.CodeTagScope
				// case SpaceRepoType:
				// 	tagScope = database.SpaceTagScope
			}
			err := c.tc.UpdateLibraryTags(ctx, tagScope, namespace, repoName, fileName, "")
			if err != nil {
				slog.Error("failed to remove Library tag", slog.String("namespace", namespace),
					slog.String("name", repoName), slog.String("ref", ref), slog.String("fileName", fileName),
					slog.Any("error", err))
				return fmt.Errorf("failed to remove Library tag, cause: %w", err)
			}
		}
	}
	c.removePromptDatasetType(ctx, repoType, namespace, repoName, ref, fileNames)
	return nil
}

func (c *GitCallbackComponent) addFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	for _, fileName := range fileNames {
		slog.Debug("add file", slog.String("file", fileName))
		// update model runtime
		c.updateModelRuntimeFrameworks(ctx, repoType, namespace, repoName, ref, fileName, false)
		// only care about readme file under root directory
		if fileName == ReadmeFileName {
			content, err := c.getFileRaw(repoType, namespace, repoName, ref, fileName)
			if err != nil {
				return err
			}
			if c.setRepoVisibility {
				go func(content string) {
					ok, err := c.checkFileContent(ctx, repoType, namespace, repoName, content)
					if err != nil {
						slog.Error("callback check file failed", slog.String("file", fileName), slog.String("error", err.Error()))
						return
					}
					if !ok {
						err := fmt.Errorf("sensitie contest detected. Set %s %s/%s to private", repoType, namespace, repoName)
						slog.Error(err.Error())
						return
					}
				}(content)
			}
			err = c.updateMetaTags(ctx, repoType, namespace, repoName, ref, content)
			if err != nil {
				return err
			}
		} else {
			var tagScope database.TagScope
			switch repoType {
			case DatasetRepoType:
				tagScope = database.DatasetTagScope
			case ModelRepoType:
				tagScope = database.ModelTagScope
			default:
				return nil
				// case CodeRepoType:
				// 	tagScope = database.CodeTagScope
				// case SpaceRepoType:
				// 	tagScope = database.SpaceTagScope
			}
			err := c.tc.UpdateLibraryTags(ctx, tagScope, namespace, repoName, "", fileName)
			if err != nil {
				slog.Error("failed to add Library tag", slog.String("namespace", namespace),
					slog.String("name", repoName), slog.String("ref", ref), slog.String("fileName", fileName),
					slog.Any("error", err))
				return fmt.Errorf("failed to add Library tag, cause: %w", err)
			}
		}
	}
	c.setPromptDatasetType(ctx, repoType, namespace, repoName, ref, fileNames)
	return nil
}

func (c *GitCallbackComponent) updateMetaTags(ctx context.Context, repoType, namespace, repoName, ref, content string) error {
	var (
		err      error
		tagScope database.TagScope
	)
	switch repoType {
	case DatasetRepoType:
		tagScope = database.DatasetTagScope
	case ModelRepoType:
		tagScope = database.ModelTagScope
	default:
		return nil
		// TODO: support code and space
		// case CodeRepoType:
		// 	tagScope = database.CodeTagScope
		// case SpaceRepoType:
		// 	tagScope = database.SpaceTagScope
	}
	_, err = c.tc.UpdateMetaTags(ctx, tagScope, namespace, repoName, content)
	if err != nil {
		slog.Error("failed to update meta tags", slog.String("namespace", namespace),
			slog.String("content", content), slog.String("repo", repoName), slog.String("ref", ref),
			slog.Any("error", err))
		return fmt.Errorf("failed to update met tags,cause: %w", err)
	}
	slog.Info("update meta tags success", slog.String("repo", path.Join(namespace, repoName)), slog.String("type", repoType))
	return nil
}

func (c *GitCallbackComponent) getFileRaw(repoType, namespace, repoName, ref, fileName string) (string, error) {
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
	content, err = c.gs.GetRepoFileRaw(context.Background(), getFileRawReq)
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

func (c *GitCallbackComponent) checkFileContent(ctx context.Context, repoType, namespace, repoName, content string) (bool, error) {
	ok, err := c.checkText(ctx, content)
	if err != nil {
		return ok, err
	}

	if !ok {
		slog.Info("sensitive content detected, set repo to private", slog.String("repo", path.Join(namespace, repoName)))
		err := c.setPrivate(ctx, repoType, namespace, repoName)
		if err != nil {
			return ok, err
		}
	}
	return ok, nil
}

func (c *GitCallbackComponent) checkText(ctx context.Context, content string) (bool, error) {
	return c.checker.CheckText(ctx, "comment_detection", content)
}

func (c *GitCallbackComponent) setPrivate(ctx context.Context, repoType, namespace, repoName string) error {
	var err error
	var dataset *database.Dataset
	var model *database.Model
	if repoType == DatasetRepoType {
		dataset, err = c.ds.FindByPath(ctx, namespace, repoName)
		if err != nil {
			slog.Error("Failed to find dataset", slog.String("namespace", namespace), slog.String("name", repoName))
			return fmt.Errorf("failed to find dataset, error: %w", err)
		}
		_, err = c.gs.UpdateRepo(ctx, gitserver.UpdateRepoReq{
			Name:          dataset.Repository.Name,
			Description:   dataset.Repository.Description,
			Private:       true,
			DefaultBranch: dataset.Repository.DefaultBranch,
		})
		if err != nil {
			return fmt.Errorf("failed to update git server dataset to private, error: %w", err)
		}
		err = c.ds.Update(ctx, *dataset)
		if err != nil {
			return fmt.Errorf("failed to update database dataset to private, error: %w", err)
		}
	} else {
		model, err = c.ms.FindByPath(ctx, namespace, repoName)
		if err != nil {
			return fmt.Errorf("failed to find model by path, error: %w", err)
		}
		_, err = c.gs.UpdateRepo(ctx, gitserver.UpdateRepoReq{
			Name:          model.Repository.Name,
			Description:   model.Repository.Description,
			Private:       true,
			DefaultBranch: model.Repository.DefaultBranch,
		})
		if err != nil {
			return fmt.Errorf("failed to update git server model to private, error: %w", err)
		}
		_, err = c.ms.Update(ctx, *model)
		if err != nil {
			return fmt.Errorf("failed to update database model to private, error: %w", err)
		}
	}
	slog.Info("set repository to private successed.", slog.String("repoType", repoType), slog.String("namespace", namespace),
		slog.String("repo", repoName))

	return err
}

func (c *GitCallbackComponent) updateModelRuntimeFrameworks(ctx context.Context, repoType, namespace, repoName, ref, fileName string, deleteAction bool) {
	slog.Debug("update model relation for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("repoType", repoType), slog.Any("fileName", fileName), slog.Any("branch", ref))
	// must be model repo and config.json
	if repoType != ModelRepoType || fileName != component.ConfigFileName || ref != ("refs/heads/"+component.MainBranch) {
		return
	}
	repo, err := c.rs.FindByPath(ctx, types.ModelRepo, namespace, repoName)
	if err != nil || repo == nil {
		slog.Warn("fail to query repo for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return
	}
	// delete event
	if deleteAction {
		err := c.rrf.DeleteByRepoID(ctx, repo.ID)
		if err != nil {
			slog.Warn("fail to remove repo runtimes for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("repoid", repo.ID), slog.Any("error", err))
		}
		return
	}
	arch, err := c.rac.GetArchitectureFromConfig(ctx, namespace, repoName)
	if err != nil {
		slog.Warn("fail to get config.json content for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return
	}
	slog.Debug("get arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("arch", arch))
	//add resource tag, like ascend
	runtime_framework_tags, _ := c.ts.GetTagsByScopeAndCategories(ctx, "model", []string{"runtime_framework", "resource"})
	fields := strings.Split(repo.Path, "/")
	c.rac.AddResourceTag(ctx, runtime_framework_tags, fields[1], repo.ID)
	runtimes, err := c.ras.ListByRArchNameAndModel(ctx, arch, fields[1])
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
	newFrames, err := c.rfs.ListByIDs(ctx, frameIDs)
	if err != nil {
		slog.Warn("fail to get runtime frameworks for git callback", slog.Any("arch", arch), slog.Any("error", err))
		return
	}
	slog.Debug("get new frames by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("newFrames", newFrames))
	var newFrameMap map[string]string = make(map[string]string)
	for _, frame := range newFrames {
		newFrameMap[string(frame.ID)] = string(frame.ID)
	}
	slog.Debug("get new frame map by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("newFrameMap", newFrameMap))
	oldRepoRuntimes, err := c.rrf.GetByRepoIDs(ctx, repo.ID)
	if err != nil {
		slog.Warn("fail to get repo runtimes for git callback", slog.Any("repo.ID", repo.ID), slog.Any("error", err))
		return
	}
	slog.Debug("get old frames by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("oldRepoRuntimes", oldRepoRuntimes))
	var oldFrameMap map[string]string = make(map[string]string)
	// get map
	for _, runtime := range oldRepoRuntimes {
		oldFrameMap[string(runtime.RuntimeFrameworkID)] = string(runtime.RuntimeFrameworkID)
	}
	slog.Debug("get old frame map by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("oldFrameMap", oldFrameMap))
	// remove incorrect relation
	for _, old := range oldRepoRuntimes {
		// check if it need remove
		_, exist := newFrameMap[string(old.RuntimeFrameworkID)]
		if !exist {
			// remove incorrect relations
			err := c.rrf.Delete(ctx, old.RuntimeFrameworkID, repo.ID, old.Type)
			if err != nil {
				slog.Warn("fail to delete old repo runtimes for git callback", slog.Any("repo.ID", repo.ID), slog.Any("runtime framework id", old.RuntimeFrameworkID), slog.Any("error", err))
			}
			// remove runtime framework tags
			c.rac.RemoveRuntimeFrameworkTag(ctx, runtime_framework_tags, repo.ID, old.RuntimeFrameworkID)
		}
	}

	// add new relation
	for _, new := range newFrames {
		// check if it need add
		_, exist := oldFrameMap[string(new.ID)]
		if !exist {
			// add new relations
			err := c.rrf.Add(ctx, new.ID, repo.ID, new.Type)
			if err != nil {
				slog.Warn("fail to add new repo runtimes for git callback", slog.Any("repo.ID", repo.ID), slog.Any("runtime framework id", new.ID), slog.Any("error", err))
			}
			// add runtime framework and resource tags
			err = c.rac.AddRuntimeFrameworkTag(ctx, runtime_framework_tags, repo.ID, new.ID)
			if err != nil {
				slog.Warn("fail to add runtime framework tag for git callback", slog.Any("repo.ID", repo.ID), slog.Any("runtime framework id", new.ID), slog.Any("error", err))
			}
		}
	}

}

func (c *GitCallbackComponent) setPromptDatasetType(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) {
	slog.Debug("setPromptDatasetType", slog.Any("fileNames", fileNames), slog.Any("repoType", repoType), slog.Any("ref", ref))
	if len(fileNames) < 1 {
		return
	}
	repoType = strings.TrimRight(repoType, "s")
	if repoType != string(types.DatasetRepo) || ref != ("refs/heads/"+component.MainBranch) {
		return
	}
	for _, fileName := range fileNames {
		if !strings.HasSuffix(strings.ToLower(fileName), ".jsonl") {
			continue
		}
		getFileReq := gitserver.GetRepoInfoByPathReq{
			Namespace: namespace,
			Name:      repoName,
			Ref:       component.MainBranch,
			Path:      fileName,
			RepoType:  types.DatasetRepo,
			File:      true,
		}
		files, err := c.gs.GetRepoFileTree(ctx, getFileReq)
		if err != nil {
			slog.Warn("fail to get file object for prompt", slog.Any("getFileReq", getFileReq), slog.Any("error", err))
			continue
		}
		if files == nil || len(files) < 1 {
			slog.Warn("did not find file object for prompt", slog.Any("getFileReq", getFileReq), slog.Any("error", err))
			continue
		}
		if files[0].Lfs || files[0].Size > c.maxPromptFS {
			continue
		}
		_, err = c.pp.ParseJsonFile(ctx, getFileReq)
		if err != nil {
			slog.Warn("file is not prompt format jsonl", slog.Any("getFileReq", getFileReq), slog.Any("error", err))
			continue
		}
		repo, err := c.rs.FindByPath(ctx, types.DatasetRepo, namespace, repoName)
		if err != nil || repo == nil {
			slog.Error("fail to query repo for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
			continue
		}
		err = c.ds.SetPromptType(ctx, repo.ID)
		if err != nil {
			slog.Error("fail to set prompt type for git callback", slog.Any("repo id", repo.ID), slog.Any("error", err))
		} else {
			return
		}
	}
}

func (c *GitCallbackComponent) removePromptDatasetType(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) {
	slog.Debug("removePromptDatasetType", slog.Any("fileNames", fileNames), slog.Any("repoType", repoType), slog.Any("ref", ref))
	if len(fileNames) < 1 {
		return
	}
	repoType = strings.TrimRight(repoType, "s")
	if repoType != string(types.DatasetRepo) || ref != ("refs/heads/"+component.MainBranch) {
		return
	}
	tree, err := component.GetFilePathObjects(namespace, repoName, "", types.DatasetRepo, ref, c.gs.GetRepoFileTree)
	if err != nil || tree == nil {
		slog.Error("failed to get repo file tree for remove prompt type, ", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return
	}

	for _, file := range tree {
		if file.Lfs || file.Size > c.maxPromptFS {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(file.Path), ".jsonl") {
			continue
		}
		getFileReq := gitserver.GetRepoInfoByPathReq{
			Namespace: namespace,
			Name:      repoName,
			Ref:       component.MainBranch,
			Path:      file.Path,
			RepoType:  types.DatasetRepo,
		}
		_, err = c.pp.ParseJsonFile(ctx, getFileReq)
		if err != nil {
			slog.Warn("file is not prompt format jsonl", slog.Any("getFileReq", getFileReq), slog.Any("error", err))
		} else {
			return
		}
	}

	repo, err := c.rs.FindByPath(ctx, types.DatasetRepo, namespace, repoName)
	if err != nil || repo == nil {
		slog.Error("fail to query repo for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return
	}
	err = c.ds.SetNormalDatasetType(ctx, repo.ID)
	if err != nil {
		slog.Error("fail to set dataset non-prompt type for git callback", slog.Any("repo id", repo.ID), slog.Any("error", err))
	}
}
