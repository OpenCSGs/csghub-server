package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	sdkTemporal "go.temporal.io/sdk/temporal"

	"opencsg.com/csghub-server/api/workflow/activity"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component/reporter"
)

const (
	DeployWorkflowQueue = "deploy-task-queue"
)

var deployActivity *activity.DeployActivity

var deployTypeNames = map[int]string{
	0: "space",
	1: "model",
	2: "finetune",
	3: "serverless",
	4: "evaluation",
	5: "notebook",
}

var GetClientFunc = temporal.GetClient

// StartDeployWorker starts temporal worker for deploy
func StartDeployWorker(
	ctx context.Context,
	cfg *config.Config,
	temporalClient temporal.Client,
	lr reporter.LogCollector,
	ib imagebuilder.Builder,
	ir imagerunner.Runner,
	gs gitserver.GitServer,
	ds database.DeployTaskStore,
	ts database.AccessTokenStore,
	ss database.SpaceStore,
	ms database.ModelStore,
	rfs database.RuntimeFrameworksStore,
	urs database.UserResourcesStore,
	mds database.MetadataStore,
) error {
	w := temporalClient.NewWorker(DeployWorkflowQueue, worker.Options{
		MaxConcurrentActivityExecutionSize:      cfg.Temporal.MaxConcurrentActivityExecutionSize,
		MaxConcurrentWorkflowTaskExecutionSize:  cfg.Temporal.MaxConcurrentWorkflowTaskExecutionSize,
		MaxConcurrentLocalActivityExecutionSize: cfg.Temporal.MaxConcurrentLocalActivityExecutionSize,
	})
	dcfg := common.BuildDeployConfig(cfg)
	act := activity.NewDeployActivity(dcfg, lr, ib, ir, gs, ds, ts, ss, ms, rfs, urs, mds)

	w.RegisterActivity(act)
	w.RegisterWorkflow(DeployWorkflow)
	err := temporalClient.Start()
	if err != nil {
		return fmt.Errorf("failed to start worker:  %w", err)
	}
	return nil
}

func GetDeploymentWorkflowID(deploy *database.Deploy) string {
	typeName, ok := deployTypeNames[deploy.Type]
	if !ok {
		typeName = "unknown"
	}

	sanitizedGitPath := strings.ReplaceAll(deploy.GitPath, "/", "-")
	if len(sanitizedGitPath) > 20 {
		sanitizedGitPath = sanitizedGitPath[:20]
	}

	return fmt.Sprintf("deploy-%s-%s-%d", typeName, sanitizedGitPath, deploy.ID)
}

func getDeploymentWorkflowID(deploy *database.Deploy) string {
	return GetDeploymentWorkflowID(deploy)
}

const (
	MaxRetryAttempts      = 3
	RetryInitialInterval  = 200 * time.Millisecond
	RetryMaximumInterval  = 2 * time.Second
	CancelTimeoutDuration = 10 * time.Second
)

// StartNewDeployTaskWithCancelOld
func StartNewDeployTaskWithCancelOld(buildTask, runTask *database.DeployTask) error {
	if runTask == nil {
		return fmt.Errorf("failed to start deployment: run task is nil")
	}

	temporalClient := GetClientFunc()
	workflowID := getDeploymentWorkflowID(runTask.Deploy)
	ctx := context.Background()

	err := retry.Do(
		func() error {
			running, err := cancelRunningWorkflow(ctx, temporalClient, workflowID)
			if err != nil {
				return fmt.Errorf("failed to handle running workflow: %w", err)
			}

			if running {
				if err = waitForWorkflowTermination(ctx, temporalClient, workflowID, CancelTimeoutDuration); err != nil {
					slog.Warn("workflow may not have fully terminated before starting new one",
						"workflow_id", workflowID, "error", err)
				}
			}

			var buildID int64
			if buildTask != nil {
				buildID = buildTask.ID
			}
			var runID int64
			if runTask != nil {
				runID = runTask.ID
			}

			_, err = temporalClient.ExecuteWorkflow(
				ctx,
				client.StartWorkflowOptions{
					ID:                    workflowID,
					TaskQueue:             DeployWorkflowQueue,
					WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_TERMINATE_IF_RUNNING,
				},
				DeployWorkflow,
				buildID,
				runID,
			)

			if err != nil {
				return fmt.Errorf("failed to start new workflow: %w", err)
			}

			return nil
		},
		retry.Attempts(3),
		retry.Delay(200*time.Millisecond),
		retry.MaxDelay(2*time.Second),
		retry.Context(ctx),
		retry.RetryIf(func(err error) bool {
			return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
		}),
		retry.OnRetry(func(n uint, err error) {
			slog.Warn("Operation failed, retrying",
				"attempt", n,
				"max_attempts", 3,
				"error", err,
			)
		}),
	)

	if err != nil {
		return fmt.Errorf("failed to manage workflow lifecycle: %w", err)
	}

	return nil
}

