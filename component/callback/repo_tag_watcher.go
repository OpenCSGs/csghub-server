package callback

import (
	"errors"
	"slices"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

type repoTagWatcher struct {
	ops []func() error
}

func WatchRepoTag(req *types.GiteaCallbackPushReq) Watcher {
	watcher := new(repoTagWatcher)

	commits := req.Commits
	ref := req.Ref
	// split req.Repository.FullName by '/'
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")
	for _, commit := range commits {
		if slices.Contains(commit.Modified, ReadmeFileName) {
			watcher.modify(namespace, repoName, repoType, ref)
			continue
		}
		if slices.Contains(commit.Added, ReadmeFileName) {
			watcher.add(namespace, repoName, repoType, ref)
			continue
		}
		if slices.Contains(commit.Removed, ReadmeFileName) {
			watcher.del(namespace, repoName, repoType, ref)
			continue
		}
	}

	return watcher
}

func (w *repoTagWatcher) Run() error {
	var err error
	for _, op := range w.ops {
		err = errors.Join(op())
	}
	return err
}

func (w *repoTagWatcher) modify(namespace, name, repoType, ref string) *repoTagWatcher {
	w.ops = append(w.ops,
		func() error {
			return nil
		})
	return w
}

func (w *repoTagWatcher) add(namespace, name, repoType, ref string) *repoTagWatcher {
	w.ops = append(w.ops,
		func() error {
			return nil
		})
	return w
}

func (w *repoTagWatcher) del(namespace, name, repoType, ref string) *repoTagWatcher {
	w.ops = append(w.ops,
		func() error {
			return nil
		})
	return w
}
