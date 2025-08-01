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
	hubcom "opencsg.com/csghub-server/common/utils/common"
)

// DeployRunner defines a k8s image running task
type DeployRunner struct {
	repo                   *RepoInfo
	task                   *database.DeployTask
	ir                     imagerunner.Runner
	store                  database.DeployTaskStore
	tokenStore             database.AccessTokenStore
	deployStartTime        time.Time
	deployCfg              common.DeployConfig
	runtimeFrameworksStore database.RuntimeFrameworksStore
}

func NewDeployRunner(ir imagerunner.Runner, r *RepoInfo, t *database.DeployTask, deployCfg common.DeployConfig) Runner {
	return &DeployRunner{
		repo:                   r,
		task:                   t,
		ir:                     ir,
		store:                  database.NewDeployTaskStore(),
		deployStartTime:        time.Now(),
		tokenStore:             database.NewAccessTokenStore(),
		deployCfg:              deployCfg,
		runtimeFrameworksStore: database.NewRuntimeFrameworksStore(),
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
		//wait svc to be created in k8s
		time.Sleep(10 * time.Second)

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
			isCancel, reason := t.shouldForceCancelDeploy(fields[0], fields[1], resp)
			if isCancel {
				return t.cancelDeploy(ctx, fields[0], fields[1], reason)
			}
			t.deployInProgress("")
			// waitting for check next time
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

func (t *DeployRunner) shouldForceCancelDeploy(orgName, repoName string, resp *types.StatusResponse) (bool, string) {
	duration := time.Since(t.deployStartTime).Minutes()
	limitTime := t.deployCfg.SpaceDeployTimeoutInMin
	if t.task.Deploy.SpaceID == 0 && t.task.Deploy.ModelID > 0 {
		limitTime = t.deployCfg.ModelDeployTimeoutInMin
	}
	if duration >= float64(limitTime) {
		// space or model deploy duration is greater than timeout defined in env (default is 30 mins)
		reason := "This Space/Model has been cancelled automatically by the system due to deployment timeout."
		slog.Warn(reason, slog.Any("duration", duration), slog.Any("timeout", limitTime), slog.Any("namespace", orgName), slog.Any("repoName", repoName))
		return true, reason
	}

	// Todo: check if pod is pending for too long due to not enough hardware resources
	// if t.task.Deploy.SpaceID > 0 && len(resp.Instances) > 0 && resp.Instances[0].Status == string(corev1.PodPending) {
	// 	reason := "The deployment has been cancelled because it took too long to acquire the necessary hardware resources."
	// 	slog.Warn(reason, slog.Any("namespace", orgName), slog.Any("repoName", repoName))
	// 	return true, reason
	// }

	return false, ""
}

func (t *DeployRunner) WatchID() int64 { return t.task.ID }

func (t *DeployRunner) deployInProgress(svcName string) {
	t.task.Status = deploying
	t.task.Message = "deploy in progress"
	// change to building status
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
	// change to building status
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
	// change to building status
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
	// change to building status
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
	// change to building status
	t.task.Deploy.Status = common.RunTimeError
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.store.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, t.task.Deploy, t.task); err != nil {
		slog.Error("failed to change deploy status to `RunTimeError`", "error", err)
	}
}

func (t *DeployRunner) makeDeployRequest() (*types.RunRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := t.tokenStore.FindByUID(ctx, t.task.Deploy.UserID)
	if err != nil {
		return nil, fmt.Errorf("cant get git access token:%w", err)
	}
	fields := strings.Split(t.repo.Path, "/")
	deploy, err := t.store.GetDeployByID(ctx, t.task.DeployID)
	if err != nil {
		return nil, fmt.Errorf("fail to get deploy with error: %w", err)
	}

	var engineArgsTemplate []types.EngineArg
	if len(deploy.RuntimeFramework) > 0 {
		frame, err := t.runtimeFrameworksStore.FindEnabledByName(ctx, deploy.RuntimeFramework)
		if err != nil {
			return nil, fmt.Errorf("get runtime framework by name %s error: %w", deploy.RuntimeFramework, err)
		}
		if len(strings.TrimSpace(frame.EngineArgs)) > 0 {
			err = json.Unmarshal([]byte(frame.EngineArgs), &engineArgsTemplate)
			if err != nil {
				return nil, fmt.Errorf("unmarshal engine args error: %w", err)
			}
		}
	}

	annoMap, err := hubcom.JsonStrToMap(deploy.Annotation)
	if err != nil {
		slog.Error("deploy annotation is invalid json data", slog.Any("Annotation", deploy.Annotation))
		return nil, err
	}
	annoMap[types.ResDeployID] = fmt.Sprintf("%v", deploy.ID)

	var hardware = types.HardWare{}
	err = json.Unmarshal([]byte(deploy.Hardware), &hardware)
	if err != nil {
		slog.Error("deploy hardware is invalid format", slog.Any("hardware", deploy.Hardware))
		return nil, err
	}

	envMap := t.makeDeployEnv(hardware, token, deploy, engineArgsTemplate)

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
		Sku:         deploy.SKU,
	}, nil
}

