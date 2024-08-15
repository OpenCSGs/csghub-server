package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type DeployTimeout struct {
	deploySpaceTimeoutInMin int
	deployModelTimeoutInMin int
}

// DeployRunner defines a k8s image running task
type DeployRunner struct {
	repo                  *RepoInfo
	task                  *database.DeployTask
	ir                    imagerunner.Runner
	store                 *database.DeployTaskStore
	tokenStore            *database.AccessTokenStore
	deployStartTime       time.Time
	deployTimeout         *DeployTimeout
	modelDownloadEndpoint string
}

func NewDeployRunner(ir imagerunner.Runner, r *RepoInfo, t *database.DeployTask, dto *DeployTimeout, mdep string) Runner {
	return &DeployRunner{
		repo:                  r,
		task:                  t,
		ir:                    ir,
		store:                 database.NewDeployTaskStore(),
		deployStartTime:       time.Now(),
		deployTimeout:         dto,
		tokenStore:            database.NewAccessTokenStore(),
		modelDownloadEndpoint: mdep,
	}
}

// Run call k8s image runner service to run a docker image
func (t *DeployRunner) Run(ctx context.Context) error {
	slog.Info("run image deploy task", slog.Int64("deplopy_task_id", t.task.ID))

	// keep checking deploy status
	for {
		if t.task.Status == deployPending {
			req, err := t.makeDeployRequest()
			if err != nil {
				return fmt.Errorf("fail to make deploy request: %w", err)
			}
			if req.ImageID == "" {
				time.Sleep(5 * time.Second)
				continue
			}
			slog.Debug("After build deploy request", slog.Any("req", req))
			resp, err := t.ir.Run(ctx, req)
			if err != nil {
				// TODO:return retryable error
				return fmt.Errorf("call image runner failed: %w", err)
			}

			t.deployInProgress(resp.Message)
			// record time of create knative service
			t.deployStartTime = time.Now()
		}

		fields := strings.Split(t.repo.Path, "/")

		targetID := t.task.Deploy.SpaceID
		if t.task.Deploy.SpaceID == 0 {
			targetID = t.task.Deploy.ID // support model deploy with multi-instance
		}
		req := &types.StatusRequest{
			ID:          targetID,
			OrgName:     fields[0],
			RepoName:    fields[1],
			SvcName:     t.task.Deploy.SvcName,
			ClusterID:   t.task.Deploy.ClusterID,
			NeedDetails: true, // check status of both knative and its pods
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
			duration := time.Since(t.deployStartTime).Minutes()
			limitTime := t.deployTimeout.deploySpaceTimeoutInMin
			if t.task.Deploy.SpaceID == 0 && t.task.Deploy.ModelID > 0 {
				limitTime = t.deployTimeout.deployModelTimeoutInMin
			}
			if duration >= float64(limitTime) {
				// space or model deploy duration is greater than timeout defined in env (default is 30 mins)
				slog.Warn("Space or Model is going to be undeploy due to timeout of deploying", slog.Any("duration", duration), slog.Any("timeout", limitTime), slog.Any("namespace", fields[0]), slog.Any("repoName", fields[1]))
				return t.cancelDeploy(ctx, fields[0], fields[1])
			}
			t.deployInProgress("")
			// wait before next check
			time.Sleep(10 * time.Second)
		case common.DeployFailed:
			slog.Error("image deploy failed", slog.String("repo_name", t.repo.Name), slog.Any("deplopy_task_id", t.task.ID), slog.Any("resp", resp))
			t.deployFailed(resp.Message)

			return fmt.Errorf("deploy failed, resp msg:%s", resp.Message)
		case common.Startup:
			slog.Info("image deploy success", slog.String("repo_name", t.repo.Name), slog.Any("deplopy_task_id", t.task.ID))
			t.deploySuccess()
			// wait before next check
			time.Sleep(10 * time.Second)

		case common.Running:
			slog.Info("image running", slog.String("repo_name", t.repo.Name), slog.Any("deplopy_task_id", t.task.ID))
			t.running(resp.Endpoint)

			return nil
		case common.RunTimeError:
			slog.Error("image runtime erro", slog.String("repo_name", t.repo.Name), slog.Any("deplopy_task_id", t.task.ID))
			t.runtimeError(resp.Message)

			return fmt.Errorf("runtime error, resp msg:%s", resp.Message)
		default:
			slog.Error("unknown image status", slog.String("repo_name", t.repo.Name), slog.Any("deplopy_task_id", t.task.ID),
				slog.Int("status", resp.Code))
			return fmt.Errorf("unknown image status, resp msg:%s", resp.Message)
		}
	}
}

func (t *DeployRunner) WatchID() int64 { return t.task.ID }

