package callback

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/sensitive"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/component"
)

const (
	DatasetRepoType = "datasets"
	ReadmeFileName  = "README.md"
)

// define GitCallbackComponent struct
type GitCallbackComponent struct {
	config  *config.Config
	gs      gitserver.GitServer
	tc      *component.TagComponent
	checker sensitive.SensitiveChecker
	ms      *database.ModelStore
	ds      *database.DatasetStore
	//set visibility if file content is sensitive
	setRepoVisibility bool
}

// new CallbackComponent
func NewGitCallback(config *config.Config) (*GitCallbackComponent, error) {
	gs, err := gitserver.NewGitServer(config)
	if err != nil {
		return nil, err
	}
	tc, err := component.NewTagComponent(config)
	if err != nil {
		return nil, err
	}
	ms := database.NewModelStore()
	ds := database.NewDatasetStore()
	return &GitCallbackComponent{
		config:  config,
		gs:      gs,
		tc:      tc,
		ms:      ms,
		ds:      ds,
		checker: sensitive.NewAliyunGreenChecker(config),
	}, nil
}

// SetRepoVisibility sets a flag whether change repo's visibility if file content is sensitive
func (c *GitCallbackComponent) SetRepoVisibility(yes bool) {
	c.setRepoVisibility = yes
}

func (c *GitCallbackComponent) HandlePush(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	commits := req.Commits
	ref := req.Ref
	//split req.Repository.FullName by '/'
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

// modifyFiles method handles modified files, skip if not modify README.md
func (c *GitCallbackComponent) modifyFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	for _, fileName := range fileNames {
		slog.Debug("modify file", slog.String("file", fileName))
		//only care about readme file under root directory
		if fileName != ReadmeFileName {
			continue
		}

		content, err := c.getFileRaw(repoType, namespace, repoName, ref, fileName)
		if err != nil {
			return err
		}
		if c.setRepoVisibility {
			go func(content string) {
				ok, err := c.checkFileContent(ctx, repoType, namespace, repoName, ref, content)
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
		//should be only one README.md
		return c.updateMetaTags(ctx, repoType, namespace, repoName, ref, content)
	}
	return nil
}

func (c *GitCallbackComponent) removeFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	//handle removed files
	//delete tagss
	for _, fileName := range fileNames {
		slog.Debug("remove file", slog.String("file", fileName))
		//only care about readme file under root directory
		if fileName == ReadmeFileName {
			//use empty content to clear all the meta tags
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
			if repoType == "datasets" {
				tagScope = database.DatasetTagScope
			} else {
				tagScope = database.ModelTagScope
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
		//only care about readme file under root directory
		if fileName == ReadmeFileName {
			content, err := c.getFileRaw(repoType, namespace, repoName, ref, fileName)
			if err != nil {
				return err
			}
			if c.setRepoVisibility {
				go func(content string) {
					ok, err := c.checkFileContent(ctx, repoType, namespace, repoName, ref, content)
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
			if repoType == DatasetRepoType {
				tagScope = database.DatasetTagScope
			} else {
				tagScope = database.ModelTagScope
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
	if repoType == DatasetRepoType {
		tagScope = database.DatasetTagScope
	} else {
		tagScope = database.ModelTagScope
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
	if repoType == DatasetRepoType {
		content, err = c.gs.GetDatasetFileRaw(namespace, repoName, ref, fileName)
	} else {
		content, err = c.gs.GetModelFileRaw(namespace, repoName, ref, fileName)
	}
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

func (c *GitCallbackComponent) checkFileContent(ctx context.Context, repoType, namespace, repoName, ref, content string) (bool, error) {

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
	return c.checker.PassTextCheck(ctx, "comment_detection", content)
}

func (c *GitCallbackComponent) setPrivate(ctx context.Context, repoType, namespace, repoName string) error {
	var err error
	var dataset *database.Dataset
	var model *database.Model
	if repoType == DatasetRepoType {
		dataset, err = c.ds.FindByPath(ctx, namespace, repoName)
		err = c.gs.UpdateDatasetRepo(namespace, repoName, dataset, dataset.Repository, &types.UpdateDatasetReq{
			Name:          dataset.Name,
			Description:   dataset.Description,
			Private:       true,
			DefaultBranch: dataset.Repository.DefaultBranch,
		})
		if err != nil {
			return fmt.Errorf("failed to update git server dataset to private, error: %w", err)
		}
		err = c.ds.Update(ctx, dataset, dataset.Repository)
		if err != nil {
			return fmt.Errorf("failed to update database dataset to private, error: %w", err)
		}
	} else {
		model, err = c.ms.FindByPath(ctx, namespace, repoName)
		err = c.gs.UpdateModelRepo(namespace, repoName, model, model.Repository, &types.UpdateModelReq{
			Name:          model.Name,
			Description:   model.Description,
			Private:       true,
			DefaultBranch: model.Repository.DefaultBranch,
		})
		if err != nil {
			return fmt.Errorf("failed to update git server model to private, error: %w", err)
		}
		err = c.ms.Update(ctx, model, model.Repository)
		if err != nil {
			return fmt.Errorf("failed to update database model to private, error: %w", err)
		}
	}
	slog.Info("set repository to private successed.", slog.String("repoType", repoType), slog.String("namespace", namespace),
		slog.String("repo", repoName))

	return err
}
