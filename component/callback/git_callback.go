package callback

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/component"
)

// define GitCallbackComponent struct
type GitCallbackComponent struct {
	config *config.Config
	gs     gitserver.GitServer
	tc     *component.TagComponent
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
	return &GitCallbackComponent{
		config: config,
		gs:     gs,
		tc:     tc,
	}, nil
}

func (c *GitCallbackComponent) HandlePush(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	commits := req.Commits
	ref := req.Ref
	//split req.Repository.FullName by '/'
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")
	for _, commit := range commits {
		c.modifyFiles(ctx, repoType, namespace, repoName, ref, commit.Modified)
		c.removeFiles(ctx, repoType, namespace, repoName, ref, commit.Removed)
		c.addFiles(ctx, repoType, namespace, repoName, ref, commit.Added)
	}
	return nil
}

// modifyFiles method handles modified files, skip if not modify README.md
func (c *GitCallbackComponent) modifyFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	for _, fileName := range fileNames {
		slog.Debug("modify file", slog.String("file", fileName))
		//only care about readme file under root directory
		if fileName != "README.md" {
			continue
		}

		//should be only one README.md
		return c.updateMetaTags(ctx, repoType, namespace, repoName, ref, fileName)
	}
	return nil
}

func (c *GitCallbackComponent) removeFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	//handle removed files
	//delete tagss
	for _, fileName := range fileNames {
		slog.Debug("remove file", slog.String("file", fileName))
		//only care about readme file under root directory
		if fileName == "README.md" {
			//use empty content to clear all the meta tags
			const content string = ""
			err := c.tc.ClearMetaTags(ctx, namespace, repoName)
			if err != nil {
				slog.Error("failed to clear meta tags", slog.String("namespace", namespace),
					slog.String("content", content), slog.String("repo", repoName), slog.String("ref", ref),
					slog.Any("error", err))
				return fmt.Errorf("failed to update met tags,cause: %w", err)
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
		if fileName == "README.md" {
			err := c.updateMetaTags(ctx, repoType, namespace, repoName, ref, fileName)
			if err != nil {
				return err
			}
		} else {
			var tagScope database.TagScope
			if repoType == "datasets" {
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

func (c *GitCallbackComponent) updateMetaTags(ctx context.Context, repoType, namespace, repoName, ref, fileName string) error {
	var (
		content  string
		err      error
		tagScope database.TagScope
	)
	if repoType == "datasets" {
		content, err = c.gs.GetDatasetFileRaw(namespace, repoName, ref, fileName)
		tagScope = database.DatasetTagScope
	} else {
		content, err = c.gs.GetModelFileRaw(namespace, repoName, ref, fileName)
		tagScope = database.ModelTagScope
	}
	if err != nil {
		slog.Error("failed to get file content", slog.String("namespace", namespace),
			slog.String("file", fileName), slog.String("repo", repoName), slog.String("ref", ref),
			slog.Any("error", err))
		return fmt.Errorf("failed to get file content,cause: %w", err)
	}
	slog.Debug("get file content success", slog.String("repoType", repoType), slog.String("namespace", namespace),
		slog.String("file", fileName), slog.String("repo", repoName), slog.String("ref", ref))

	_, err = c.tc.UpdateMetaTags(ctx, tagScope, namespace, repoName, content)
	if err != nil {
		slog.Error("failed to update meta tags", slog.String("namespace", namespace),
			slog.String("content", content), slog.String("repo", repoName), slog.String("ref", ref),
			slog.Any("error", err))
		return fmt.Errorf("failed to update met tags,cause: %w", err)
	}
	return nil
}
