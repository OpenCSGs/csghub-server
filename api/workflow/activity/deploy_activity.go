package activity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/log"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	utilcommon "opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component/reporter"
)

const (
	TaskTypeBuild  = 0
	TaskTypeDeploy = 1
)

const (
	DeployStatusPending      = common.TaskStatusDeployPending
	DeployStatusDeploying    = common.TaskStatusDeploying
	DeployStatusFailed       = common.TaskStatusDeployFailed
	DeployStatusStartUp      = common.TaskStatusDeployStartUp
	DeployStatusRunning      = common.TaskStatusDeployRunning
	DeployStatusRunTimeError = common.TaskStatusDeployRunTimeError
)

type DeployActivity struct {
	cfg common.DeployConfig
	lr  reporter.LogCollector
	ib  imagebuilder.Builder
	ir  imagerunner.Runner
	gs  gitserver.GitServer

	ds  database.DeployTaskStore
	ts  database.AccessTokenStore
	ss  database.SpaceStore
	ms  database.ModelStore
	rfs database.RuntimeFrameworksStore
	urs database.UserResourcesStore
	mds database.MetadataStore
}

func NewDeployActivity(
	cfg common.DeployConfig,
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
) *DeployActivity {
	return &DeployActivity{
		cfg: cfg,
		lr:  lr,
		ib:  ib,
		ir:  ir,
		gs:  gs,
		ds:  ds,
		ts:  ts,
		ss:  ss,
		ms:  ms,
		rfs: rfs,
		urs: urs,
		mds: mds,
	}
}

func (a *DeployActivity) Deploy(ctx context.Context, taskId int64) error {
	task, err := a.ds.GetDeployTask(ctx, taskId)
	if err != nil {
		return fmt.Errorf("failed to get deploy task: %w", err)
	}
	a.reportLog(types.DeployInProgress.String(), types.StepDeploying, task)

	repoInfo, err := a.getRepositoryInfo(ctx, task)
	if err != nil {
		if herr := a.handleRepoInfoError(ctx, task, err); herr != nil {
			return herr
		}

		return fmt.Errorf("deploy failed to get repository info: %w", err)
	}

	deployRequest, err := a.createDeployRequest(ctx, task, repoInfo)
	if err != nil {
		return fmt.Errorf("failed to create deploy request: %w", err)
	}

	if deployRequest.ImageID == "" {
		return fmt.Errorf("failed to deploy: image id is empty")
	}

	if deployRequest.OrderDetailID != 0 {
		if err = a.updateUserResourceDeployID(ctx, deployRequest); err != nil {
			return err
		}
	}

	updatedTask, err := a.ds.GetDeployTask(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("failed to get deploy task: %w", err)
	}

	if updatedTask.Status == common.Pending {
		runResponse, err := a.ir.Run(ctx, deployRequest)
		if err != nil {
			a.reportLog(types.DeployFailed.String()+": \n"+err.Error(), types.StepDeploying, task)
			if herr := a.handleDeployError(task, err); herr != nil {
				return herr
			}
			return fmt.Errorf("failed to call image runner: %w", err)
		}
		serviceName := runResponse.Message
		if err := a.updateDeployTaskStatus(task, serviceName); err != nil {
			return err
		}
	}

	return nil
}

func (a *DeployActivity) Build(ctx context.Context, taskId int64) error {
	task, err := a.ds.GetDeployTask(ctx, taskId)
	if err != nil {
		return fmt.Errorf("failed to get deploy task: %w", err)
	}
	if task.Status == common.TaskStatusBuildSkip {
		return nil
	}
	repoInfo, err := a.getRepositoryInfo(ctx, task)
	if err != nil {
		if herr := a.handleRepoInfoError(ctx, task, err); herr != nil {
			return herr
		}

		return fmt.Errorf("failed to get repository info: %w", err)
	}

	buildRequest, err := a.createBuildRequest(ctx, task, repoInfo)
	if err != nil {
		return fmt.Errorf("failed to create build request: %w", err)
	}

	return a.pollBuildStatus(ctx, task, repoInfo, buildRequest)
}

func (a *DeployActivity) getLogger(ctx context.Context) log.Logger {
	if ctx.Value("test") == "test" {
		return slog.Default()
	}
	return activity.GetLogger(ctx)
}

