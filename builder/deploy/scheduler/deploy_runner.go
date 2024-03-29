package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/store/database"
)

// DeployRunner defines a k8s image running task
type DeployRunner struct {
	space *database.Space
	task  *database.DeployTask
	ir    imagerunner.Runner
	store *database.DeployTaskStore
}

func NewDeployRunner(ir imagerunner.Runner, s *database.Space, t *database.DeployTask) Runner {
	return &DeployRunner{
		space: s,
		task:  t,
		ir:    ir,
		store: database.NewDeployTaskStore(),
	}
}

// Run call k8s image runner service to run a docker image
func (t *DeployRunner) Run(ctx context.Context) error {
	slog.Info("run image deploy task", slog.Int64("deplopy_task_id", t.task.ID))

	// keep checking deploy status
	for {
		if t.task.Status == deployPending {
			req := t.makeDeployRequest()
			if req.ImageID == "" {
				time.Sleep(5 * time.Second)
				continue
			}
			_, err := t.ir.Run(ctx, req)
			if err != nil {
				// TODO:return retryable error
				return fmt.Errorf("call image runner failed: %w", err)
			}

			t.deployInProgress()
		}

		fields := strings.Split(t.space.Repository.Path, "/")
		req := &imagerunner.StatusRequest{
			SpaceID:   t.space.ID,
			OrgName:   fields[0],
			SpaceName: fields[1],
		}
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		resp, err := t.ir.Status(timeoutCtx, req)
		cancel()
		if err != nil {
			// return -1, fmt.Errorf("failed to call builder status api,%w", err)
			slog.Error("failed to call runner status api", slog.Any("error", err), slog.Any("task", t.task))
			// wait before next check
			time.Sleep(10 * time.Second)
			continue
		}

		if resp.DeployID > t.task.DeployID {
			t.deployFailed(fmt.Sprintf("cancel by new deploy:%d", resp.DeployID))
			return nil
		}
		switch resp.Code {
		case common.Deploying:
			t.deployInProgress()
			// wait before next check
			time.Sleep(10 * time.Second)
		case common.DeployFailed:
			slog.Info("image deploy failed", slog.String("space_name", t.space.Repository.Name), slog.Any("deplopy_task_id", t.task.ID))
			t.deployFailed(resp.Message)

			return fmt.Errorf("deploy failed, resp msg:%s", resp.Message)
		case common.Startup:
			slog.Info("image deploy success", slog.String("space_name", t.space.Repository.Name), slog.Any("deplopy_task_id", t.task.ID))
			t.deploySuccess()
			// wait before next check
			time.Sleep(10 * time.Second)

		case common.Running:
			slog.Info("image running", slog.String("space_name", t.space.Repository.Name), slog.Any("deplopy_task_id", t.task.ID))
			t.running()

			return nil
		case common.RunTimeError:
			slog.Error("image runtime erro", slog.String("space_name", t.space.Repository.Name), slog.Any("deplopy_task_id", t.task.ID))
			t.runtimeError(resp.Message)

			return fmt.Errorf("runtime error, resp msg:%s", resp.Message)
		default:
			slog.Error("unknown image status", slog.String("space_name", t.space.Repository.Name), slog.Any("deplopy_task_id", t.task.ID),
				slog.Int("status", resp.Code))
			return fmt.Errorf("unknown image status, resp msg:%s", resp.Message)
		}
	}
}
func (t *DeployRunner) WatchID() int64 { return t.task.ID }

func (t *DeployRunner) deployInProgress() {
	t.task.Status = deploying
	t.task.Message = "deploy in progress"
	// change to buidling status
	t.task.Deploy.Status = common.Deploying
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.store.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `Deploying`", "error", err)
	}
}

func (t *DeployRunner) deploySuccess() {
	t.task.Status = deployStartUp
	t.task.Message = "deploy succeeded, wati for startup"
	// change to buidling status
	t.task.Deploy.Status = common.Startup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.store.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `Startup`", "error", err)
	}
}

func (t *DeployRunner) deployFailed(msg string) {
	t.task.Status = deployFailed
	t.task.Message = msg
	// change to buidling status
	t.task.Deploy.Status = common.DeployFailed
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.store.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `DeployFailed`", "error", err)
	}
}

func (t *DeployRunner) running() {
	t.task.Status = deployRunning
	t.task.Message = "running"
	// change to buidling status
	t.task.Deploy.Status = common.Running
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.store.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `Running`", "error", err)
	}
}

func (t *DeployRunner) runtimeError(msg string) {
	t.task.Status = deployRunTimeError
	t.task.Message = msg
	// change to buidling status
	t.task.Deploy.Status = common.RunTimeError
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.store.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `RunTimeError`", "error", err)
	}
}

func (t *DeployRunner) makeDeployRequest() *imagerunner.RunRequest {
	fields := strings.Split(t.space.Repository.Path, "/")
	deploy, _ := t.store.GetDeploy(context.Background(), t.task.DeployID)
	return &imagerunner.RunRequest{
		SpaceID:   t.space.ID,
		OrgName:   fields[0],
		SpaceName: fields[1],
		UserName:  t.space.Repository.User.Name,
		Hardware:  t.parseHardware(t.space.Hardware),
		Env:       t.space.Env,
		GitRef:    t.space.Repository.DefaultBranch,
		ImageID:   deploy.ImageID,
		DeployID:  deploy.ID,
	}
}

func (t *DeployRunner) parseHardware(intput string) string {
	if strings.Contains(intput, "GPU") || strings.Contains(intput, "NVIDIA") {
		return "gpu"
	}

	return "cpu"
}
