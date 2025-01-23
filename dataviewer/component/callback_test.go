package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	dvCom "opencsg.com/csghub-server/dataviewer/common"

	mockGit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mock_temporal "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
)

var _ client.WorkflowRun = (*MockWorkflowRun)(nil)

type MockWorkflowRun struct {
}

func (mw *MockWorkflowRun) GetID() string {
	return "113"
}

func (mw *MockWorkflowRun) GetRunID() string {
	return "234"
}
func (mw *MockWorkflowRun) Get(ctx context.Context, valuePtr interface{}) error {
	return nil
}

func (mw *MockWorkflowRun) GetWithOptions(ctx context.Context, valuePtr interface{}, options client.WorkflowRunGetOptions) error {
	return nil
}

func NewTestNewCallbackComponent(cfg *config.Config,
	tc temporal.Client, gs gitserver.GitServer,
	rs database.RepoStore, ds database.DataviewerStore,
) CallbackComponent {
	abc := &callbackComponentImpl{
		cfg:             cfg,
		workflowClient:  tc,
		gitServer:       gs,
		repoStore:       rs,
		dataviewerStore: ds,
	}
	return abc
}

func TestDatasetViewerComppnent_TriggerWorkflow(t *testing.T) {
	ctx := context.TODO()

	config := &config.Config{}

	req := types.UpdateViewerReq{
		Namespace: "test-ns",
		Name:      "test-name",
		Branch:    "test-branch",
		RepoType:  types.DatasetRepo,
		RepoID:    int64(1),
	}

	repo := mockdb.NewMockRepoStore(t)
	repo.EXPECT().FindByPath(ctx, req.RepoType, req.Namespace, req.Name).Return(&database.Repository{
		ID:            int64(1),
		DefaultBranch: "test-branch",
	}, nil)

	dvstore := mockdb.NewMockDataviewerStore(t)

	dvstore.EXPECT().GetRunningJobsByRepoID(ctx, int64(1)).Return([]database.DataviewerJob{}, nil)

	dvstore.EXPECT().GetViewerByRepoID(ctx, int64(1)).Return(&database.Dataviewer{
		ID:         int64(1),
		RepoID:     int64(1),
		WorkflowID: "test-workflow-id",
	}, nil)

	dvstore.EXPECT().CreateJob(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, job database.DataviewerJob) error {
		require.NotNil(t, job.WorkflowID)
		return nil
	})

	mockGitServer := mockGit.NewMockGitServer(t)

	mtc := mock_temporal.NewMockClient(t)

	mtc.EXPECT().ExecuteWorkflow(
		mock.Anything, mock.Anything,
		mock.AnythingOfType("func(internal.Context, common.WorkflowUpdateParams) error"),
		dvCom.WorkflowUpdateParams{Req: req, Config: config}).Return(&MockWorkflowRun{}, nil)

	cbComp := NewTestNewCallbackComponent(config, mtc, mockGitServer, repo, dvstore)

	workflow, err := cbComp.TriggerDataviewUpdateWorkflow(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, workflow)
}