// pollBuildStatus
func (a *DeployActivity) pollBuildStatus(ctx context.Context, task *database.DeployTask, repoInfo common.RepoInfo, buildRequest *types.ImageBuilderRequest) error {
	continueLoop, err := a.checkBuildStatus(ctx, task, buildRequest)
	if err != nil {
		return err
	}
	if !continueLoop {
		return nil
	}

	heartbeatTicker := time.NewTicker(1 * time.Second)
	defer heartbeatTicker.Stop()

	statusCheckTicker := time.NewTicker(5 * time.Second)
	defer statusCheckTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.getLogger(ctx).Info("Build activity cancelled before core logic", "task_id", task.ID)
			go a.stopBuild(task, repoInfo)
			return a.handleBuildCancelled(task)

		case <-heartbeatTicker.C:
			activity.RecordHeartbeat(ctx, task.ID)
			if ctx.Err() != nil {
				a.getLogger(ctx).Info("Build activity cancelled during heartbeat", "task_id", task.ID)
				return a.handleBuildError(task, fmt.Errorf("build activity cancelled: %w", ctx.Err()))
			}
		case <-statusCheckTicker.C:
			continueLoop, err := a.checkBuildStatus(ctx, task, buildRequest)
			if err != nil {
				return err
			}
			if !continueLoop {
				return nil
			}
		}
	}
}

func (a *DeployActivity) checkBuildStatus(ctx context.Context, task *database.DeployTask, buildRequest *types.ImageBuilderRequest) (bool, error) {
	updatedTask, err := a.ds.GetDeployTask(ctx, task.ID)
	if err != nil {
		return false, fmt.Errorf("failed to get deploy task: %w", err)
	}

	switch {
	case updatedTask.Status == common.TaskStatusBuildPending:
		if err := a.ib.Build(ctx, buildRequest); err != nil {
			if herr := a.handleBuildError(task, err); herr != nil {
				a.getLogger(ctx).Error("Build failed", "task_id", task.ID, "error", err)
				return false, herr
			}

			a.getLogger(ctx).Error("Build failed", "task_id", task.ID, "error", err)
			a.reportLog(types.BuildFailed.String()+": \n"+err.Error(), types.StepBuildFailed, task)
			return false, fmt.Errorf("build failed: %w", err)
		}
		if err := a.handleBuildTaskToBuildInQueue(task); err != nil {
			a.getLogger(ctx).Error("Failed to handle build task to build in queue", "task_id", task.ID, "error", err)
			return false, err
		}
		a.reportLog(types.BuildInProgress.String(), types.StepBuildInProgress, task)
		return true, nil
	case updatedTask.Status == common.TaskStatusBuildFailed:
		a.getLogger(ctx).Info("Build task failed", "task_id", task.ID, "status", updatedTask.Status)
		return false, fmt.Errorf("build task failed: %s", updatedTask.Message)
	case updatedTask.Status == common.TaskStatusBuildSucceed:
		a.getLogger(ctx).Info("Build task succeed", "task_id", task.ID, "status", updatedTask.Status)
		return false, nil
	case a.isTaskTimedOut(updatedTask):
		a.reportLog("build task timeout", types.StepBuildFailed, task)
		return false, a.handleBuildTaskTimeout(updatedTask)
	default:
		return true, nil
	}
}

// IsTaskTimedOut
func (a *DeployActivity) isTaskTimedOut(task *database.DeployTask) bool {
	var timeoutMinutes int

	if task.TaskType == 0 {
		timeoutMinutes = a.cfg.BuildTimeoutInMin // build task
	} else {
		timeoutMinutes = a.cfg.SpaceDeployTimeoutInMin // deploy task
		if task.Deploy.SpaceID == 0 && task.Deploy.ModelID > 0 {
			timeoutMinutes = a.cfg.ModelDeployTimeoutInMin
		}
	}

	timeoutDuration := time.Duration(timeoutMinutes) * time.Minute
	deadline := task.CreatedAt.Add(timeoutDuration)
	return time.Now().After(deadline)
}