func (t *DeployRunner) deployInProgress(svcName string) {
	t.task.Status = deploying
	t.task.Message = "deploy in progress"
	// change to buidling status
	t.task.Deploy.Status = common.Deploying
	if len(svcName) > 0 {
		t.task.Deploy.SvcName = svcName
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.store.UpdateInTx(ctx, []string{"status", "svc_name"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
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

func (t *DeployRunner) running(endpoint string) {
	t.task.Status = deployRunning
	t.task.Message = "running"
	// change to buidling status
	t.task.Deploy.Status = common.Running
	t.task.Deploy.Endpoint = endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.store.UpdateInTx(ctx, []string{"status", "endpoint"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
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

func (t *DeployRunner) makeDeployRequest() (*types.RunRequest, error) {
	token, err := t.tokenStore.FindByUID(context.Background(), t.task.Deploy.UserID)
	if err != nil {
		return nil, fmt.Errorf("cant get git access token:%w", err)
	}
	fields := strings.Split(t.repo.Path, "/")
	deploy, err := t.store.GetDeployByID(context.Background(), t.task.DeployID)
	if err != nil {
		return nil, fmt.Errorf("fail to get deploy with error :%w", err)
	}

	annoMap, err := common.JsonStrToMap(deploy.Annotation)
	if err != nil {
		slog.Error("deploy annotation is invalid json data", slog.Any("Annotation", deploy.Annotation))
		return nil, err
	}
	annoMap[types.ResDeployID] = fmt.Sprintf("%v", deploy.ID)

	envMap, err := common.JsonStrToMap(deploy.Env)
	if err != nil {
		slog.Error("deploy env is invalid json data", slog.Any("env", deploy.Env))
		return nil, err
	}

	var hardware = types.HardWare{}
	err = json.Unmarshal([]byte(deploy.Hardware), &hardware)
	if err != nil {
		slog.Error("deploy hardware is invalid format", slog.Any("hardware", deploy.Hardware))
		return nil, err
	}

	// for space and models
	envMap["HTTPCloneURL"] = t.getHttpCloneURLWithToken(t.repo.HTTPCloneURL, token.Token)
	envMap["ACCESS_TOKEN"] = token.Token
	envMap["REPO_ID"] = t.repo.Path       // "namespace/name"
	envMap["REVISION"] = deploy.GitBranch // branch
	if hardware.Gpu.Num != "" {
		envMap["GPU_NUM"] = hardware.Gpu.Num
	}

	if deploy.SpaceID > 0 {
		// sdk port for space
		if t.repo.Sdk == GRADIO.Name {
			envMap["port"] = GRADIO.Port
		} else if t.repo.Sdk == STREAMLIT.Name {
			envMap["port"] = STREAMLIT.Port
		} else if t.repo.Sdk == NGINX.Name {
			envMap["port"] = NGINX.Port
		} else {
			envMap["port"] = "8080"
		}
	}

	if deploy.Type == types.InferenceType || deploy.Type == types.ServerlessType {
		// runtime framework port for model
		envMap["port"] = strconv.Itoa(deploy.ContainerPort)
		envMap["HF_ENDPOINT"] = t.modelDownloadEndpoint // "https://hub-stg.opencsg.com/"
	}

	if deploy.Type == types.FinetuneType {
		envMap["port"] = strconv.Itoa(deploy.ContainerPort)
		envMap["HF_ENDPOINT"], _ = url.JoinPath(t.modelDownloadEndpoint, "hf")
		envMap["HF_TOKEN"] = token.Token
	}

	targetID := deploy.SpaceID
	// deployID is unique for space and model
	if deploy.SpaceID == 0 && deploy.ModelID > 0 {
		targetID = deploy.ID // support model deploy with multi-instance
	}

	return &types.RunRequest{
		ID:          targetID,
		OrgName:     fields[0],
		RepoName:    fields[1],
		RepoType:    t.repo.RepoType,
		UserName:    t.repo.UserName,
		Annotation:  annoMap,
		Hardware:    hardware,
		Env:         envMap,
		GitPath:     deploy.GitPath,
		GitRef:      deploy.GitBranch,
		ImageID:     deploy.ImageID,
		DeployID:    deploy.ID,
		MinReplica:  deploy.MinReplica,
		MaxReplica:  deploy.MaxReplica,
		Accesstoken: token.Token,
		ClusterID:   deploy.ClusterID,
		SvcName:     deploy.SvcName,
		DeployType:  deploy.Type,
		UserID:      deploy.UserUUID,
		CostPerHour: deploy.CostPerHour,
		Sku:         deploy.SKU,
	}, nil
}

func (t *DeployRunner) cancelDeploy(ctx context.Context, orgName, repoName string) error {
	targetID := t.task.Deploy.SpaceID
	if t.task.Deploy.SpaceID == 0 {
		// support model deploy with multi-instance
		targetID = t.task.Deploy.ID
	}
	stopReq := &types.StopRequest{
		ID:       targetID,
		OrgName:  orgName,
		RepoName: repoName,
		SvcName:  t.task.Deploy.SvcName,
	}
	_, err := t.ir.Stop(ctx, stopReq)
	if err != nil {
		return fmt.Errorf("fail to undeploy space/model with err: %v", err)
	}
	t.deployFailed("space/model deploy timeout")
	return nil
}

func (t *DeployRunner) getHttpCloneURLWithToken(httpCloneUrl, token string) string {
	num := strings.Index(httpCloneUrl, "://")
	if num > -1 {
		return fmt.Sprintf("%s%s@%s", httpCloneUrl[0:num+3], token, httpCloneUrl[num+3:])
	}
	return httpCloneUrl
}
