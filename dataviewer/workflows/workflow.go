package workflows

import (
	"fmt"
	"log/slog"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	dvCom "opencsg.com/csghub-server/dataviewer/common"
)

func BuildDataViewerRunWorker(tc temporal.Client, cfg *config.Config, act DataViewerActivity) error {
	workerOptions := worker.Options{
		MaxConcurrentActivityExecutionSize:      cfg.DataViewer.MaxConcurrentActivityExecutionSize,
		MaxConcurrentLocalActivityExecutionSize: cfg.DataViewer.MaxConcurrentLocalActivityExecutionSize,
		MaxConcurrentWorkflowTaskExecutionSize:  cfg.DataViewer.MaxConcurrentWorkflowTaskExecutionSize,
		EnableSessionWorker:                     true,
		MaxConcurrentSessionExecutionSize:       cfg.DataViewer.MaxConcurrentSessionExecutionSize,
	}
	slog.Info("run viewer worker", slog.Any("options", cfg.DataViewer))
	wr := tc.NewWorker(dvCom.TaskQueueDataViewerDatasetUpdate, workerOptions)

	wr.RegisterActivity(act.BeginViewerJob)
	wr.RegisterActivity(act.GetCardFromReadme)
	wr.RegisterActivity(act.ScanRepoFiles)
	wr.RegisterActivity(act.DetermineCardData)
	wr.RegisterActivity(act.CheckIfNeedRebuild)
	wr.RegisterActivity(act.CreateParquetBranch)
	wr.RegisterActivity(act.CopyParquetFiles)
	wr.RegisterActivity(act.DownloadSplitFiles)
	wr.RegisterActivity(act.ConvertToParquetFiles)
	wr.RegisterActivity(act.UploadParquetFiles)
	wr.RegisterActivity(act.UpdateCardData)
	wr.RegisterActivity(act.CleanUp)
	wr.RegisterActivity(act.UpdateWorkflowStatus)

	wr.RegisterWorkflow(DataViewerUpdateWorkflow)

	err := tc.Start()
	if err != nil {
		return fmt.Errorf("failed to start worker:  %w", err)
	}
	return nil
}

func DataViewerUpdateWorkflow(ctx workflow.Context, updateReq dvCom.WorkflowUpdateParams) error {
	info := workflow.GetInfo(ctx)
	workflowID := info.WorkflowExecution.ID
	runID := info.WorkflowExecution.RunID
	slog.Debug("start workflow", slog.Any("workflowID", workflowID), slog.Any("runID", runID), slog.Any("req", updateReq.Req))

	retryPolicy := &sdktemporal.RetryPolicy{
		InitialInterval: 2 * time.Minute,
		MaximumAttempts: updateReq.Config.DataViewer.ActivityMaximumAttempts,
	}

	workflowActivityTimeout := time.Duration(updateReq.Config.DataViewer.ActivityStartToCloseTimeout) * time.Second
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: workflowActivityTimeout,
		RetryPolicy:         retryPolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	sessionExecTimeout := time.Duration(updateReq.Config.DataViewer.SessionExecutionTimeout) * time.Minute
	so := &workflow.SessionOptions{
		CreationTimeout:  5 * time.Minute,
		ExecutionTimeout: sessionExecTimeout,
	}

	sessionCtx, err := workflow.CreateSession(ctx, so)
	if err != nil {
		return fmt.Errorf("create workflow session error: %w", err)
	}
	defer workflow.CompleteSession(sessionCtx)

	shouldUpdateViewer, err := runWorkFlow(sessionCtx, updateReq)
	updateWorkflowStatus(sessionCtx, updateReq, err, shouldUpdateViewer)
	if err != nil {
		return fmt.Errorf("run data viewer workflow error: %w", err)
	}
	return nil
}

