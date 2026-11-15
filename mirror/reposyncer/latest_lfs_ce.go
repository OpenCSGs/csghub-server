//go:build !ee && !saas

package reposyncer

func (w *RepoSyncWorker) initLatestLFS() {}