// getRepositoryInfo
func (a *DeployActivity) getRepositoryInfo(ctx context.Context, task *database.DeployTask) (common.RepoInfo, error) {
	var repoInfo common.RepoInfo

	if task.Deploy.SpaceID > 0 {
		space, err := a.ss.ByID(ctx, task.Deploy.SpaceID)
		if err != nil {
			return repoInfo, fmt.Errorf("failed to get space by ID: %w", err)
		}
		return a.createSpaceRepoInfo(space, task.Deploy.ID), nil
	}

	if task.Deploy.ModelID > 0 {
		model, err := a.ms.ByID(ctx, task.Deploy.ModelID)
		if err != nil {
			return repoInfo, fmt.Errorf("failed to get model by ID: %w", err)
		}
		return a.createModelRepoInfo(model, task.Deploy.ID), nil
	}

	repoInfo.Path = "/"
	return repoInfo, nil
}

// createSpaceRepoInfo
func (a *DeployActivity) createSpaceRepoInfo(space *database.Space, deployID int64) common.RepoInfo {
	cloneInfo := utilcommon.BuildCloneInfoByDomain(a.cfg.PublicDomain, a.cfg.SSHDomain, space.Repository)

	return common.RepoInfo{
		Path:          space.Repository.Path,
		Name:          space.Repository.Name,
		Sdk:           space.Sdk,
		SdkVersion:    space.SdkVersion,
		DriverVersion: space.DriverVersion,
		HTTPCloneURL:  cloneInfo.HTTPCloneURL,
		SpaceID:       space.ID,
		RepoID:        space.Repository.ID,
		UserName:      space.Repository.User.Username,
		DeployID:      deployID,
		ModelID:       0,
		RepoType:      string(types.SpaceRepo),
	}
}

// createModelRepoInfo
func (a *DeployActivity) createModelRepoInfo(model *database.Model, deployID int64) common.RepoInfo {
	return common.RepoInfo{
		Path:     model.Repository.Path,
		Name:     model.Repository.Name,
		ModelID:  model.ID,
		RepoID:   model.Repository.ID,
		UserName: model.Repository.User.Username,
		DeployID: deployID,
		SpaceID:  0,
		RepoType: string(types.ModelRepo),
	}
}

func (a *DeployActivity) handleRepoInfoError(ctx context.Context, task *database.DeployTask, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return a.handleRepositoryNotFound(task)
	}
	return fmt.Errorf("failed to get repository info: %w", err)
}

func (a *DeployActivity) updateDeployTaskStatus(task *database.DeployTask, serviceName string) error {
	task.Status = DeployStatusDeploying
	task.Message = "deploy in progress"
	task.Deploy.Status = common.Deploying

	if len(serviceName) > 0 {
		task.Deploy.SvcName = serviceName
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(5))
	defer cancel()
	if err := a.ds.UpdateInTx(ctx, []string{"status", "svc_name"}, []string{"status", "message"}, task.Deploy, task); err != nil {
		return err
	}
	return nil
}

func (a *DeployActivity) updateTaskStatus(task *database.DeployTask) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(5))
	defer cancel()

	if err := a.ds.UpdateInTx(ctx, []string{"status"}, []string{"status", "message"}, task.Deploy, task); err != nil {
		return err
	}

	return nil
}

// handleRepositoryNotFound
func (a *DeployActivity) handleRepositoryNotFound(task *database.DeployTask) error {
	task.Status = common.TaskStatusBuildFailed
	task.Message = "repository not found, please check the repository path"
	task.Deploy.Status = common.BuildFailed
	if err := a.updateTaskStatus(task); err != nil {
		return fmt.Errorf("handleRepositoryNotFound failed to update deploy task status: %w", err)
	}
	return nil
}

func (a *DeployActivity) handleBuildCancelled(task *database.DeployTask) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(5))
	defer cancel()
	task.Status = common.TaskStatusCancelled
	task.Message = "Cancelled"
	if err := a.ds.UpdateDeployTask(ctx, task); err != nil {
		return fmt.Errorf("handleBuildCancelled failed to update deploy task status: %w", err)
	}

	return nil
}

func (a *DeployActivity) handleBuildTaskTimeout(task *database.DeployTask) error {
	task.Status = common.TaskStatusBuildFailed
	task.Message = "build task timeout"
	task.Deploy.Status = common.BuildFailed

	if err := a.updateTaskStatus(task); err != nil {
		return fmt.Errorf("handleBuildTaskTimeout failed to update deploy task status: %w", err)
	}

	return nil
}

