package types

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// Stage represents the major stage of a process, like image building or deployment.
// It helps group logs from the same high-level operation.
type Stage string

// Step represents a specific step within a Stage.
// It provides fine-grained context about the log entry.
type Step string

// LogLevel defines the severity of the log message.
type LogLevel string

const (
	// Log Levels
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelDebug LogLevel = "debug"

	// Stages
	StagePreBuild  Stage = "pre-build"
	StageBuild     Stage = "build"
	StageDeploy    Stage = "deploy"
	StageRunning   Stage = "running"
	StageCleanup   Stage = "cleanup"
	StageCancelled Stage = "cancelled"

	// Steps for pre-build
	StepWaitingForResource Step = "waiting_for_resource"
	StepInitializing       Step = "initializing"

	// Steps for build
	StepBuildPending    Step = "buildPending"
	StepBuildInProgress Step = "buildInProgress"
	StepBuildFailed     Step = "buildFailed"
	StepBuildSucceed    Step = "buildSucceed"

	// Steps for deploy
	StepDeployPending      Step = "deployPending"
	StepDeploying          Step = "deploying"
	StepDeployFailed       Step = "deployFailed"
	StepDeployStartUp      Step = "deployStartUp"
	StepDeployRunning      Step = "deployRunning"
	StepDeployRunTimeError Step = "deployRunTimeError"

	// StepPodCreated Steps for running
	StepPodCreated Step = "pod_created"

	// StepDeletingResources Steps for cleanup
	StepDeletingResources Step = "deleting_resources"

	// StepCancelled Steps for cancelled
	StepCancelled Step = "cancelled"

	// LogLabel
	LogLabelTypeKey      string = "csghub_log_label_type"
	LogLabelKeyClusterID string = "csghub_log_label_cluster_id"
	LogLabelImageBuilder string = "imagebuilder"
	LogLabelDeploy       string = "deploy"

	// stream key
	StreamKeyDeployID     = "csghub_deploy_id"
	StreamKeyDeployType   = "csghub_deploy_type"
	StreamKeyDeployTypeID = "csghub_deploy_type_id"
	StreamKeyDeployTaskID = "csghub_deploy_task_id"

	StreamKeyInstanceName   = "pod_name"
	StreamKeyDeployCommitID = "csghub_deploy_commit_id"
)

type ReportMsg string

// String implements fmt.Stringer interface
func (r ReportMsg) String() string {
	return string(r)
}

const (
	// PreBuilder
	PreBuildSubmit ReportMsg = "Deployment request received and added to the queue. It will start automatically when its turn arrives."

	// Deployment
	DeployInProgress ReportMsg = "Deploying the service. This includes starting containers and configuring networking—please wait."
	DeployStarting   ReportMsg = "Deployment completed; service is starting and will be reachable shortly."
	DeployFailed     ReportMsg = "Deployment failed. View deployment logs for details and retry; contact support if needed."
	DeployRunning    ReportMsg = "Service is running. Click to open the endpoint or view details."
	DeployError      ReportMsg = "Deployment error. Check deployment logs for details and retry."
	DeployCancelled  ReportMsg = "Deployment cancelled. Re-submit when ready."

	// Build
	BuildInProgress ReportMsg = "Building the container image. This can take a few minutes; a notification will be sent when it's ready."
	BuildSucceeded  ReportMsg = "Image build finished successfully. Preparing the service for deployment."
	BuildFailed     ReportMsg = "Build failed. View build logs for details and retry; contact support if the issue persists."

	// Image Build
	ImageUpdated ReportMsg = "Image build status updated. Check the image details for the latest information."
	ImageDeleted ReportMsg = "Image removed from the registry. If unexpected, check recent actions or contact support."

	// Knative service
	KsvcCreated ReportMsg = "Service created successfully and is initializing."
	KsvcUpdated ReportMsg = "Service status updated. Check the service page for details."
	KsvcDeleted ReportMsg = "Service removed. If unexpected, check activity logs or contact support."

	// Argo Workflow
	WorkflowUpdated ReportMsg = "Workflow status updated. View workflow details for step-by-step progress."
	WorkflowDeleted ReportMsg = "Workflow removed. If unexpected, check recent activity or logs."
)

// PodInfo represents the basic information about a Pod
type PodInfo struct {
	Namespace     string            `json:"-"`
	PodName       string            `json:"-"`
	PodUID        string            `json:"-"`
	ServiceName   string            `json:"-"`
	ContainerName string            `json:"-"`
	Stream        string            `json:"-"`
	Labels        map[string]string `json:"-"`
	Phase         corev1.PodPhase   `json:"-"`
}

// LogEntry defines the structured log format for Loki.
type LogEntry struct {
	TraceID   string       `json:"trace_id,omitempty"`
	Timestamp time.Time    `json:"-"`
	Level     LogLevel     `json:"level"`
	Message   string       `json:"msg"`
	Stage     Stage        `json:"stage,omitempty"`
	Step      Step         `json:"step,omitempty"`
	DeployID  string       `json:"cluster_id,omitempty"`
	Category  LogCategrory `json:"category,omitempty"`
	// Custom Scene fields for custom metadata.e.g. cluster_id, task_id, deploy_type, etc.
	Labels map[string]string `json:"labels,omitempty"`
	// Fields for k8s pod logs
	PodInfo *PodInfo `json:"pod_info,omitempty"`
}

type LogCategrory string

func (r LogCategrory) String() string {
	return string(r)
}

var (
	LogCategoryPlatform  LogCategrory = "platform"
	LogCategoryContainer LogCategrory = "container"
)

type ClientType string

func (r ClientType) String() string {
	return string(r)
}

var (
	ClientTypeRunner       ClientType = "runner"
	ClientTypeCSGHUB       ClientType = "csghub"
	ClientTypeLogCollector ClientType = "logcollector"
)

const (
	TimeLayout = "2006-01-02 15:04:05"
)
