package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/store/database"
)

// BuilderRunner defines a docker image building task
type BuilderRunner struct {
	space *database.Space
	task  *database.DeployTask
	ib    imagebuilder.Builder
	store *database.DeployTaskStore
}

func NewBuidRunner(b imagebuilder.Builder, s *database.Space, t *database.DeployTask) Runner {
	return &BuilderRunner{
		space: s,
		task:  t,
		ib:    b,
		store: database.NewDeployTaskStore(),
	}
}

func (t *BuilderRunner) makeBuildRequest() *imagebuilder.BuildRequest {
	fields := strings.Split(t.space.Repository.Path, "/")
	return &imagebuilder.BuildRequest{
		OrgName:   fields[0],
		SpaceName: fields[1],
		UserName:  t.space.Repository.User.Name,
		Hardware:  t.space.Hardware,
		// PythonVersion:  t.space.Template,
		SDKType:    t.space.Sdk,
		SDKVersion: t.space.SdkVersion,
		GitRef:     t.space.Repository.DefaultBranch,
		// TODO:load info for user
		// GitUserID:      t.space,
		// GitAccessToken: "",
		BuildID: t.task.DeployID,
	}
}

// Run call image builder service to build a docker image
func (t *BuilderRunner) Run(ctx context.Context) error {
	slog.Info("run image build task", slog.Int64("deplopy_task_id", t.task.ID))

	if t.task.Status == buildPending {
		resp, err := t.ib.Build(context.Background(), t.makeBuildRequest())
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
			BuildID:   t.task.DeployID,
		}
		resp, err := t.ib.Status(context.Background(), req)
		if err != nil {
			// return -1, fmt.Errorf("failed to call builder status api,%w", err)
			slog.Error("failed to call builder status api", slog.Any("error", err), slog.Any("task", t))
			// wait before next check
			time.Sleep(10 * time.Second)
			continue
		}
		switch resp.Code {
		case buildInProgress:
			// wait before next check
			time.Sleep(10 * time.Second)
			continue
		case buildSucceed:
			slog.Info("image build succeeded", slog.String("space_name", t.space.Repository.Name), slog.Any("deplopy_task_id", t.task.ID))
			t.buildSuccess(*resp)

			return nil
		case buildFailed:
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
	if err := t.store.UpdateInTx(ctx, t.task.Deploy, t.task); err != nil {
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
	if err := t.store.UpdateInTx(ctx, t.task.Deploy, t.task); err != nil {
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
	if err := t.store.UpdateInTx(ctx, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `BuildFailed`", "error", err)
	}
}

func (t *BuilderRunner) WatchID() int64 { return t.task.ID }