func (t *DeployRunner) makeDeployEnv(
	hardware types.HardWare,
	token *database.AccessToken,
	deploy *database.Deploy,
	engineArgsTemplate []types.EngineArg,
) map[string]string {
	envMap, err := hubcom.JsonStrToMap(deploy.Env)
	if err != nil {
		slog.Error("deploy env is invalid json data", slog.Any("deploy", deploy))
	}

	varMap, err := hubcom.JsonStrToMap(deploy.Variables)
	if err != nil {
		slog.Error("deploy variables is invalid json data", slog.Any("deploy", deploy))
	} else {
		for key, value := range varMap {
			envMap[key] = value
		}
	}

	// for space and models
	envMap["S3_INTERNAL"] = fmt.Sprintf("%v", t.deployCfg.S3Internal)
	envMap["HTTPCloneURL"] = t.getHttpCloneURLWithToken(t.repo.HTTPCloneURL, token.Token)
	envMap["ACCESS_TOKEN"] = token.Token
	envMap["REPO_ID"] = t.repo.Path       // "namespace/name"
	envMap["REVISION"] = deploy.GitBranch // branch
	if len(engineArgsTemplate) > 0 {
		ENGINE_ARGS := ""
		argValuesMap, err := hubcom.JsonStrToMap(deploy.EngineArgs)
		if err != nil {
			slog.Error("deploy engine args is invalid json data", slog.Any("deploy", *deploy), slog.Any("error", err))
		} else {
			for _, arg := range engineArgsTemplate {
				if value, ok := argValuesMap[arg.Name]; ok {
					ENGINE_ARGS += " " + fmt.Sprintf(arg.Format, value)
				}
			}
		}
		slog.Debug("makeDeployEnv", slog.Any("ENGINE_ARGS", ENGINE_ARGS))
		envMap["ENGINE_ARGS"] = ENGINE_ARGS
	}

	common.UpdateEvaluationEnvHardware(envMap, hardware)

	if deploy.SpaceID > 0 {
		// sdk port for space
		switch t.repo.Sdk {
		case types.GRADIO.Name:
			envMap["port"] = strconv.Itoa(types.GRADIO.Port)
		case types.STREAMLIT.Name:
			envMap["port"] = strconv.Itoa(types.STREAMLIT.Port)
		case types.NGINX.Name:
			envMap["port"] = strconv.Itoa(types.NGINX.Port)
		case types.DOCKER.Name:
			envMap["port"] = strconv.Itoa(deploy.ContainerPort)
			envMap["HF_ENDPOINT"] = t.deployCfg.ModelDownloadEndpoint
		case types.MCPSERVER.Name:
			envMap["port"] = strconv.Itoa(types.MCPSERVER.Port)
		default:
			envMap["port"] = strconv.Itoa(types.DefaultContainerPort)
		}
	}

	if deploy.Type == types.InferenceType || deploy.Type == types.ServerlessType {
		// runtime framework port for model
		envMap["port"] = strconv.Itoa(deploy.ContainerPort)
		envMap["HF_ENDPOINT"] = t.deployCfg.ModelDownloadEndpoint // "https://hub.opencsg-stg.com/"
		envMap["HF_HUB_OFFLINE"] = "1"
		envMap["HF_TASK"] = string(deploy.Task)
	}

	if deploy.Type == types.FinetuneType {
		envMap["port"] = strconv.Itoa(deploy.ContainerPort)
		envMap["HF_ENDPOINT"], _ = url.JoinPath(t.deployCfg.ModelDownloadEndpoint, "csg")
		envMap["HF_TOKEN"] = token.Token
		envMap["USE_CSGHUB_MODEL"] = "1"
		envMap["USE_CSGHUB_DATASET"] = "1"
		envMap["JUPYTER_ENABLE_LAB"] = "yes"
		envMap["TERM"] = "xterm-256color"
	}

	if t.deployCfg.PublicRootDomain == "" {
		if deploy.Type == types.FinetuneType {
			envMap["CONTEXT_PATH"] = "/endpoint/" + deploy.SvcName
		}
		if deploy.Type == types.SpaceType {
			envMap["GRADIO_ROOT_PATH"] = "/endpoint/" + deploy.SvcName
			envMap["STREAMLIT_SERVER_BASE_URL_PATH"] = "/endpoint/" + deploy.SvcName
		}
	}

	return envMap
}

func (t *DeployRunner) cancelDeploy(ctx context.Context, orgName, repoName, reason string) error {
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
	t.deployFailed(reason)
	return nil
}

func (t *DeployRunner) getHttpCloneURLWithToken(httpCloneUrl, token string) string {
	num := strings.Index(httpCloneUrl, "://")
	if num > -1 {
		return fmt.Sprintf("%s%s@%s", httpCloneUrl[0:num+3], token, httpCloneUrl[num+3:])
	}
	return httpCloneUrl
}
