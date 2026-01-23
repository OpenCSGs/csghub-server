package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
)

func TestRepoComponent_DeletePendingDeletion(t *testing.T) {
	mockPendingDeletion := mockdb.NewMockPendingDeletionStore(t)
	mockGit := mockgit.NewMockGitServer(t)
	repoComp := &repoComponentImpl{
		pendingDeletion: mockPendingDeletion,
		git:             mockGit,
	}

	pd := &database.PendingDeletion{
		TableName: database.PendingDeletionTableNameRepository,
		Value:     "repo1",
	}

	mockPendingDeletion.EXPECT().FindByTableNameWithBatch(
		context.Background(),
		database.PendingDeletionTableNameRepository,
		1000,
		0,
	).Return([]*database.PendingDeletion{pd}, nil)
	mockGit.EXPECT().DeleteRepo(context.Background(), "repo1").Return(nil)

	mockPendingDeletion.EXPECT().Delete(context.Background(), pd).Return(nil)

	err := repoComp.DeletePendingDeletion(context.Background())
	require.Nil(t, err)
}