// handleBuildError
func (a *DeployActivity) handleBuildError(task *database.DeployTask, err error) error {
	task.Status = common.TaskStatusBuildFailed
	task.Message = fmt.Sprintf("build task failed: %s", err.Error())
	task.Deploy.Status = common.BuildFailed

	if err := a.updateTaskStatus(task); err != nil {
		return fmt.Errorf("handleBuildError failed to update deploy task status: %w", err)
	}
	return nil
}

// updateTaskStatusToBuildInQueue
func (a *DeployActivity) handleBuildTaskToBuildInQueue(task *database.DeployTask) error {
	task.Status = common.TaskStatusBuildInQueue
	task.Message = "build in queue"
	task.Deploy.Status = common.BuildInQueue

	if err := a.updateTaskStatus(task); err != nil {
		return fmt.Errorf("handleBuildTaskToBuildInQueue failed to update deploy task status: %w", err)
	}

	return nil
}

// handleImageRunnerError
func (a *DeployActivity) handleDeployError(task *database.DeployTask, err error) error {
	task.Status = DeployStatusFailed
	task.Message = err.Error()
	task.Deploy.Status = common.DeployFailed

	if err := a.updateTaskStatus(task); err != nil {
		return fmt.Errorf("handleDeployError failed to update deploy task status: %w", err)
	}
	return nil
}

func (a *DeployActivity) reportLog(message string, step types.Step, task *database.DeployTask) {
	stage := types.StageBuild
	logkey := types.LogLabelImageBuilder
	if task.TaskType == TaskTypeDeploy {
		stage = types.StageDeploy
		logkey = types.LogLabelDeploy
	}
	logEntry := types.LogEntry{
		Message:  message,
		Stage:    stage,
		Step:     step,
		DeployID: strconv.FormatInt(task.DeployID, 10),
		Labels: map[string]string{
			types.LogLabelTypeKey: logkey,
		},
	}

	if task.Deploy != nil {
		logEntry.Labels[types.StreamKeyDeployType] = strconv.Itoa(task.Deploy.Type)
		logEntry.Labels[types.StreamKeyDeployTypeID] = strconv.FormatInt(task.DeployID, 10)
		logEntry.Labels[types.StreamKeyDeployTaskID] = strconv.FormatInt(task.ID, 10)
	}

	a.lr.Report(logEntry)
}

// createBuildRequest
func (a *DeployActivity) createBuildRequest(ctx context.Context, task *database.DeployTask, repoInfo common.RepoInfo) (*types.ImageBuilderRequest, error) {
	accessToken, err := a.ts.FindByUID(ctx, task.Deploy.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get git access token: %w", err)
	}

	pathParts := strings.Split(repoInfo.Path, "/")
	if len(pathParts) < 2 {
		return nil, fmt.Errorf("invalid repository path format: %s", repoInfo.Path)
	}

	lastCommitReq := gitserver.GetRepoLastCommitReq{
		RepoType:  types.RepositoryType(repoInfo.RepoType),
		Namespace: pathParts[0],
		Name:      pathParts[1],
		Ref:       task.Deploy.GitBranch,
	}
	lastCommit, err := a.gs.GetRepoLastCommit(ctx, lastCommitReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository last commit: %w", err)
	}

	return &types.ImageBuilderRequest{
		OrgName:        pathParts[0],
		SpaceName:      pathParts[1],
		Hardware:       a.parseHardware(task.Deploy.Hardware),
		PythonVersion:  "3.10",
		Sdk:            repoInfo.Sdk,
		DriverVersion:  repoInfo.DriverVersion,
		Sdk_version:    repoInfo.SdkVersion,
		SpaceURL:       repoInfo.HTTPCloneURL,
		GitRef:         task.Deploy.GitBranch,
		UserId:         accessToken.User.Username,
		GitAccessToken: accessToken.Token,
		DeployId:       strconv.FormatInt(task.DeployID, 10),
		FactoryBuild:   false,
		ClusterID:      task.Deploy.ClusterID,
		LastCommitID:   lastCommit.ID,
		TaskId:         task.ID,
		RepoId:         repoInfo.RepoID,
	}, nil
}

