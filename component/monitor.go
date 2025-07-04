package component

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/prometheus"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type MonitorComponent interface {
	CPUUsage(ctx context.Context, req *types.MonitorReq) (*types.MonitorCPUResp, error)
	MemoryUsage(ctx context.Context, req *types.MonitorReq) (*types.MonitorMemoryResp, error)
	RequestCount(ctx context.Context, req *types.MonitorReq) (*types.MonitorRequestCountResp, error)
	RequestLatency(ctx context.Context, req *types.MonitorReq) (*types.MonitorRequestLatencyResp, error)
}

type monitorComponentImpl struct {
	client          prometheus.PrometheusClient
	userSvcClient   rpc.UserSvcClient
	deployTaskStore database.DeployTaskStore
	repoStore       database.RepoStore
	k8sNameSpace    string
	deployer        deploy.Deployer
}

func NewMonitorComponent(cfg *config.Config) (MonitorComponent, error) {
	domainParts := strings.SplitN(cfg.Space.InternalRootDomain, ".", 2)
	client := prometheus.NewPrometheusClient(cfg)
	usc := rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", cfg.User.Host, cfg.User.Port),
		rpc.AuthWithApiKey(cfg.APIToken))
	return &monitorComponentImpl{
		k8sNameSpace:    domainParts[0],
		client:          client,
		userSvcClient:   usc,
		deployTaskStore: database.NewDeployTaskStore(),
		repoStore:       database.NewRepoStore(),
		deployer:        deploy.NewDeployer(),
	}, nil
}

func (m *monitorComponentImpl) CPUUsage(ctx context.Context, req *types.MonitorReq) (*types.MonitorCPUResp, error) {
	access, err, namespace, container := m.hasPermission(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to check user %s permission, error: %w", req.CurrentUser, err)
	}
	if !access {
		return nil, fmt.Errorf("user %s has no permission to access cpu usage", req.CurrentUser)
	}
	query := fmt.Sprintf("avg_over_time(rate(container_cpu_usage_seconds_total{pod='%s',namespace='%s',container='%s'}[1m])[%s:])[%s:%s]", req.Instance, namespace, container, req.LastDuration, req.LastDuration, req.TimeRange)
	slog.Debug("cpu-usage", slog.Any("query", query))

	promeResp, err := m.client.SerialData(query)
	if err != nil {
		return nil, fmt.Errorf("fail to get cpu usage error: %w", err)
	}
	slog.Debug("get cpu usage", slog.Any("promeResp", promeResp))
	resp := types.MonitorCPUResp{}
	if len(promeResp.Data.Result) < 1 {
		return &resp, nil
	}
	pResult := promeResp.Data.Result[0]
	resp.ResultType = promeResp.Data.ResultType
	rdata := types.MonitorData{}
	rdata.Metric = getMetrics(pResult.Metric)

	limit, err := m.CPULimit(ctx, req)
	if err != nil {
		slog.Warn("fail to get cpu limit", slog.Any("err", err))
	}

	for _, values := range pResult.Values {
		valArray, err := convertToArrayFloat64(values)
		if err != nil {
			slog.Warn("failed to convert cpu values to float array", slog.Any("values", values),
				slog.Any("err", err), slog.Any("req", req))
			continue
		}
		val := math.Min(float64((int64((valArray[1]/limit)*10000))/100), 100)
		// percent val%
		rdata.Values = append(rdata.Values, types.MonitorValue{
			Timestamp: int64(valArray[0]),
			Value:     val,
		})
	}
	resp.Result = append(resp.Result, rdata)
	return &resp, nil
}

