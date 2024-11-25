package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	gsmock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func NewTestRecomComponent(stores *tests.MockStores, gitServer gitserver.GitServer) *recomComponentImpl {
	return &recomComponentImpl{
		repos: stores.Repo,
		gs:    gitServer,
	}
}

func TestRecomComponent_CalculateTotalScore(t *testing.T) {
	gitServer := gsmock.NewMockGitServer(t)
	rc := &recomComponentImpl{gs: gitServer}
	ctx := context.TODO()

	gitServer.EXPECT().GetRepoFileTree(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "foo",
		Name:      "bar",
	}).Return(nil, nil)

	// Test case 1: repository created 24 hours ago
	repo1 := &database.Repository{Path: "foo/bar"}
	repo1.CreatedAt = time.Now().Add(-24 * time.Hour)
	weights1 := map[string]string{
		"freshness": expFreshness,
	}
	score1 := rc.CalcTotalScore(ctx, repo1, weights1)
	if score1 > 100 || score1 < 98 {
		t.Errorf("Expected score1 should in range [98,100], got: %f", score1)
	}

	// Test case 2: repository created 48 hours ago
	repo2 := &database.Repository{Path: "foo/bar"}
	repo2.CreatedAt = time.Now().Add(-50 * time.Hour)
	weights2 := map[string]string{
		"freshness": expFreshness,
	}
	score2 := rc.CalcTotalScore(ctx, repo2, weights2)
	if score2 > 98.0 || score2 < 60.0 {
		t.Errorf("Expected score1 should in range [60,98), got: %f", score2)
	}

	// Test case 3: repository created 168 hours ago
	repo3 := &database.Repository{Path: "foo/bar"}
	repo3.CreatedAt = time.Now().Add(-168 * time.Hour)
	weights3 := map[string]string{
		"freshness": expFreshness,
	}
	score3 := rc.CalcTotalScore(ctx, repo3, weights3)
	if score3 < 0 || score3 > 60 {
		t.Errorf("Expected score1 should in range [0,60), got: %f", score2)
	}
}

const expFreshness = `
if hours <= 48{
	score = 100 - 2.0/48.0 * hours
} else if hours <= 168{
	score = 98 - 38.0/120.0 * (hours - 48)
} else{
	score = 60 * (1 / (((hours - 168) / 48.0) + 1))
}
`

// const expDownloads = `
// if downloads <= 10{
//     score = downloads * (10 / 10)
// }else if downloads <= 100{
//     score = 10 + (downloads - 10) * (40.0 / 90.0)
// }else if downloads <= 1000{
//     score = 50 + (downloads - 100) * (40.0 / 900.0)
// }else{
//     score = 90 + 10 * (1 - (1 / (downloads / 1000.0)))
// }
// `
