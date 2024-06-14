package imagerunner

import (
	"io"

	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/types"
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

		Hardware   types.HardWare    `json:"hardware,omitempty"`   // resource requirements
		Env        map[string]string `json:"env,omitempty"`        // runtime env variables
		Annotation map[string]string `json:"annotation,omitempty"` // resource annotations

		RuntimeFramework string `json:"runtime_framework"` // runtime framework of image, TGI/vllm/Pipeline/Deepspeed/LLamacpp
		ImageID          string `json:"image_id"`          // container_image
		DeployID         int64  `json:"deploy_id"`
		Accesstoken      string `json:"access_token"`
		ClusterID        string `json:"cluster_id"`
		SvcName          string `json:"svc_name"`
		DeployType       int    `json:"deploy_type"`
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
	}

	StatusResponse struct {
		DeployID  int64            `json:"deploy_id"`
		Code      int              `json:"code"`
		Message   string           `json:"message"`
		Endpoint  string           `json:"url"`
		Instances []types.Instance `json:"instance"`
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
		ClusterID   string                              `json:"cluster_id"`
		ClusterName string                              `json:"cluster_name"`
		Region      string                              `json:"region"`
		Nodes       map[string]cluster.NodeResourceInfo `json:"nodes"`
		Zone        string                              `json:"zone"`
		Provider    string                              `json:"provider"`
	}

	CluserRequest struct {
		ClusterID     string `json:"cluster_id"`
		ClusterConfig string `json:"cluster_config"`
		Region        string `json:"region"`
		Zone          string `json:"zone"`     //cn-beijing
		Provider      string `json:"provider"` //ali
		Enable        bool   `json:"enable"`
	}

	ReplicaResponse struct {
		DeployID       int64            `json:"deploy_id"`
		Code           int              `json:"code"`
		Message        string           `json:"message"`
		ActualReplica  int              `json:"actual_replica"`
		DesiredReplica int              `json:"desired_replica"`
		Instances      []types.Instance `json:"instance"`
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
	UpdateClusterResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	ClusterRes struct {
		ClusterID string            `json:"cluster_id"`
		Region    string            `json:"region"`
		Zone      string            `json:"zone"`     //cn-beijing
		Provider  string            `json:"provider"` //ali
		Resources []types.Resources `json:"resources"`
	}

	ClusterResponse struct {
		ClusterID string                            `json:"cluster_id"`
		Region    string                            `json:"region"`
		Zone      string                            `json:"zone"`     //cn-beijing
		Provider  string                            `json:"provider"` //ali
		Enable    bool                              `json:"enable"`
		Nodes     map[string]types.NodeResourceInfo `json:"nodes"`
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
)
