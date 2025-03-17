package component

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/d5/tengo/v2"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

type recomComponentImpl struct {
	recomStore database.RecomStore
	repoStore  database.RepoStore
	gitServer  gitserver.GitServer
}

type RecomComponent interface {
	SetOpWeight(ctx context.Context, repoID, weight int64) error
	// loop through repositories and calculate the recom score of the repository
	CalculateRecomScore(ctx context.Context)
	CalcTotalScore(ctx context.Context, repo *database.Repository, weights map[string]string) float64
}

func NewRecomComponent(cfg *config.Config) (RecomComponent, error) {
	gs, err := git.NewGitServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init git server,%w", err)
	}

	return &recomComponentImpl{
		recomStore: database.NewRecomStore(),
		repoStore:  database.NewRepoStore(),
		gitServer:  gs,
	}, nil
}

func (rc *recomComponentImpl) SetOpWeight(ctx context.Context, repoID, weight int64) error {
	_, err := rc.repoStore.FindById(ctx, repoID)
	if err != nil {
		return fmt.Errorf("failed to find repository with id %d, err:%w", repoID, err)
	}
	return rc.recomStore.UpsetOpWeights(ctx, repoID, weight)
}

// loop through repositories and calculate the recom score of the repository
func (rc *recomComponentImpl) CalculateRecomScore(ctx context.Context) {
	weights, err := rc.loadWeights()
	if err != nil {
		slog.Error("Error loading weights", "error", err)
		return
	}
	repos, err := rc.repoStore.All(ctx)
	if err != nil {
		slog.Error("Error fetching repositories", "error", err)
		return
	}
	for _, repo := range repos {
		repoID := repo.ID
		score := rc.CalcTotalScore(ctx, repo, weights)
		err := rc.recomStore.UpsertScore(ctx, repoID, score)
		if err != nil {
			slog.Error("Error updating recom score", slog.Int64("repo_id", repoID), slog.Float64("score", score),
				slog.String("error", err.Error()))
		}
	}
}

func (rc *recomComponentImpl) CalcTotalScore(ctx context.Context, repo *database.Repository, weights map[string]string) float64 {
	score := float64(0)

	if freshness, ok := weights["freshness"]; ok {
		score += rc.calcFreshnessScore(repo.CreatedAt, freshness)
	}

	if downloads, ok := weights["downloads"]; ok {
		score += rc.calcDownloadsScore(repo.DownloadCount, downloads)
	}

	qualityScore, err := rc.calcQualityScore(ctx, repo)
	if err != nil {
		slog.Error("failed to calculate quality score", slog.Any("error", err))
	} else {
		score += qualityScore
	}

	return score
}

func (rc *recomComponentImpl) calcFreshnessScore(createdAt time.Time, weightExp string) float64 {
	// TODO:cache compiled script
	hours := time.Since(createdAt).Hours()
	scriptFreshness := tengo.NewScript([]byte(weightExp))
	_ = scriptFreshness.Add("score", 0.0)
	_ = scriptFreshness.Add("hours", 0)
	sc, err := scriptFreshness.Compile()
	if err != nil {
		panic(err)
	}
	_ = sc.Set("hours", hours)
	err = sc.Run()
	if err != nil {
		panic(err)
	}

	return sc.Get("score").Float()
}

func (rc *recomComponentImpl) calcDownloadsScore(downloads int64, weightExp string) float64 {
	// TODO:cache compiled script
	scriptFreshness := tengo.NewScript([]byte(weightExp))
	_ = scriptFreshness.Add("score", 0.0)
	_ = scriptFreshness.Add("downloads", 0)
	sc, err := scriptFreshness.Compile()
	if err != nil {
		panic(err)
	}
	_ = sc.Set("downloads", downloads)
	err = sc.Run()
	if err != nil {
		panic(err)
	}

	return sc.Get("score").Float()
}

func (rc *recomComponentImpl) calcQualityScore(ctx context.Context, repo *database.Repository) (float64, error) {
	score := 0.0
	// get file counts from git server
	namespace, name := repo.NamespaceAndName()
	files, err := GetFilePaths(ctx, namespace, name, "", repo.RepositoryType, "", rc.gitServer.GetTree)
	if err != nil {
		return 0, fmt.Errorf("failed to get repo file tree,%w", err)
	}
	fileCount := len(files)
	for _, f := range files {
		if f == "README.md" {
			fileCount--
		}
		if f == "LICENSE" {
			fileCount--
		}
		if f == ".gitattributes" {
			fileCount--
		}
	}

	if fileCount >= 2 {
		score += 300.0
	}
	return score, nil
}

func (rc *recomComponentImpl) loadWeights() (map[string]string, error) {
	ctx := context.Background()
	items, err := rc.recomStore.LoadWeights(ctx)
	if err != nil {
		return nil, err
	}

	weights := make(map[string]string)
	for _, item := range items {
		weights[item.Name] = item.WeightExp
	}
	return weights, nil
}

// func (rc *recomComponentImpl) loadOpWeights() (map[int64]int, error) {
// 	ctx := context.Background()
// 	items, err := rc.rs.LoadOpWeights(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	weights := make(map[int64]int)
// 	for _, item := range items {
// 		weights[item.RepositoryID] = item.Weight
// 	}
// 	return weights, nil
// }
