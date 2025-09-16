package callback

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

var nonPipelineTriggerFiles = []string{
	types.ReadmeFileName,
	types.GitAttributesFileName,
	types.Gitignore,
}

type spaceDeployWatcher struct {
	ops []func() error
	ss  database.SpaceStore
	sc  component.SpaceComponent
}

func WatchSpaceChange(req *types.GiteaCallbackPushReq, ss database.SpaceStore, sc component.SpaceComponent) Watcher {
	watcher := new(spaceDeployWatcher)
	watcher.ss = ss
	watcher.sc = sc
	// split req.Repository.FullName by '/' for example: <repotype>_<namespace>/<reponame>
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")

	if repoType != "spaces" {
		slog.Warn("[git_callback] not a space repo type and skip space deploy watcher")
		return watcher
	}
	if onlyNonDeployFilesChanged(req.Commits) {
		slog.Warn("[git_callback] only non-deploy files changed and skip space deploy watcher",
			slog.Any("namespace", namespace), slog.Any("repoName", repoName))
		return watcher
	}

	// username = namespace in fullname of gitea
	slog.Info("[git_callback] create space deploy tasks", slog.Any("namespace", namespace), slog.Any("repoName", repoName))
	watcher.deploy(namespace, repoName, namespace)
	return watcher
}

func (w *spaceDeployWatcher) Run() error {
	var err error
	for _, op := range w.ops {
		err = errors.Join(err, op())
	}
	return err
}

func (w *spaceDeployWatcher) deploy(namespace string, repoName string, currentUser string) *spaceDeployWatcher {
	w.ops = append(w.ops,
		func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()

			space, err := w.ss.FindByPath(ctx, namespace, repoName)
			if err != nil {
				return fmt.Errorf("[git_callback] failed to find space %s/%s error: %w", namespace, repoName, err)
			}

			w.sc.FixHasEntryFile(ctx, space)
			if !space.HasAppFile {
				slog.Warn("[git_callback] no app file found and skip space deploy", slog.Any("namespace", namespace), slog.Any("repoName", repoName))
				return nil
			}
			// trigger space deployment by gitea call back
			_, err = w.sc.Deploy(ctx, namespace, repoName, currentUser)
			if err != nil {
				return fmt.Errorf("[git_callback] failed to trigger space %s/%s deploy error: %w", namespace, repoName, err)
			} else {
				return nil
			}
		})

	return w
}

func isNonDeployTriggerFile(filename string) bool {
	for _, f := range nonPipelineTriggerFiles {
		if f == filename {
			return true
		}
	}
	return false
}

func onlyNonDeployFilesChanged(commits []types.GiteaCallbackPushReq_Commit) bool {
	for _, commit := range commits {
		allFiles := append(append([]string{}, commit.Added...), commit.Modified...)
		allFiles = append(allFiles, commit.Removed...)

		for _, file := range allFiles {
			if !isNonDeployTriggerFile(file) {
				return false
			}
		}
	}
	return true
}
