package callback

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type spaceDeployWatcher struct {
	ops []func() error
	ss  *database.SpaceStore
	sc  *component.SpaceComponent
}

func WatchSpaceChange(req *types.GiteaCallbackPushReq, ss *database.SpaceStore, sc *component.SpaceComponent) Watcher {
	watcher := new(spaceDeployWatcher)
	watcher.ss = ss
	watcher.sc = sc
	// split req.Repository.FullName by '/' for example: <repotype>_<namespace>/<reponame>
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")

	if repoType != "spaces" {
		return watcher
	}
	// username = namespace in fullname of gitea
	watcher.deploy(namespace, repoName, namespace)
	return watcher
}

func (w *spaceDeployWatcher) Run() error {
	var err error
	for _, op := range w.ops {
		errors.Join(err, op())
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
				return fmt.Errorf("failed to find space: %w", err)
			}

			w.sc.FixHasEntryFile(ctx, space)
			if !space.HasAppFile {
				return nil
			}
			// trigger space deployment by gitea call back
			_, err = w.sc.Deploy(ctx, namespace, repoName, currentUser)
			if err != nil {
				return fmt.Errorf("failed to trigger space delopy: %w", err)
			} else {
				return nil
			}
		})

	return w
}