func CancelRunningWorkflow(ctx context.Context, temporalClient client.Client, workflowID string) (bool, error) {
	describeResp, err := temporalClient.DescribeWorkflowExecution(ctx, workflowID, "")

	if err != nil {
		if IsWorkflowNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to describe workflow: %w", err)
	}

	if describeResp.WorkflowExecutionInfo.Status == enums.WORKFLOW_EXECUTION_STATUS_RUNNING {
		if err := temporalClient.CancelWorkflow(ctx, workflowID, describeResp.WorkflowExecutionInfo.Execution.RunId); err != nil {
			return false, fmt.Errorf("failed to cancel existing workflow: %w", err)
		}
		time.Sleep(2 * time.Second)
		return true, nil
	}

	return false, nil
}

func cancelRunningWorkflow(ctx context.Context, temporalClient client.Client, workflowID string) (bool, error) {
	return CancelRunningWorkflow(ctx, temporalClient, workflowID)
}

func WaitForWorkflowTermination(ctx context.Context, temporalClient client.Client, workflowID string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pollInterval := 200 * time.Millisecond
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timeout waiting for workflow termination: %w", waitCtx.Err())
		case <-ticker.C:
			describeResp, err := temporalClient.DescribeWorkflowExecution(ctx, workflowID, "")
			if err != nil {
				if IsWorkflowNotFoundError(err) {
					return nil
				}
				return fmt.Errorf("failed to describe workflow: %w", err)
			}

			if describeResp.WorkflowExecutionInfo.Status != enums.WORKFLOW_EXECUTION_STATUS_RUNNING {
				return nil
			}
		}
	}
}

func waitForWorkflowTermination(ctx context.Context, temporalClient client.Client, workflowID string, timeout time.Duration) error {
	return WaitForWorkflowTermination(ctx, temporalClient, workflowID, timeout)
}

func IsWorkflowNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "workflow execution not found") ||
		strings.Contains(err.Error(), "workflow not found")
}

func DeployWorkflow(ctx workflow.Context, buildTaskId, runTaskId int64) ([]string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("deploy workflow started")

	result := []string{"deploy workflow started"}
	retryPolicy := &sdkTemporal.RetryPolicy{
		MaximumAttempts:    3,
		InitialInterval:    RetryInitialInterval,
		BackoffCoefficient: 2.0,
		MaximumInterval:    RetryMaximumInterval,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 1,
		HeartbeatTimeout:    time.Second * 30,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)

	// Step 1: build
	if buildTaskId != 0 {
		result = append(result, "ExecuteActivity build")
		err := workflow.ExecuteActivity(actCtx, deployActivity.Build, buildTaskId).Get(ctx, nil)
		if err != nil {
			logger.Error("failed to build", "error", err)
			result = append(result, "ExecuteActivity build failed")
			return result, err
		}

		result = append(result, "ExecuteActivity build succeeded")
	} else {
		result = append(result, "ExecuteActivity build skipped")
	}

	// Step 2: run
	if runTaskId != 0 {
		result = append(result, "ExecuteActivity run")
		err := workflow.ExecuteActivity(actCtx, deployActivity.Deploy, runTaskId).Get(ctx, nil)
		if err != nil {
			logger.Error("failed to run", "error", err)
			result = append(result, "ExecuteActivity run failed")
			return result, err
		}

		result = append(result, "ExecuteActivity run succeeded")
	}

	logger.Info("deploy workflow completed")
	result = append(result, "deploy workflow completed")
	return result, nil
}
