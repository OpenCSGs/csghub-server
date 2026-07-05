package types

func (req EvaluationReq) ToClawEvaluationReq() ClawEvaluationReq {
	return ClawEvaluationReq{
		Username:           req.Username,
		OwnerNamespace:     req.OwnerNamespace,
		TaskName:           req.TaskName,
		TaskDesc:           req.TaskDesc,
		RuntimeFrameworkId: req.RuntimeFrameworkId,
		ResourceId:         req.ResourceId,
		Command:            req.Command,
		Model:              req.Model,
		BaseURL:            req.BaseURL,
		ApiKey:             req.ApiKey,
		Config:             req.Config,
		Tasks:              req.Tasks,
		Trials:             req.Trials,
		Parallel:           req.Parallel,
		JudgeModel:         req.JudgeModel,
		NoJudge:            req.NoJudge,
		TraceDir:           req.TraceDir,
		Proxy:              req.Proxy,
		Token:              req.Token,
		Hardware:           req.Hardware,
		UserUUID:           req.UserUUID,
		ClusterID:          req.ClusterID,
		Image:              req.Image,
		RepoType:           req.RepoType,
		TaskType:           req.TaskType,
		ResourceName:       req.ResourceName,
		Nodes:              req.Nodes,
		DeployExtend:       req.DeployExtend,
	}
}

type ClawEvaluationReq struct {
	Username           string `json:"-"`
	OwnerNamespace     string `json:"owner_namespace,omitempty"`
	TaskName           string `json:"task_name" binding:"required"`
	TaskDesc           string `json:"task_desc"`
	RuntimeFrameworkId int64  `json:"runtime_framework_id" binding:"required"`
	ResourceId         int64  `json:"resource_id" binding:"required"`

	Command    string `json:"command,omitempty"`
	Model      string `json:"model" binding:"required"`
	BaseURL    string `json:"base_url" binding:"required"`
	ApiKey     string `json:"api_key,omitempty"`
	Config     string `json:"config,omitempty"`
	Tasks      string `json:"tasks,omitempty"`
	Trials     int    `json:"trials,omitempty"`
	Parallel   int    `json:"parallel,omitempty"`
	JudgeModel string `json:"judge_model,omitempty"`
	NoJudge    bool   `json:"no_judge,omitempty"`
	TraceDir   string `json:"trace_dir,omitempty"`
	Proxy      string `json:"proxy,omitempty"`

	JudgeBaseURL string `json:"-"`
	JudgeApiKey  string `json:"-"`

	Token          string   `json:"-"`
	Hardware       HardWare `json:"-"`
	UserUUID       string   `json:"-"`
	ClusterID      string   `json:"-"`
	Image          string   `json:"-"`
	RepoType       string   `json:"-"`
	TaskType       TaskType `json:"-"`
	ResourceName   string   `json:"-"`
	Nodes          []Node   `json:"-"`
	DeployExtend
}