// createDeployRequest
func (a *DeployActivity) createDeployRequest(ctx context.Context, task *database.DeployTask, repoInfo common.RepoInfo) (*types.RunRequest, error) {
	logger := a.getLogger(ctx)

	accessToken, err := a.ts.FindByUID(ctx, task.Deploy.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get git access token: %w", err)
	}

	pathParts := strings.Split(repoInfo.Path, "/")
	deployInfo, err := a.ds.GetDeployByID(ctx, task.DeployID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deploy with error: %w", err)
	}

	var engineArgsTemplates []types.EngineArg
	var toolCallParsers map[string]string
	if len(deployInfo.RuntimeFramework) > 0 {
		framework, err := a.rfs.FindByImageID(ctx, deployInfo.ImageID)
		if err != nil {
			return nil, fmt.Errorf("failed to get runtime framework by name %s: %w", deployInfo.RuntimeFramework, err)
		}
		trimmedEngineArgs := strings.TrimSpace(framework.EngineArgs)
		if len(trimmedEngineArgs) > 0 {
			if err := json.Unmarshal([]byte(trimmedEngineArgs), &engineArgsTemplates); err != nil {
				return nil, fmt.Errorf("failed to unmarshal engine args: %w", err)
			}
		}
		trimmedToolCallParsers := strings.TrimSpace(framework.ToolCallParsers)
		if len(trimmedToolCallParsers) > 0 {
			if err := json.Unmarshal([]byte(trimmedToolCallParsers), &toolCallParsers); err != nil {
				logger.Error("Failed to unmarshal tool call parsers", "error", err)
			}
		}
	}

	annotationMap, err := utilcommon.JsonStrToMap(deployInfo.Annotation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse deploy annotation: %w", err)
	}
	annotationMap[types.ResDeployID] = fmt.Sprintf("%v", deployInfo.ID)

	var hardware types.HardWare
	if err := json.Unmarshal([]byte(deployInfo.Hardware), &hardware); err != nil {
		logger.Error("Deploy hardware is invalid format", "hardware", deployInfo.Hardware, "task_id", task.ID)
		return nil, fmt.Errorf("failed to parse deploy hardware: %w", err)
	}

	envMap, err := a.makeDeployEnv(ctx, hardware, accessToken, deployInfo, engineArgsTemplates, toolCallParsers, repoInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to make deploy env: %w", err)
	}
	targetID := deployInfo.SpaceID

	if deployInfo.SpaceID == 0 && deployInfo.ModelID > 0 {
		targetID = deployInfo.ID
	}

	return &types.RunRequest{
		ID:            targetID,
		OrgName:       pathParts[0],
		RepoName:      pathParts[1],
		RepoType:      repoInfo.RepoType,
		UserName:      repoInfo.UserName,
		Annotation:    annotationMap,
		Hardware:      hardware,
		Env:           envMap,
		GitPath:       deployInfo.GitPath,
		GitRef:        deployInfo.GitBranch,
		ImageID:       deployInfo.ImageID,
		DeployID:      deployInfo.ID,
		MinReplica:    deployInfo.MinReplica,
		MaxReplica:    deployInfo.MaxReplica,
		Accesstoken:   accessToken.Token,
		ClusterID:     deployInfo.ClusterID,
		SvcName:       deployInfo.SvcName,
		DeployType:    deployInfo.Type,
		UserID:        deployInfo.UserUUID,
		Sku:           deployInfo.SKU,
		OrderDetailID: deployInfo.OrderDetailID,
		TaskId:        task.ID,
	}, nil
}

func (a *DeployActivity) parseHardware(input string) string {
	if strings.Contains(input, "GPU") || strings.Contains(input, "NVIDIA") {
		return "gpu"
	}
	return "cpu"
}

func (a *DeployActivity) stopBuild(buildTask *database.DeployTask, repoInfo common.RepoInfo) {
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	paths := strings.Split(repoInfo.Path, "/")
	err := a.ib.Stop(stopCtx, types.ImageBuildStopReq{
		OrgName:   paths[0],
		SpaceName: repoInfo.Name,
		DeployId:  fmt.Sprintf("%d", buildTask.DeployID),
		TaskId:    fmt.Sprintf("%d", buildTask.ID),
		ClusterID: buildTask.Deploy.ClusterID,
	})
	if err != nil {
		slog.Error("Failed to stop build", slog.Any("error", err))
		// Ignore the error of stopping the build, as it may be because the build has already been completed or does not exist
	}
}