func (m *monitorComponentImpl) CPULimit(ctx context.Context, req *types.MonitorReq) (float64, error) {
	query := fmt.Sprintf("kube_pod_container_resource_limits{pod='%s',namespace='%s',resource='cpu'}", req.Instance, m.k8sNameSpace)
	slog.Info("cpu-limit", slog.Any("query", query))

	promeResp, err := m.client.SerialData(query)
	if err != nil {
		return 0, fmt.Errorf("fail to get cpu limit error: %w", err)
	}
	slog.Debug("get cpu limit", slog.Any("promeResp", promeResp))
	if len(promeResp.Data.Result) < 1 {
		return 0, fmt.Errorf("fail to get cpu limit, no result found")
	}
	pResult := promeResp.Data.Result[0]
	if len(pResult.Value) < 2 {
		return 0, fmt.Errorf("fail to get cpu limit, no value found")
	}
	valueArray := pResult.Value
	v, err := convertToFloat64(valueArray[1])
	if err != nil {
		return 0, fmt.Errorf("fail to convert cpu limit value %v to float64, error: %w", valueArray[1], err)
	}
	if v <= 0 {
		return 1, fmt.Errorf("fail to get cpu limit, value %v is less than 0", v)
	}
	return v, nil
}

func (m *monitorComponentImpl) MemoryUsage(ctx context.Context, req *types.MonitorReq) (*types.MonitorMemoryResp, error) {
	access, err, namespace, container := m.hasPermission(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to check user %s permission, error: %w", req.CurrentUser, err)
	}
	if !access {
		return nil, fmt.Errorf("user %s has no permission to access memory usage", req.CurrentUser)
	}

	query := fmt.Sprintf("avg_over_time(container_memory_usage_bytes{pod='%s',namespace='%s',container='%s'}[%s:])[%s:%s]",
		req.Instance, namespace, container, req.LastDuration, req.LastDuration, req.TimeRange)
	slog.Debug("memory-usage", slog.Any("query", query))

	promeResp, err := m.client.SerialData(query)
	if err != nil {
		return nil, fmt.Errorf("fail to get memory usage error: %w", err)
	}
	slog.Debug("get memory usage", slog.Any("promeResp", promeResp))
	resp := types.MonitorMemoryResp{}
	if len(promeResp.Data.Result) < 1 {
		return &resp, nil
	}
	pResult := promeResp.Data.Result[0]
	resp.ResultType = promeResp.Data.ResultType
	rdata := types.MonitorData{}
	rdata.Metric = getMetrics(pResult.Metric)

	for _, values := range pResult.Values {
		valArray, err := convertToArrayFloat64(values)
		if err != nil {
			slog.Warn("failed to convert memory values to float array", slog.Any("values", values),
				slog.Any("err", err), slog.Any("req", req))
			continue
		}
		val := float64(int64((valArray[1]/1024/1024/1024)*100)) / 100 // bytes convert to GB
		rdata.Values = append(rdata.Values, types.MonitorValue{
			Timestamp: int64(valArray[0]),
			Value:     val,
		})
	}
	resp.Result = append(resp.Result, rdata)
	return &resp, nil
}

