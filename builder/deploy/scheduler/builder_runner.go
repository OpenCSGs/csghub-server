package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/store/database"
)

// BuilderRunner defines a docker image building task
type BuilderRunner struct {
	space       *database.Space
	task        *database.DeployTask
	ib          imagebuilder.Builder
	deployStore *database.DeployTaskStore
	tokenStore  *database.AccessTokenStore
}

func NewBuidRunner(b imagebuilder.Builder, s *database.Space, t *database.DeployTask) Runner {
	return &BuilderRunner{
		space:       s,
		task:        t,
		ib:          b,
		deployStore: database.NewDeployTaskStore(),
		tokenStore:  database.NewAccessTokenStore(),
	}
}

func (t *BuilderRunner) makeBuildRequest() (*imagebuilder.BuildRequest, error) {
	token, err := t.tokenStore.FindByUID(context.Background(), t.space.Repository.UserID)
	if err != nil {
		return nil, fmt.Errorf("cant get git access token:%w", err)
	}
	fields := strings.Split(t.space.Repository.Path, "/")
	return &imagebuilder.BuildRequest{
		OrgName:   fields[0],
		SpaceName: fields[1],
		// Hardware:  t.space.Hardware,
		Hardware: "cpu",
		// PythonVersion:  t.space.PythonVersion,
		PythonVersion: "3.10",
		SDKType:       "gradio",
		SDKVersion:    "3.37.0",
		// SDKType:        t.space.Sdk,
		// SDKVersion:     t.space.SdkVersion,
		SpaceGitURL:    t.space.Repository.HTTPCloneURL,
		GitRef:         t.space.Repository.DefaultBranch,
		GitUserID:      token.User.Username,
		GitAccessToken: token.Token,
		BuildID:        strconv.FormatInt(t.task.DeployID, 10),
		FactoryBuild:   false,
	}, nil
}

// Run call image builder service to build a docker image
func (t *BuilderRunner) Run(ctx context.Context) error {
	slog.Info("run image build task", slog.Int64("deplopy_task_id", t.task.ID))

	if t.task.Status == buildPending {
		req, err := t.makeBuildRequest()
		if err != nil {
			return fmt.Errorf("make build request failed: %w", err)
		}
		resp, err := t.ib.Build(context.Background(), req)
		if err != nil {
			// TODO:return retryable error
			return fmt.Errorf("call image builder failed: %w", err)
		}

		if resp.Code != 0 {
			// job failed
			return fmt.Errorf("image builder reported error,code:%d,msg:%s", resp.Code, resp.Message)
		}

		t.buildInProgress()
	}

	// keep checking build status
	for {
		fields := strings.Split(t.space.Repository.Path, "/")
		req := &imagebuilder.StatusRequest{
			OrgName:   fields[0],
			SpaceName: fields[1],
			BuildID:   strconv.FormatInt(t.task.DeployID, 10),
		}
		resp, err := t.ib.Status(context.Background(), req)
		slog.Debug("image builder called", slog.Any("resp", resp), slog.Any("error", err))
		if err != nil {
			// return -1, fmt.Errorf("failed to call builder status api,%w", err)
			slog.Error("failed to call builder status api", slog.Any("error", err), slog.Any("task", t))
			// wait before next check
			time.Sleep(10 * time.Second)
			continue
		}
		switch {
		case resp.Inprogress():
			// wait before next check
			time.Sleep(10 * time.Second)
			continue
		case resp.Success():
			slog.Info("image build succeeded", slog.String("space_name", t.space.Repository.Name), slog.Any("deplopy_task_id", t.task.ID))
			t.buildSuccess(*resp)

			return nil
		case resp.Fail():
			slog.Info("image build failed", slog.String("space_name", t.space.Repository.Name), slog.Any("deplopy_task_id", t.task.ID))
			t.buildFailed()

			return nil
		}
	}
}

func (t *BuilderRunner) buildInProgress() {
	t.task.Status = buildInProgress
	t.task.Message = "build in progress"
	// change to buidling status
	t.task.Deploy.Status = common.Building
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.deployStore.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `Building`", "error", err)
	}
}

func (t *BuilderRunner) buildSuccess(resp imagebuilder.StatusResponse) {
	t.task.Status = buildSucceed
	t.task.Message = "build succeeded"
	// change to buidling status
	t.task.Deploy.Status = common.BuildSuccess
	t.task.Deploy.ImageID = resp.ImageID
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.deployStore.UpdateInTx(ctx, []string{"status", "image_id"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `BuildSuccess`", "error", err)
	}
}

func (t *BuilderRunner) buildFailed() {
	t.task.Status = buildFailed
	t.task.Message = "build failed"
	// change to buidling status
	t.task.Deploy.Status = common.BuildFailed
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.deployStore.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `BuildFailed`", "error", err)
	}
}

func (t *BuilderRunner) WatchID() int64 { return t.task.ID }
