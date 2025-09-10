package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/component/reporter"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// BuilderRunner defines a docker image building task
type BuilderRunner struct {
	repo        *RepoInfo
	task        *database.DeployTask
	ib          imagebuilder.Builder
	deployStore database.DeployTaskStore
	tokenStore  database.AccessTokenStore
	logReporter reporter.LogCollector
	git         gitserver.GitServer
	timeout     int
}

func NewBuildRunner(gc gitserver.GitServer, b imagebuilder.Builder, r *RepoInfo, t *database.DeployTask, logReporter reporter.LogCollector, timeout int) (Runner, error) {
	return &BuilderRunner{
		repo:        r,
		task:        t,
		ib:          b,
		deployStore: database.NewDeployTaskStore(),
		tokenStore:  database.NewAccessTokenStore(),
		git:         gc,
		logReporter: logReporter,
		timeout:     timeout,
	}, nil
}

func (t *BuilderRunner) makeBuildRequest() (*types.ImageBuilderRequest, error) {
	token, err := t.tokenStore.FindByUID(context.Background(), t.task.Deploy.UserID)
	if err != nil {
		return nil, fmt.Errorf("cant get git access token:%w", err)
	}
	fields := strings.Split(t.repo.Path, "/")
	sdkVer := ""
	if t.repo.SdkVersion == "" {
		slog.Debug("Use SDK default version", slog.Any("repository path", t.repo.Path))
		if t.repo.Sdk == types.GRADIO.Name {
			sdkVer = types.GRADIO.Version
		} else if t.repo.Sdk == types.STREAMLIT.Name {
			sdkVer = types.STREAMLIT.Version
		}
	} else {
		sdkVer = t.repo.SdkVersion
	}

	commit, err := t.git.GetRepoLastCommit(context.Background(), gitserver.GetRepoLastCommitReq{
		RepoType:  types.RepositoryType(t.repo.RepoType),
		Namespace: fields[0],
		Name:      fields[1],
		Ref:       t.task.Deploy.GitBranch,
	})

	if err != nil {
		return nil, fmt.Errorf("get repo last commit failed: %w", err)
	}

	return &types.ImageBuilderRequest{
		OrgName:   fields[0],
		SpaceName: fields[1],
		Hardware:  t.parseHardware(t.task.Deploy.Hardware),
		// PythonVersion:  t.space.PythonVersion,
		PythonVersion: "3.10",
		// SDKType:       "gradio",
		// SDKVersion:    "3.37.0",
		Sdk:            t.repo.Sdk,
		DriverVersion:  t.repo.DriverVersion,
		Sdk_version:    sdkVer,
		SpaceURL:       t.repo.HTTPCloneURL,
		GitRef:         t.task.Deploy.GitBranch,
		UserId:         token.User.Username,
		GitAccessToken: token.Token,
		DeployId:       strconv.FormatInt(t.task.DeployID, 10),
		FactoryBuild:   false,
		ClusterID:      t.task.Deploy.ClusterID,
		LastCommitID:   commit.ID,
		TaskId:         t.task.ID,
	}, nil
}

func (t *BuilderRunner) parseHardware(intput string) string {
	if strings.Contains(intput, "GPU") || strings.Contains(intput, "NVIDIA") {
		return "gpu"
	}

	return "cpu"
}

// Run call image builder service to build a docker image
func (t *BuilderRunner) Run(ctx context.Context) error {
	slog.Info("run image build task", slog.Int64("deplopy_task_id", t.task.ID))
	t.reporterLog(types.BuildInProgress.String(), types.StepBuildInProgress)

	if t.task.Status == BuildPending {
		req, err := t.makeBuildRequest()
		if err != nil {
			return fmt.Errorf("make build request failed: %w", err)
		}
		slog.Debug("make build request", slog.Any("req", req))
		err = t.ib.Build(ctx, req)
		if err != nil {
			// TODO:return retryable error
			return fmt.Errorf("call image builder failed: %w", err)
		}

		t.buildInQueue()
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			task, err := t.deployStore.GetDeployTask(ctx, t.task.ID)
			if err != nil {
				return fmt.Errorf("get deploy task failed: %w", err)
			}

			if task.Status == BuildFailed || task.Status == BuildSucceed {
				return nil
			}

			if task.CreatedAt.Add(time.Duration(t.timeout) * time.Second * 10).Before(time.Now()) {
				t.buildFailed("build task timeout")
			}
		}
	}
}

func (t *BuilderRunner) buildInQueue() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deploy, err := t.deployStore.GetDeployByID(ctx, t.task.DeployID)
	if err != nil {
		slog.Error("failed to get deploy info when build in queue", "deploy_id", t.task.DeployID, "error", err)
		return
	}
	if deploy.Status == common.Building {
		//"deploy status is already building, skip setting build in progress status"
		return
	}
	t.task.Status = BuildInQueue
	t.task.Message = "build in queue"
	// change to building status
	t.task.Deploy.Status = common.BuildInQueue
	if err := t.deployStore.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `Building`", "error", err)
	}
}

func (t *BuilderRunner) buildFailed(msg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deploy, err := t.deployStore.GetDeployByID(ctx, t.task.DeployID)
	if err != nil {
		slog.Error("failed to get deploy info when build failed", "error", err)
		return
	}
	if deploy.Status != common.Building {
		slog.Warn("deploy status is not building, skip setting build failed status")
		return
	}

	t.task.Status = BuildFailed
	t.task.Message = msg
	// change to building status
	t.task.Deploy.Status = common.BuildFailed
	if err := t.deployStore.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `BuildFailed`", "error", err)
	}
	t.reporterLog(msg, types.StepBuildFailed)
}

func (t *BuilderRunner) WatchID() int64 { return t.task.ID }

func (t *BuilderRunner) reporterLog(msg string, step types.Step) {
	logEntry := types.LogEntry{
		Message:  msg,
		Stage:    types.StageBuild,
		Step:     step,
		DeployID: strconv.FormatInt(t.task.DeployID, 10),
		Labels: map[string]string{
			types.LogLabelTypeKey: types.LogLabelImageBuilder,
		},
	}
	if nil != t.task.Deploy {
		logEntry.Labels[types.StreamKeyDeployType] = strconv.Itoa(t.task.Deploy.Type)
		logEntry.Labels[types.StreamKeyDeployTypeID] = strconv.FormatInt(t.task.ID, 10)
		logEntry.Labels[types.StreamKeyDeployTaskID] = strconv.FormatInt(t.task.ID, 10)
	}
	t.logReporter.Report(logEntry)
}