// makeDeployEnv
func (a *DeployActivity) makeDeployEnv(ctx context.Context, hardware types.HardWare, accessToken *database.AccessToken, deployInfo *database.Deploy, engineArgsTemplates []types.EngineArg, toolCallParsers map[string]string, repoInfo common.RepoInfo) (map[string]string, error) {
	logger := a.getLogger(ctx)

	envMap, err := utilcommon.JsonStrToMap(deployInfo.Env)
	if err != nil {
		logger.Error("Deploy env is invalid json data", "deploy", deployInfo, "error", err)
		envMap = make(map[string]string)
	}

	varMap, err := utilcommon.JsonStrToMap(deployInfo.Variables)
	if err != nil {
		logger.Error("Deploy variables is invalid json data", "deploy", deployInfo, "error", err)
	} else {
		for key, value := range varMap {
			envMap[key] = value
		}
	}

	pathParts := strings.Split(repoInfo.Path, "/")
	commit, err := a.gs.GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: pathParts[0],
		Name:      pathParts[1],
		Ref:       deployInfo.GitBranch,
		RepoType:  types.RepositoryType(repoInfo.RepoType),
	})

	if err != nil {
		return nil, err
	}

	commitID, err := utilcommon.ShortenCommitID7(commit.ID)
	if err != nil {
		return nil, errorx.ErrInvalidCommitID
	}
	envMap["S3_INTERNAL"] = fmt.Sprintf("%v", a.cfg.S3Internal)
	envMap["HTTPCloneURL"] = a.getHttpCloneURLWithToken(repoInfo.HTTPCloneURL, accessToken.User.Username, accessToken.Token)
	envMap["ACCESS_TOKEN"] = accessToken.Token
	envMap["REPO_ID"] = repoInfo.Path // "namespace/name"
	envMap["REVISION"] = commitID     // branch

	if len(engineArgsTemplates) > 0 {
		var engineArgs strings.Builder
		argValuesMap, err := utilcommon.JsonStrToMap(deployInfo.EngineArgs)
		if err != nil {
			logger.Error("Deploy engine args is invalid json data", "deploy", *deployInfo, "error", err)
		} else {
			for _, arg := range engineArgsTemplates {
				if value, ok := argValuesMap[arg.Name]; ok {
					if arg.Value != "" && value == arg.Value {
						continue
					}
					// handle boolean value
					if !strings.Contains(arg.Format, "%") {
						if value == "false" || value == "0" || value == "" {
							continue
						}
						engineArgs.WriteString(" ")
						engineArgs.WriteString(arg.Format)
					} else {
						engineArgs.WriteString(" ")
						engineArgs.WriteString(fmt.Sprintf(arg.Format, value))
					}
				}
			}
		}

		// Process tool-calling arguments
		engineArgsStr := engineArgs.String()
		if strings.Contains(engineArgsStr, "--enable-auto-tool-choice") && len(toolCallParsers) > 0 {
			modelArch := a.getModelArchitecture(ctx, deployInfo.RepoID)
			if modelArch != "" {
				if parser, ok := toolCallParsers[modelArch]; ok {
					engineArgsStr = strings.Replace(engineArgsStr, "--enable-auto-tool-choice", "--enable-auto-tool-choice --tool-call-parser "+parser, 1)
					logger.Info("Added tool-call-parser", "architecture", modelArch, "parser", parser)
				} else {
					logger.Warn("No tool-call-parser found for architecture, using default openai parser", "architecture", modelArch)
					engineArgsStr = strings.Replace(engineArgsStr, "--enable-auto-tool-choice", "--enable-auto-tool-choice --tool-call-parser openai", 1)
				}
			} else {
				logger.Warn("Model architecture not found, removing --enable-auto-tool-choice")
				engineArgsStr = strings.ReplaceAll(engineArgsStr, "--enable-auto-tool-choice", "")
			}
		}

		logger.Debug("makeDeployEnv", "ENGINE_ARGS", engineArgsStr)
		envMap["ENGINE_ARGS"] = engineArgsStr
	}

	common.UpdateEvaluationEnvHardware(envMap, hardware)

	if deployInfo.SpaceID > 0 {
		// SDK port for space
		switch repoInfo.Sdk {
		case types.GRADIO.Name:
			envMap["port"] = strconv.Itoa(types.GRADIO.Port)
		case types.STREAMLIT.Name:
			envMap["port"] = strconv.Itoa(types.STREAMLIT.Port)
		case types.NGINX.Name:
			envMap["port"] = strconv.Itoa(types.NGINX.Port)
		case types.DOCKER.Name:
			envMap["port"] = strconv.Itoa(deployInfo.ContainerPort)
			envMap["HF_ENDPOINT"] = a.cfg.ModelDownloadEndpoint
		case types.MCPSERVER.Name:
			envMap["port"] = strconv.Itoa(types.MCPSERVER.Port)
		default:
			envMap["port"] = strconv.Itoa(types.DefaultContainerPort)
		}
	}

	if deployInfo.Type == types.InferenceType || deployInfo.Type == types.ServerlessType {
		// Runtime framework port for model
		envMap["port"] = strconv.Itoa(deployInfo.ContainerPort)
		envMap["HF_ENDPOINT"] = a.cfg.ModelDownloadEndpoint // "https://hub.opencsg-stg.com/"
		envMap["HF_HUB_OFFLINE"] = "1"
		envMap["HF_TASK"] = string(deployInfo.Task)
	}

	if deployInfo.Type == types.FinetuneType {
		envMap["port"] = strconv.Itoa(deployInfo.ContainerPort)
		envMap["HF_ENDPOINT"], _ = url.JoinPath(a.cfg.ModelDownloadEndpoint, "csg")
		envMap["HF_TOKEN"] = accessToken.Token
		envMap["USE_CSGHUB_MODEL"] = "1"
		envMap["USE_CSGHUB_DATASET"] = "1"
		envMap["JUPYTER_ENABLE_LAB"] = "yes"
		envMap["TERM"] = "xterm-256color"
	}

	if deployInfo.Type == types.NotebookType {
		envMap["port"] = strconv.Itoa(deployInfo.ContainerPort)
	}

	if a.cfg.PublicRootDomain == "" {
		if deployInfo.Type == types.FinetuneType {
			envMap["CONTEXT_PATH"] = "/endpoint/" + deployInfo.SvcName
		}
		if deployInfo.Type == types.SpaceType {
			envMap["GRADIO_ROOT_PATH"] = "/endpoint/" + deployInfo.SvcName
			envMap["STREAMLIT_SERVER_BASE_URL_PATH"] = "/endpoint/" + deployInfo.SvcName
		}
	}

	return envMap, nil
}