func (m *monitorComponentImpl) RequestCount(ctx context.Context, req *types.MonitorReq) (*types.MonitorRequestCountResp, error) {
	access, err, namespace, _ := m.hasPermission(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to check user %s permission, error: %w", req.CurrentUser, err)
	}
	if !access {
		return nil, fmt.Errorf("user %s has no permission to access request count", req.CurrentUser)
	}

	// issue: github.com/knative/serving/issues/14925
	query := fmt.Sprintf("avg_over_time(revision_request_count{pod_name='%s',namespace='%s'}[%s:])[%s:%s]",
		req.Instance, namespace, req.LastDuration, req.LastDuration, req.TimeRange)
	slog.Debug("request-count", slog.Any("query", query))

	promeResp, err := m.client.SerialData(query)
	if err != nil {
		return nil, fmt.Errorf("fail to get memory limit error: %w", err)
	}
	slog.Debug("get request count", slog.Any("promeResp", promeResp))
	resp := types.MonitorRequestCountResp{}
	if len(promeResp.Data.Result) < 1 {
		return &resp, nil
	}
	resp.ResultType = promeResp.Data.ResultType

	totalRequestCount := int64(0)
	for _, pResult := range promeResp.Data.Result {
		rdata := types.MonitorData{}
		rdata.Metric = getMetrics(pResult.Metric)
		initCountVal := float64(0)
		if len(pResult.Values) > 0 {
			for idx, pValues := range pResult.Values {
				valArray, err := convertToArrayFloat64(pValues)
				if err != nil {
					slog.Warn("failed to convert request count values to float array", slog.Any("values", pValues),
						slog.Any("err", err), slog.Any("req", req))
					continue
				}
				if idx == 0 {
					initCountVal = valArray[1]
				}
				val := float64(int64(math.Max(valArray[1]-initCountVal, 0)))
				rdata.Values = append(rdata.Values, types.MonitorValue{
					Timestamp: int64(valArray[0]),
					Value:     val,
				})
			}
			valuesLen := len(rdata.Values)
			if valuesLen > 0 {
				totalRequestCount += int64(rdata.Values[valuesLen-1].Value - rdata.Values[0].Value)
			}
		}
		resp.Result = append(resp.Result, rdata)
	}
	resp.TotalRequestCount = totalRequestCount
	return &resp, nil
}

func (m *monitorComponentImpl) RequestLatency(ctx context.Context, req *types.MonitorReq) (*types.MonitorRequestLatencyResp, error) {
	access, err, namespace, _ := m.hasPermission(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to check user %s permission, error: %w", req.CurrentUser, err)
	}
	if !access {
		return nil, fmt.Errorf("user %s has no permission to access request latency", req.CurrentUser)
	}

	query := fmt.Sprintf("sum(increase(revision_app_request_latencies_bucket{pod_name='%s',namespace='%s'}[%s:])) by (le)",
		req.Instance, namespace, req.LastDuration)
	slog.Debug("request-latency", slog.Any("query", query))

	promeResp, err := m.client.SerialData(query)
	if err != nil {
		return nil, fmt.Errorf("fail to get cpu memory usage error: %w", err)
	}
	slog.Debug("get request latency", slog.Any("promeResp", promeResp))
	resp := types.MonitorRequestLatencyResp{}
	if len(promeResp.Data.Result) < 1 {
		return &resp, nil
	}
	resp.ResultType = promeResp.Data.ResultType

	for _, pResult := range promeResp.Data.Result {
		rdata := types.MonitorData{}
		rdata.Metric = getMetrics(pResult.Metric)
		valArray, err := convertToArrayFloat64(pResult.Value)
		if err != nil {
			slog.Warn("failed to convert request latency value to float array", slog.Any("value", pResult.Value),
				slog.Any("err", err), slog.Any("req", req))
			continue
		}
		rdata.Value = types.MonitorValue{
			Timestamp: int64(valArray[0]),
			Value:     float64(int64(valArray[1])),
		}
		resp.Result = append(resp.Result, rdata)
	}

	return &resp, nil
}

func convertToArrayFloat64(values []any) ([]float64, error) {
	var result []float64
	if len(values) < 2 {
		return nil, fmt.Errorf("values %v length is less than 2", values)
	}
	timestamp, err := convertToFloat64(values[0])
	if err != nil {
		return nil, err
	}
	result = append(result, timestamp)
	used, err := convertToFloat64(values[1])
	if err != nil {
		return nil, err
	}
	result = append(result, used)
	return result, nil
}

func convertToFloat64(value any) (float64, error) {
	floatV, ok := value.(float64)
	if ok {
		return floatV, nil
	}

	int64V, ok := value.(int64)
	if ok {
		return float64(int64V), nil
	}

	strV, ok := value.(string)
	if ok {
		floatV, err := strconv.ParseFloat(strV, 64)
		if err != nil {
			return 0, fmt.Errorf("fail to parse string value %s to float, error: %w", strV, err)
		}
		return floatV, nil
	}

	intV, ok := value.(int)
	if ok {
		return float64(intV), nil
	}

	return 0, fmt.Errorf("value %v (type %T) is not int or float or string", value, value)

}

