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
	// set visibility if file content is sensitive
	setRepoVisibility bool
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
	if err != nil {
		return nil, err
	}
	svGen := NewSyncVersionGenerator()
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
	return nil
}

func (c *GitCallbackComponent) removeFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	// handle removed files
	// delete tagss
	for _, fileName := range fileNames {
		slog.Debug("remove file", slog.String("file", fileName))
		// only care about readme file under root directory
		if fileName == ReadmeFileName {
			// use empty content to clear all the meta tags
			const content string = ""
			err := c.tc.ClearMetaTags(ctx, namespace, repoName)
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
	return nil
}

func (c *GitCallbackComponent) addFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	for _, fileName := range fileNames {
		slog.Debug("add file", slog.String("file", fileName))
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