// getModelArchitecture reads the model architecture from metadata
func (a *DeployActivity) getModelArchitecture(ctx context.Context, repoID int64) string {
	logger := a.getLogger(ctx)

	// Get metadata from database
	metadata, err := a.mds.FindByRepoID(ctx, repoID)
	if err != nil {
		logger.Warn("Failed to get metadata from database", "error", err, "repo_id", repoID)
		return ""
	}

	if metadata.Architecture != "" {
		return metadata.Architecture
	}

	logger.Warn("Model architecture not found in metadata", "repo_id", repoID)
	return ""
}

// getHttpCloneURLWithToken
func (a *DeployActivity) getHttpCloneURLWithToken(httpCloneURL, username, token string) string {
	protocolIndex := strings.Index(httpCloneURL, "://")
	if protocolIndex > -1 {
		return fmt.Sprintf("%s%s:%s@%s", httpCloneURL[0:protocolIndex+3], username, token, httpCloneURL[protocolIndex+3:])
	}
	return httpCloneURL
}

// updateUserResourceDeployID
func (a *DeployActivity) updateUserResourceDeployID(ctx context.Context, req *types.RunRequest) error {
	userResource, err := a.urs.FindUserResourcesByOrderDetailId(ctx, req.UserID, req.OrderDetailID)
	if err != nil {
		return fmt.Errorf("failed to find user resource by order detail id: %w", err)
	}

	userResource.DeployId = req.DeployID
	if err := a.urs.UpdateDeployId(ctx, userResource); err != nil {
		return fmt.Errorf("failed to update deploy id for user resource: %w", err)
	}
	return nil
}