func getMetrics(metrics map[string]string) map[string]string {
	result := map[string]string{}
	podName, ok := metrics["pod"]
	if ok {
		result["instance"] = podName
	}
	serviceName, ok := metrics["service_name"]
	if ok {
		result["service_name"] = serviceName
	}
	nameSpace, ok := metrics["namespace"]
	if ok {
		result["namespace"] = nameSpace
	}
	responseCodeClass, ok := metrics["response_code_class"]
	if ok {
		result["response_code_class"] = responseCodeClass
	}
	le, ok := metrics["le"]
	if ok {
		result["le"] = le
	}
	return result
}

func (m *monitorComponentImpl) hasPermission(ctx context.Context, req *types.MonitorReq) (bool, error, string, string) {
	if req.DeployType == "evaluation" {
		return m.hasPermissionForEval(ctx, req)
	} else {
		return m.hasPermissionForDeploy(ctx, req)
	}
}

func (m *monitorComponentImpl) hasPermissionForDeploy(ctx context.Context, req *types.MonitorReq) (bool, error, string, string) {
	namespace := m.k8sNameSpace
	container := "user-container"
	user, err := m.userSvcClient.GetUserInfo(ctx, req.CurrentUser, req.CurrentUser)
	if err != nil {
		return false, fmt.Errorf("failed to get user %s info error: %w", req.CurrentUser, err), "", ""
	}
	if user == nil {
		return false, fmt.Errorf("user %s not found", req.CurrentUser), "", ""
	}
	dbUser := &database.User{
		RoleMask: strings.Join(user.Roles, ","),
	}
	isAdmin := dbUser.CanAdmin()
	if isAdmin {
		return true, nil, namespace, container
	}
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo by path %s/%s/%s, error: %w", req.RepoType, req.Namespace, req.Name, err), "", ""
	}
	deploy, err := m.deployTaskStore.GetDeployByID(ctx, req.DeployID)
	if err != nil {
		return false, fmt.Errorf("failed to get deploy by id %d, error: %w", req.DeployID, err), "", ""
	}

	if repo.ID != deploy.RepoID {
		return false, fmt.Errorf("invalid deploy id %d", req.DeployID), "", ""
	}

	if !strings.HasPrefix(req.Instance, deploy.SvcName) {
		return false, fmt.Errorf("invalid instance %s", req.Instance), "", ""
	}

	if deploy.UserID != user.ID {
		return false, nil, "", ""
	}
	return true, nil, namespace, container
}

func (m *monitorComponentImpl) hasPermissionForEval(ctx context.Context, req *types.MonitorReq) (bool, error, string, string) {
	user, err := m.userSvcClient.GetUserInfo(ctx, req.CurrentUser, req.CurrentUser)
	if err != nil {
		return false, fmt.Errorf("failed to get user %s info error: %w", req.CurrentUser, err), "", ""
	}
	if user == nil {
		return false, fmt.Errorf("user %s not found", req.CurrentUser), "", ""
	}
	dbUser := &database.User{
		RoleMask: strings.Join(user.Roles, ","),
	}
	isAdmin := dbUser.CanAdmin()
	req2 := types.EvaluationGetReq{
		ID:       req.DeployID,
		Username: req.CurrentUser,
	}
	wf, err := m.deployer.GetEvaluation(ctx, req2)
	if err != nil {
		return false, fmt.Errorf("fail to get evaluation result, %w", err), "", ""
	}
	if isAdmin {
		return true, nil, wf.Namespace, "main"
	}
	if wf.Username != user.Username {
		return false, nil, "", ""
	}
	return true, nil, wf.Namespace, "main"
}
