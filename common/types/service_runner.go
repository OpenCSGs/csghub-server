package types

import (
	"io"

	"k8s.io/client-go/kubernetes"
	knative "knative.dev/serving/pkg/client/clientset/versioned"
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

		RuntimeFramework string `json:"runtime_framework"` // runtime framework of image, TGI/vllm/Pipeline/Deepspeed/LLamacpp
		ImageID          string `json:"image_id"`          // container_image
		DeployID         int64  `json:"deploy_id"`
		Accesstoken      string `json:"access_token"`
		ClusterID        string `json:"cluster_id"`
		SvcName          string `json:"svc_name"`
		DeployType       int    `json:"deploy_type"`
		UserID           string `json:"user_id"`
		Sku              string `json:"sku"`
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
		DeployID    int64      `json:"deploy_id"`
		UserID      string     `json:"user_id"`
		Code        int        `json:"code"`
		Message     string     `json:"message"`
		Endpoint    string     `json:"url"`
		Instances   []Instance `json:"instance"`
		Replica     int        `json:"replica"`
		DeployType  int        `json:"deploy_type"`
		ServiceName string     `json:"service_name"`
		DeploySku   string     `json:"deploy_sku"`
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

	CluserResponse struct {
		ClusterID    string                      `json:"cluster_id"`
		ClusterName  string                      `json:"cluster_name"`
		Region       string                      `json:"region"`
		Nodes        map[string]NodeResourceInfo `json:"nodes"`
		Zone         string                      `json:"zone"`
		Provider     string                      `json:"provider"`
		StorageClass string                      `json:"storage_class"`
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
		ClusterID string `json:"cluster_id"`
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
		ID        int64  `json:"id"`
		OrgName   string `json:"org_name"`
		RepoName  string `json:"repo_name"`
		ClusterID string `json:"cluster_id"`
		SvcName   string `json:"svc_name"`
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
		ImageID    string            `json:"image_id" binding:"required"`
		Hardware   HardWare          `json:"hardware,omitempty"`
		Env        map[string]string `json:"env,omitempty"`
		Annotation map[string]string `json:"annotation,omitempty"`
		DeployID   int64             `json:"deploy_id" binding:"required"`
		RepoType   string            `json:"repo_type"`
		MinReplica int               `json:"min_replica"`
		MaxReplica int               `json:"max_replica"`
		ClusterID  string            `json:"cluster_id"`
		DeployType int               `json:"deploy_type"`
		UserID     string            `json:"user_id"`
		Sku        string            `json:"sku"`
	}
)