func runWorkFlow(sessionCtx workflow.Context, updateWorkflow dvCom.WorkflowUpdateParams) (bool, error) {
	var err error

	err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.BeginViewerJob).Get(sessionCtx, nil)
	if err != nil {
		return false, fmt.Errorf("run data viewer activity BeginViewerJob error: %w", err)
	}

	var definedCardData dvCom.CardData
	err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.GetCardFromReadme, updateWorkflow.Req).Get(sessionCtx, &definedCardData)
	if err != nil {
		return false, fmt.Errorf("run data viewer activity GetCardFromReadme error: %w", err)
	}

	var repoFileClass dvCom.RepoFilesClass
	err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.ScanRepoFiles,
		dvCom.ScanRepoFileReq{
			Req:         updateWorkflow.Req,
			MaxFileSize: updateWorkflow.Config.DataViewer.MaxFileSize},
	).Get(sessionCtx, &repoFileClass)
	if err != nil {
		return false, fmt.Errorf("run data viewer activity GetCardFromReadme error: %w", err)
	}

	var repoDataType dvCom.RepoDataType

	if len(repoFileClass.ParquetFiles) > 0 {
		repoDataType = RepoParquetData
	} else if len(repoFileClass.JsonlFiles) > 0 {
		repoDataType = RepoJsonData
	} else if len(repoFileClass.CsvFiles) > 0 {
		repoDataType = RepoCsvData
	}

	var computedCardData dvCom.CardData
	err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.DetermineCardData, dvCom.DetermineCardReq{
		Card:         definedCardData,
		Class:        repoFileClass,
		RepoDataType: repoDataType,
	}).Get(sessionCtx, &computedCardData)
	if err != nil {
		return false, fmt.Errorf("run data viewer activity DetermineCardData error: %w", err)
	}

	var needRebuild bool
	err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.CheckIfNeedRebuild, dvCom.CheckBuildReq{
		Req:  updateWorkflow.Req,
		Card: computedCardData,
	}).Get(sessionCtx, &needRebuild)
	if err != nil {
		return false, fmt.Errorf("run data viewer activity CheckIfNeedRebuild error: %w", err)
	}

	if !needRebuild {
		return false, nil
	}

	var newBranch string
	err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.CreateParquetBranch, updateWorkflow.Req).Get(sessionCtx, &newBranch)
	if err != nil {
		return false, fmt.Errorf("run data viewer activity CreateParquetBranch error: %w", err)
	}
	if len(newBranch) < 1 {
		return false, fmt.Errorf("run data viewer activity CreateParquetBranch error and new branch is empty")
	}

	var finalCardData dvCom.CardData

	if repoDataType == RepoParquetData {
		err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.CopyParquetFiles, dvCom.CopyParquetReq{
			Req:              updateWorkflow.Req,
			ComputedCardData: computedCardData,
			NewBranch:        newBranch,
		}).Get(sessionCtx, &finalCardData)
		if err != nil {
			return false, fmt.Errorf("run data viewer activity CopyParquetFiles error: %w", err)
		}
	} else if repoDataType == RepoJsonData || repoDataType == RepoCsvData {
		var downloadCard dvCom.DownloadCard
		err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.DownloadSplitFiles,
			dvCom.DownloadFileReq{
				Req:  updateWorkflow.Req,
				Card: computedCardData,
			},
		).Get(sessionCtx, &downloadCard)
		if err != nil {
			return false, fmt.Errorf("run data viewer activity CopyParquetFiles error: %w", err)
		}

		err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.ConvertToParquetFiles,
			dvCom.ConvertReq{
				Req:          updateWorkflow.Req,
				DownloadCard: downloadCard,
				RepoDataType: repoDataType,
			},
		).Get(sessionCtx, nil)
		if err != nil {
			return false, fmt.Errorf("run data viewer activity ConvertToParquetFiles error: %w", err)
		}

		err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.UploadParquetFiles,
			dvCom.UploadParquetReq{
				Req:          updateWorkflow.Req,
				DownloadCard: downloadCard,
				NewBranch:    newBranch,
			},
		).Get(sessionCtx, &finalCardData)
		if err != nil {
			return false, fmt.Errorf("run data viewer activity UploadParquetFiles error: %w", err)
		}
	}

	err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.UpdateCardData,
		dvCom.UpdateCardReq{
			Req:            updateWorkflow.Req,
			OriginCardData: definedCardData,
			FinalCardData:  finalCardData,
		},
	).Get(sessionCtx, nil)
	if err != nil {
		return false, fmt.Errorf("run data viewer activity UpdateCardData error: %w", err)
	}

	err = workflow.ExecuteActivity(sessionCtx, DataViewerActivity.CleanUp, updateWorkflow.Req).Get(sessionCtx, nil)
	if err != nil {
		return false, fmt.Errorf("run data viewer activity CleanUp error: %w", err)
	}

	return true, nil
}

func updateWorkflowStatus(sessionCtx workflow.Context,
	updateWorkflow dvCom.WorkflowUpdateParams,
	wfErr error,
	shouldUpdateViewer bool,
) {
	errMsg := ""
	if wfErr != nil {
		errMsg = wfErr.Error()
	}
	err := workflow.ExecuteActivity(sessionCtx, DataViewerActivity.UpdateWorkflowStatus,
		dvCom.UpdateWorkflowStatusReq{
			Req:                updateWorkflow.Req,
			WorkflowErrMsg:     errMsg,
			ShouldUpdateViewer: shouldUpdateViewer,
		},
	).Get(sessionCtx, nil)
	if err != nil {
		slog.Error("run data viewer workflow update activity", slog.Any("error", err))
	}
}
