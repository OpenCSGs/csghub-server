package callback

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/tagparser"
)

type repoRelationWatcher struct {
	ops []func() error
	rs  *database.RepoStore
	rrs *database.RepoRelationsStore
	gs  gitserver.GitServer

	readmeStatus string
}

func WatchRepoRelation(req *types.GiteaCallbackPushReq, ss *database.RepoStore,
	rrs *database.RepoRelationsStore,
	gs gitserver.GitServer) Watcher {
	watcher := new(repoRelationWatcher)
	watcher.rs = ss
	watcher.rrs = rrs
	watcher.gs = gs

	//only care about main branch
	if req.Ref != "refs/heads/main" {
		return watcher
	}
	// split req.Repository.FullName by '/'
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")

	if repoType == CodeRepoType || repoType == DatasetRepoType {
		return watcher
	}

	commits := req.Commits
	ref := req.Ref
	for _, commit := range commits {
		if slices.Contains(commit.Modified, ReadmeFileName) {
			watcher.readmeStatus = "modified"
			continue
		}
		if slices.Contains(commit.Added, ReadmeFileName) {
			if watcher.readmeStatus != "modified" {
				watcher.readmeStatus = "added"
			}
			continue
		}
		if slices.Contains(commit.Removed, ReadmeFileName) {
			watcher.readmeStatus = "removed"
			continue
		}
	}

	//readme file not changed in this whole push, so do nothing
	if watcher.readmeStatus == "" {
		return watcher
	}

	watcher.regenerate(namespace, repoName, repoType, ref)

	return watcher
}

func (w *repoRelationWatcher) Run() error {
	var err error
	for _, op := range w.ops {
		errors.Join(err, op())
	}
	return err
}

func (w *repoRelationWatcher) toRepoIDsFromReadme(namespace, repoName, repoType, ref string) ([]int64, error) {
	var readme string
	var err error
	var toRepoIDs []int64
	var paths []string

	readme, err = w.getFileRaw(repoType, namespace, repoName, ref, ReadmeFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to get readme file,%w", err)
	}
	meta, err := tagparser.MetaTags(readme)
	if err != nil {
		return nil, fmt.Errorf("failed to parse readme file meta,%w", err)
	}
	if repoType == ModelRepoType {
		datasetItems := meta["datasets"]
		codeItems := meta["codes"]
		for _, datasetItem := range datasetItems {
			paths = append(paths, fmt.Sprintf("%s%s", "datasets_", datasetItem))
		}
		for _, codeItem := range codeItems {
			paths = append(paths, fmt.Sprintf("%s%s", "codes_", codeItem))
		}
	}
	if repoType == SpaceRepoType {
		modelItems := meta["models"]
		if len(modelItems) == 0 {
			return toRepoIDs, nil
		}
		for _, modelItem := range modelItems {
			paths = append(paths, fmt.Sprintf("%s%s", "models_", modelItem))
		}
	}

	if len(paths) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		toRepos, err := w.rs.FindByGitPaths(ctx, paths, database.Columns("id"))
		cancel()
		if err != nil {
			return nil, fmt.Errorf("failed to find repos by paths,%w", err)
		}
		for _, repo := range toRepos {
			toRepoIDs = append(toRepoIDs, repo.ID)
		}

	}
	return toRepoIDs, nil
}
func (w *repoRelationWatcher) regenerate(namespace, repoName, repoType, ref string) *repoRelationWatcher {
	w.ops = append(w.ops,
		func() error {
			var fromRepoID int64
			var err error
			var toRepoIDs []int64

			if w.readmeStatus != "removed" {
				toRepoIDs, err = w.toRepoIDsFromReadme(namespace, repoName, repoType, ref)
				if err != nil {
					return fmt.Errorf("failed to get relation to repos from readme,%w", err)
				}
			}
			//TODO: get to repo ids from app.py by parsing model ids

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()

			if repoType == ModelRepoType {
				fromRepo, err := w.rs.FindByPath(ctx, types.ModelRepo, namespace, repoName)
				if err != nil {
					return fmt.Errorf("failed to find model repo,%w", err)
				}
				fromRepoID = fromRepo.ID

			}

			if repoType == SpaceRepoType {
				fromRepo, err := w.rs.FindByPath(ctx, types.SpaceRepo, namespace, repoName)
				if err != nil {
					return fmt.Errorf("failed to find space repo,%w", err)
				}
				fromRepoID = fromRepo.ID
			}

			return w.rrs.Override(ctx, fromRepoID, toRepoIDs...)
		})

	return w
}

func (w *repoRelationWatcher) getFileRaw(repoType, namespace, repoName, ref, fileName string) (string, error) {
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
	content, err = w.gs.GetRepoFileRaw(context.Background(), getFileRawReq)
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
