package workflows

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	mock_temporal "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	mock_activity "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/dataviewer/workflows"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	dvCom "opencsg.com/csghub-server/dataviewer/common"
)

type workflowTester struct {
	env   *testsuite.TestWorkflowEnvironment
	mocks struct {
		mockact *mock_activity.MockDataViewerActivity
	}
}

func newWorkflowTester(t *testing.T) (*workflowTester, error) {
	suite := testsuite.WorkflowTestSuite{}
	tester := &workflowTester{
		env: suite.NewTestWorkflowEnvironment(),
	}
	tester.env.SetWorkerOptions(worker.Options{
		EnableSessionWorker: true,
	})

	mockact := mock_activity.NewMockDataViewerActivity(t)
	tester.mocks.mockact = mockact

	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	mtc := mock_temporal.NewMockClient(t)
	mtc.EXPECT().NewWorker(dvCom.TaskQueueDataViewerDatasetUpdate, mock.Anything).Return(tester.env)
	mtc.EXPECT().Start().Return(nil)

	err = BuildDataViewerRunWorker(mtc, cfg, mockact)

	if err != nil {
		return nil, err
	}

	return tester, nil
}

func TestWorkflow_DataviewerWorkflow(t *testing.T) {
	t.Run("json/csv", func(t *testing.T) {
		tester, err := newWorkflowTester(t)
		require.NoError(t, err)

		cfg, err := config.LoadConfig()
		require.Nil(t, err)

		req := types.UpdateViewerReq{
			Namespace: "test",
			Name:      "test",
			Branch:    "test",
			RepoType:  types.DatasetRepo,
		}

		tester.mocks.mockact.EXPECT().BeginViewerJob(mock.Anything).Return(nil)
		tester.mocks.mockact.EXPECT().GetCardFromReadme(mock.Anything, req).Return(&dvCom.CardData{}, nil)
		tester.mocks.mockact.EXPECT().ScanRepoFiles(mock.Anything,
			dvCom.ScanRepoFileReq{
				Req:         req,
				MaxFileSize: cfg.DataViewer.MaxFileSize,
			}).Return(&dvCom.RepoFilesClass{
			JsonlFiles: map[string]*types.File{
				"test": {
					Path: "test",
					Size: 1024,
				},
			},
		}, nil)
		tester.mocks.mockact.EXPECT().DetermineCardData(mock.Anything,
			dvCom.DetermineCardReq{
				Card: dvCom.CardData{},
				Class: dvCom.RepoFilesClass{
					JsonlFiles: map[string]*types.File{
						"test": {
							Path: "test",
							Size: 1024,
						},
					},
				},
				RepoDataType: RepoJsonData,
			}).Return(&dvCom.CardData{}, nil)

		tester.mocks.mockact.EXPECT().CheckIfNeedRebuild(mock.Anything, dvCom.CheckBuildReq{
			Req:  req,
			Card: dvCom.CardData{},
		}).Return(true, nil)

		tester.mocks.mockact.EXPECT().CreateParquetBranch(mock.Anything, req).Return(dvCom.ParquetBranch, nil)

		tester.mocks.mockact.EXPECT().DownloadSplitFiles(mock.Anything,
			dvCom.DownloadFileReq{
				Req:  req,
				Card: dvCom.CardData{},
			},
		).Return(&dvCom.DownloadCard{}, nil)

		tester.mocks.mockact.EXPECT().ConvertToParquetFiles(mock.Anything, dvCom.ConvertReq{
			Req:          req,
			DownloadCard: dvCom.DownloadCard{},
			RepoDataType: RepoJsonData,
		}).Return(nil)

		tester.mocks.mockact.EXPECT().UploadParquetFiles(mock.Anything, dvCom.UploadParquetReq{
			Req:          req,
			DownloadCard: dvCom.DownloadCard{},
			NewBranch:    dvCom.ParquetBranch,
		}).Return(&dvCom.CardData{}, nil)

		tester.mocks.mockact.EXPECT().UpdateCardData(mock.Anything, dvCom.UpdateCardReq{
			Req:            req,
			OriginCardData: dvCom.CardData{},
			FinalCardData:  dvCom.CardData{},
		}).Return(nil)

		tester.mocks.mockact.EXPECT().CleanUp(mock.Anything, req).Return(nil)

		tester.mocks.mockact.EXPECT().UpdateWorkflowStatus(mock.Anything, dvCom.UpdateWorkflowStatusReq{
			Req:                req,
			WorkflowErr:        nil,
			ShouldUpdateViewer: true,
		}).Return(nil)

		tester.env.ExecuteWorkflow(DataViewerUpdateWorkflow, dvCom.WorkflowUpdateParams{
			Req:    req,
			Config: cfg,
		})

		require.True(t, tester.env.IsWorkflowCompleted())
		require.NoError(t, tester.env.GetWorkflowError())
	})

	t.Run("parquet", func(t *testing.T) {
		tester, err := newWorkflowTester(t)
		require.NoError(t, err)

		cfg, err := config.LoadConfig()
		require.Nil(t, err)

		req := types.UpdateViewerReq{
			Namespace: "test",
			Name:      "test",
			Branch:    "test",
			RepoType:  types.DatasetRepo,
		}

		tester.mocks.mockact.EXPECT().BeginViewerJob(mock.Anything).Return(nil)
		tester.mocks.mockact.EXPECT().GetCardFromReadme(mock.Anything, req).Return(&dvCom.CardData{}, nil)
		tester.mocks.mockact.EXPECT().ScanRepoFiles(mock.Anything,
			dvCom.ScanRepoFileReq{
				Req:         req,
				MaxFileSize: cfg.DataViewer.MaxFileSize,
			}).Return(&dvCom.RepoFilesClass{
			ParquetFiles: map[string]*types.File{
				"test": {
					Path: "test",
					Size: 1024,
				},
			},
		}, nil)
		tester.mocks.mockact.EXPECT().DetermineCardData(mock.Anything,
			dvCom.DetermineCardReq{
				Card: dvCom.CardData{},
				Class: dvCom.RepoFilesClass{
					ParquetFiles: map[string]*types.File{
						"test": {
							Path: "test",
							Size: 1024,
						},
					},
				},
				RepoDataType: RepoParquetData,
			}).Return(&dvCom.CardData{}, nil)

		tester.mocks.mockact.EXPECT().CheckIfNeedRebuild(mock.Anything, dvCom.CheckBuildReq{
			Req:  req,
			Card: dvCom.CardData{},
		}).Return(true, nil)

		tester.mocks.mockact.EXPECT().CreateParquetBranch(mock.Anything, req).Return(dvCom.ParquetBranch, nil)

		tester.mocks.mockact.EXPECT().CopyParquetFiles(mock.Anything, dvCom.CopyParquetReq{
			Req:              req,
			ComputedCardData: dvCom.CardData{},
			NewBranch:        dvCom.ParquetBranch,
		}).Return(&dvCom.CardData{}, nil)

		tester.mocks.mockact.EXPECT().UpdateCardData(mock.Anything, dvCom.UpdateCardReq{
			Req:            req,
			OriginCardData: dvCom.CardData{},
			FinalCardData:  dvCom.CardData{},
		}).Return(nil)

		tester.mocks.mockact.EXPECT().CleanUp(mock.Anything, req).Return(nil)

		tester.mocks.mockact.EXPECT().UpdateWorkflowStatus(mock.Anything, dvCom.UpdateWorkflowStatusReq{
			Req:                req,
			WorkflowErr:        nil,
			ShouldUpdateViewer: true,
		}).Return(nil)

		tester.env.ExecuteWorkflow(DataViewerUpdateWorkflow, dvCom.WorkflowUpdateParams{
			Req:    req,
			Config: cfg,
		})

		require.True(t, tester.env.IsWorkflowCompleted())
		require.NoError(t, tester.env.GetWorkflowError())
	})

}
