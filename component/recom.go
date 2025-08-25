package component

import (
	"context"
	"errors"
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
	userStore  database.UserStore
	repoStore  database.RepoStore
	gitServer  gitserver.GitServer
}

type RecomComponent interface {
	SetOpWeight(ctx context.Context, repoID, weight int64) error
	// loop through repositories and calculate the recom score of the repository
	CalculateRecomScore(ctx context.Context, batchSize int) error
	// CalcTotalScore(ctx context.Context, repo *database.Repository, weights map[string]string) (float64, error)
}

func NewRecomComponent(cfg *config.Config) (RecomComponent, error) {
	gs, err := git.NewGitServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init git server,%w", err)
	}

	return &recomComponentImpl{
		recomStore: database.NewRecomStore(),
		userStore:  database.NewUserStore(),
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
func (rc *recomComponentImpl) CalculateRecomScore(ctx context.Context, batchSize int) error {
	weights, err := rc.loadWeights()
	if err != nil {
		return errors.New("error loading weights")
	}
	if batchSize <= 0 {
		batchSize = 500
	}
	lastRepoID := int64(0)
	for {
		repos, err := rc.repoStore.BatchGet(ctx, lastRepoID, batchSize, nil)
		if err != nil {
			return errors.New("error fetching repositories")
		}
		var repoIDs []int64
		for _, repo := range repos {
			repoIDs = append(repoIDs, repo.ID)
		}
		scores, err := rc.recomStore.FindScoreByRepoIDs(ctx, repoIDs)
		if err != nil {
			return errors.New("error fetching scores of repos")
		}
		// scores to map
		scoresMap := make(map[int64]map[database.RecomWeightName]*database.RecomRepoScore)
		for _, score := range scores {
			if _, ok := scoresMap[score.RepositoryID]; !ok {
				scoresMap[score.RepositoryID] = make(map[database.RecomWeightName]*database.RecomRepoScore)
			}
			scoresMap[score.RepositoryID][score.WeightName] = score
		}
		var newScores []*database.RecomRepoScore
		for _, repo := range repos {
			repoID := repo.ID
			oldRepoScores := scoresMap[repoID]
			newRepoScores, err := rc.calcTotalScore(ctx, &repo, weights, oldRepoScores)
			if err != nil {
				slog.Error("failed to calc total score, skip", slog.Int64("repo_id", repoID), slog.String("error", err.Error()))
				continue
			}
			newScores = append(newScores, newRepoScores...)
		}

		err = rc.recomStore.UpsertScore(ctx, newScores)
		if err != nil {
			slog.Error("failed to flush recom score", slog.Any("error", err), slog.Any("repo_ids", repoIDs))
		} else {
			slog.Info("flush recom score success", slog.Any("repo_ids", repoIDs))
		}

		if len(repos) < batchSize {
			break
		}

		// Update lastRepoID to the ID of the last repository in this batch
		if len(repos) > 0 {
			lastRepoID = repos[len(repos)-1].ID
		}
	}

	return nil
}

func (rc *recomComponentImpl) calcTotalScore(ctx context.Context, repo *database.Repository, weights map[database.RecomWeightName]string, oldScores map[database.RecomWeightName]*database.RecomRepoScore) ([]*database.RecomRepoScore, error) {
	scores := make([]*database.RecomRepoScore, 0)

	// weight freshness
	fscore, ok := oldScores[database.RecomWeightFreshness]
	if !ok {
		fscore = &database.RecomRepoScore{RepositoryID: repo.ID, WeightName: database.RecomWeightFreshness, Score: 0}
	}

	if freshness, ok := weights[database.RecomWeightFreshness]; ok {
		fscore.Score = rc.calcFreshnessScore(repo.CreatedAt, freshness)
	} else {
		// reset weight score if weights removed from system
		fscore.Score = 0
	}
	scores = append(scores, fscore)

	// weight downloads
	dscore, ok := oldScores[database.RecomWeightDownloads]
	if !ok {
		dscore = &database.RecomRepoScore{RepositoryID: repo.ID, WeightName: database.RecomWeightDownloads, Score: 0}
	}
	if downloads, ok := weights[database.RecomWeightDownloads]; ok {
		dscore.Score = rc.calcDownloadsScore(repo.DownloadCount, downloads)
	} else {
		// reset weight score if weights removed from system
		dscore.Score = 0
	}
	scores = append(scores, dscore)

	// weight quality
	qscore, ok := oldScores[database.RecomWeightQuality]
	if !ok {
		qscore = &database.RecomRepoScore{RepositoryID: repo.ID, WeightName: database.RecomWeightQuality, Score: 0}
	}
	if repo.UpdatedAt.After(qscore.UpdatedAt) {
		qualityScore, err := rc.calcQualityScore(ctx, repo)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate quality score,error:%w", err)
		} else {
			qscore.Score = qualityScore
		}
	}
	scores = append(scores, qscore)

	for weightName, oldScore := range oldScores {
		if weightName == database.RecomWeightFreshness || weightName == database.RecomWeightDownloads || weightName == database.RecomWeightQuality ||
			//total weight score will be recalculated in next step
			weightName == database.RecomWeightTotal {
			continue
		}

		// keep other weight scores not calculated, like 'op' weight score
		scores = append(scores, oldScore)
	}

	// recalculate total score
	total := 0.0
	for _, score := range scores {
		total += score.Score
	}

	scores = append(scores, &database.RecomRepoScore{RepositoryID: repo.ID, WeightName: database.RecomWeightTotal, Score: total})
	return scores, nil
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

func (rc *recomComponentImpl) loadWeights() (map[database.RecomWeightName]string, error) {
	ctx := context.Background()
	items, err := rc.recomStore.LoadWeights(ctx)
	if err != nil {
		return nil, err
	}

	weights := make(map[database.RecomWeightName]string)
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
