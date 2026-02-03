package types

import (
	"io"
	"time"

	"k8s.io/client-go/kubernetes"
	knative "knative.dev/serving/pkg/client/clientset/versioned"
)

// todo  删除
const (
	StrategyTypeBlueGreen StrategyType = "blue_green"
	StrategyTypeCanary    StrategyType = "canary"
)

type StrategyType string

type KubeScheduler string

func (ks KubeScheduler) ToString() string {
	return string(ks)
}

const (
	KubeSchedulerVolcano KubeScheduler = "volcano"
)

type VolcanoVgpuModel string

const (
	VolcanoVgpuModelHamiCore VolcanoVgpuModel = "hami-core"
	VolcanoVgpuModelMig      VolcanoVgpuModel = "mig"
)

type (
	RunRequest struct {
		ID       int64  `json:"id"`
		UserName string `json:"user_name"`
		OrgName  string `json:"org_name"`
		RepoName string `json:"repo_name"`
		RepoType string `json:"repo_type"`

		GitPath string `json:"git_path"` // git repo path
		GitRef  string `json:"git_ref"`  // git repo branch

		MinReplica int `json:"min_replica"` // min replica of instance/pod
		MaxReplica int `json:"max_replica"` // max replica of instance/pod

		Hardware   HardWare          `json:"hardware,omitempty"`   // resource requirements
		Env        map[string]string `json:"env,omitempty"`        // runtime env variables
		Annotation map[string]string `json:"annotation,omitempty"` // resource annotations

		RuntimeFramework string     `json:"runtime_framework"` // runtime framework of image, TGI/vllm/Pipeline/Deepspeed/LLamacpp
		ImageID          string     `json:"image_id"`          // container_image
		DeployID         int64      `json:"deploy_id"`
		Accesstoken      string     `json:"access_token"`
		ClusterID        string     `json:"cluster_id"`
		SvcName          string     `json:"svc_name"`
		DeployType       int        `json:"deploy_type"`
		UserID           string     `json:"user_id"`
		Sku              string     `json:"sku"`
		OrderDetailID    int64      `json:"order_detail_id"`
		TaskId           int64      `json:"task_id"`
		Nodes            []Node     `json:"nodes"`
		Scheduler        *Scheduler `json:"scheduler,omitempty"`
	}

	Node struct {
		Name       string
		EnableVXPU bool
	}

	RunResponse struct {
		DeployID int64  `json:"deploy_id"`
		Code     int    `json:"code"`
		Message  string `json:"message"`
	}

	StopRequest struct {
		ID        int64  `json:"id"`
		OrgName   string `json:"org_name"`
		RepoName  string `json:"repo_name"`
		ClusterID string `json:"cluster_id"`
		SvcName   string `json:"svc_name"`
	}

	StopResponse struct {
		DeployID int64  `json:"deploy_id"`
		Code     int    `json:"code"`
		Message  string `json:"message"`
	}

	StatusRequest struct {
		ID          int64  `json:"id"`
		OrgName     string `json:"org_name"`
		RepoName    string `json:"repo_name"`
		ClusterID   string `json:"cluster_id"`
		SvcName     string `json:"svc_name"`
		NeedDetails bool   `json:"need_details"`
		DeployType  int    `json:"deploy_type"`
	}

	StatusResponse struct {
		DeployID       int64      `json:"deploy_id"`
		UserID         string     `json:"user_id"`
		Code           int        `json:"code"`
		Message        string     `json:"message"`
		Endpoint       string     `json:"url"`
		Instances      []Instance `json:"instance"`
		Replica        int        `json:"replica"`
		DeployType     int        `json:"deploy_type"`
		ServiceName    string     `json:"service_name"`
		DeploySku      string     `json:"deploy_sku"`
		OrderDetailID  int64      `json:"order_detail_id"`
		ActualReplica  int        `json:"actual_replica"`
		DesiredReplica int        `json:"desired_replica"`
		Reason         string     `json:"reason"`

		Revisions []Revision `json:"revision"`
	}

	Revision struct {
		RevisionName   string `json:"revision_name,omitempty"`
		CommitID       string `json:"commit_id,omitempty"`
		TrafficPercent int    `json:"traffic_percent,omitempty"`
		DeployType     string `json:"deploy_type,omitempty"`
	}

	LogsRequest struct {
		ID        int64  `json:"id"`
		OrgName   string `json:"org_name"`
		RepoName  string `json:"repo_name"`
		DeployID  int64  `json:"deploy_id"`
		ClusterID string `json:"cluster_id"`
		SvcName   string `json:"svc_name"`
	}

	LogsResponse struct {
		SSEReadCloser io.ReadCloser `json:"sse_read_closer"`
	}

	CheckRequest struct {
		ID        int64  `json:"id"`
		OrgName   string `json:"org_name"`
		RepoName  string `json:"repo_name"`
		ClusterID string `json:"cluster_id"`
		SvcName   string `json:"svc_name"`
	}

	CluserRequest struct {
		ClusterID     string `json:"cluster_id"`
		ClusterConfig string `json:"cluster_config"`
		Region        string `json:"region"`
		Zone          string `json:"zone"`     //cn-beijing
		Provider      string `json:"provider"` //ali
		Enable        bool   `json:"enable"`
		StorageClass  string `json:"storage_class"`
	}

	ReplicaResponse struct {
		DeployID       int64      `json:"deploy_id"`
		Code           int        `json:"code"`
		Message        string     `json:"message"`
		ActualReplica  int        `json:"actual_replica"`
		DesiredReplica int        `json:"desired_replica"`
		Instances      []Instance `json:"instance"`
	}

	ServiceRequest struct {
		ServiceName string `json:"-"`
		ClusterID   string `json:"cluster_id"`
	}

	ServiceInfoResponse struct {
		ServiceName string   `json:"service_name"`
		PodNames    []string `json:"pod_names"`
	}

	InstanceLogsRequest struct {
		ID           int64  `json:"id"`
		OrgName      string `json:"org_name"`
		RepoName     string `json:"repo_name"`
		ClusterID    string `json:"cluster_id"`
		SvcName      string `json:"svc_name"`
		InstanceName string `json:"instance_name"`
	}

	PurgeRequest struct {
		ID         int64  `json:"id"`
		OrgName    string `json:"org_name"`
		RepoName   string `json:"repo_name"`
		ClusterID  string `json:"cluster_id"`
		SvcName    string `json:"svc_name"`
		DeployType int    `json:"deploy_type"`
		UserID     string `json:"user_id"`
	}

	PurgeResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	Cluster struct {
		ID            string                // Unique identifier for the cluster
		ConfigPath    string                // Path to the kubeconfig file
		Client        *kubernetes.Clientset // Kubernetes client
		KnativeClient *knative.Clientset    // Knative client
	}
	SVCRequest struct {
		ImageID       string            `json:"image_id" binding:"required"`
		Hardware      HardWare          `json:"hardware,omitempty"`
		Env           map[string]string `json:"env,omitempty"`
		Annotation    map[string]string `json:"annotation,omitempty"`
		DeployID      int64             `json:"deploy_id" binding:"required"`
		RepoType      string            `json:"repo_type"`
		MinReplica    int               `json:"min_replica"`
		MaxReplica    int               `json:"max_replica"`
		ClusterID     string            `json:"cluster_id"`
		DeployType    int               `json:"deploy_type"`
		UserID        string            `json:"user_id"`
		Sku           string            `json:"sku"`
		OrderDetailID int64             `json:"order_detail_id"`
		SvcName       string            `json:"-"`
		TaskId        int64             `json:"task_id"`
		Nodes         []Node            `json:"nodes"`

		StrategyType string     `json:"strategy_type"` // blue_green/canary
		Scheduler    *Scheduler `json:"scheduler,omitempty"`
	}

	Scheduler struct {
		Volcano *VolcanoConfig `json:"volcano,omitempty"`
	}

	VolcanoConfig struct {
		SchedulerName     string              `json:"schedulerName,omitempty"`
		PriorityClassName string              `json:"priorityClassName,omitempty"`
		Queue             string              `json:"queue,omitempty"`
		MinAvailable      int32               `json:"minAvailable,omitempty"`
		Policies          []VolcanoPolicy     `json:"policies,omitempty"`
		Plugins           map[string][]string `json:"plugins,omitempty"`
		MaxRetry          int32               `json:"maxRetry,omitempty"`
		VGPUMode          string              `json:"volcano.sh/vgpu-mode,omitempty"`
	}

	VolcanoPolicy struct {
		Event  string `json:"event"`
		Action string `json:"action"`
	}

	EngineArg struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Format string `json:"format"`
	}

	TrafficTarget struct {
		RevisionName string `json:"revision_name,omitempty"`
		Percent      int64  `json:"percent"`
	}
	TrafficReq struct {
		Commit         string `json:"commit"`
		TrafficPercent int64  `json:"traffic_percent"`
	}

	CreateRevisionReq struct {
		ClusterID      string `json:"cluster_id"`
		SvcName        string `json:"svc_name"`
		Commit         string `json:"commit"`
		InitialTraffic int    `json:"initial_traffic"`
	}

	CreateRevisionResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	KsvcRevisionInfo struct {
		RevisionName   string    `json:"revision_name"`
		Commit         string    `json:"commit"`
		CreateTime     time.Time `json:"create_time"`
		IsReady        bool      `json:"is_ready"`
		TrafficPercent int64     `json:"traffic_percent"`
		Message        string    `json:"message"`
		Reason         string    `json:"reason"`
	}
)
