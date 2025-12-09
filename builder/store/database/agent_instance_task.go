package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// AgentInstanceTask represents the link between an agent instance and an async task
type AgentInstanceTask struct {
	ID          int64               `bun:",pk,autoincrement" json:"id"`
	InstanceID  int64               `bun:",notnull" json:"instance_id"`
	TaskType    types.AgentTaskType `bun:",notnull" json:"task_type"` // Agent task type (e.g., "finetune", "inference")
	TaskID      string              `bun:",notnull" json:"task_id"`
	SessionUUID string              `bun:",notnull" json:"session_uuid"` // Session UUID
	UserUUID    string              `bun:",notnull" json:"user_uuid"`    // User UUID
	DeletedAt   time.Time           `bun:",soft_delete,nullzero" json:"deleted_at"`
	times
}

// AgentInstanceTaskStore provides database operations for AgentInstanceTask
type AgentInstanceTaskStore interface {
	Create(ctx context.Context, task *AgentInstanceTask) (*AgentInstanceTask, error)
	ListTasks(ctx context.Context, userUUID string, filter types.AgentTaskFilter, per int, page int) ([]types.AgentTaskListItem, int, error)
	GetTaskByID(ctx context.Context, userUUID string, id int64) (*types.AgentTaskDetail, error)
}

// TaskSource represents the source table for a task type
type TaskSource string

const (
	TaskSourceArgoWorkflow TaskSource = "argo_workflow"
	TaskSourceDeploy       TaskSource = "deploy"
)

// TaskSourceConfig defines how to query a task source
type TaskSourceConfig struct {
	Source          TaskSource
	JoinTable       string
	JoinCondition   string
	TaskNameColumn  string
	StatusColumn    string
	StatusIsInt     bool   // true if status is int, false if it's a string/enum
	AdditionalWhere string // additional WHERE conditions (e.g., "d.status != ?")
	AdditionalArgs  []any
}

// taskSourceMap maps task types to their source configurations
var taskSourceMap = map[types.AgentTaskType]TaskSourceConfig{
	types.AgentTaskTypeFinetuneJob: {
		Source:         TaskSourceArgoWorkflow,
		JoinTable:      "argo_workflows AS aw",
		JoinCondition:  "ait.task_id = aw.task_id",
		TaskNameColumn: "aw.task_name",
		StatusColumn:   "aw.status",
		StatusIsInt:    false,
	},
	types.AgentTaskTypeInference: {
		Source:          TaskSourceDeploy,
		JoinTable:       "deploys AS d",
		JoinCondition:   "ait.task_id = CAST(d.id AS TEXT)",
		TaskNameColumn:  "d.deploy_name",
		StatusColumn:    "d.status",
		StatusIsInt:     true,
		AdditionalWhere: "d.status != ?",
		AdditionalArgs:  []interface{}{common.Deleted},
	},
}

// agentInstanceTaskStoreImpl is the implementation of AgentInstanceTaskStore
type agentInstanceTaskStoreImpl struct {
	db *DB
}

// NewAgentInstanceTaskStore creates a new AgentInstanceTaskStore
func NewAgentInstanceTaskStore() AgentInstanceTaskStore {
	return &agentInstanceTaskStoreImpl{
		db: defaultDB,
	}
}

// NewAgentInstanceTaskStoreWithDB creates a new AgentInstanceTaskStore with a specific DB
func NewAgentInstanceTaskStoreWithDB(db *DB) AgentInstanceTaskStore {
	return &agentInstanceTaskStoreImpl{
		db: db,
	}
}

// Create inserts a new AgentInstanceTask into the database
func (s *agentInstanceTaskStoreImpl) Create(ctx context.Context, task *AgentInstanceTask) (*AgentInstanceTask, error) {
	res, err := s.db.Core.NewInsert().Model(task).Exec(ctx, task)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"instance_id": task.InstanceID,
			"task_id":     task.TaskID,
		})
	}
	return task, nil
}

// buildListQuery builds a query for listing tasks of a specific type
func (s *agentInstanceTaskStoreImpl) buildListQuery(taskType types.AgentTaskType, userUUID string, filter types.AgentTaskFilter) (*bun.SelectQuery, error) {
	config, exists := taskSourceMap[taskType]
	if !exists {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}

	query := s.db.Core.NewSelect().
		TableExpr("agent_instance_tasks AS ait").
		Join(fmt.Sprintf("JOIN %s ON %s", config.JoinTable, config.JoinCondition))

	statusExpr := fmt.Sprintf("CAST(%s AS TEXT) AS status", config.StatusColumn)

	query = query.ColumnExpr(fmt.Sprintf(`ait.id,
		ait.task_id,
		%s AS task_name,
		? AS task_type,
		%s,
		ait.instance_id,
		ait.session_uuid,
		ait.updated_at,
		ait.created_at`, config.TaskNameColumn, statusExpr), taskType)

	query = query.Where("ait.user_uuid = ?", userUUID).
		Where("ait.task_type = ?", taskType)

	// Apply additional WHERE conditions
	if config.AdditionalWhere != "" {
		query = query.Where(config.AdditionalWhere, config.AdditionalArgs...)
	}

	// Apply search filter
	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(fmt.Sprintf("LOWER(%s) LIKE ?", config.TaskNameColumn), searchPattern)
	}

	// Apply status filter
	if filter.Status != "" {
		statusFilter := s.buildStatusFilter(filter.Status, config)
		if len(statusFilter) > 0 {
			if config.StatusIsInt {
				var statusInts []int
				for _, sf := range statusFilter {
					var si int
					if _, err := fmt.Sscanf(sf, "%d", &si); err == nil {
						statusInts = append(statusInts, si)
					}
				}
				if len(statusInts) > 0 {
					query = query.Where(fmt.Sprintf("%s IN (?)", config.StatusColumn), bun.In(statusInts))
				}
			} else {
				query = query.Where(fmt.Sprintf("CAST(%s AS TEXT) IN (?)", config.StatusColumn), bun.In(statusFilter))
			}
		}
	}

	// Apply instance_id filter
	if filter.InstanceID != nil {
		query = query.Where("ait.instance_id = ?", *filter.InstanceID)
	}

	// Apply session_uuid filter
	if filter.SessionUUID != "" {
		query = query.Where("ait.session_uuid = ?", filter.SessionUUID)
	}

	return query, nil
}

// ListTasks lists agent tasks with filtering and pagination
func (s *agentInstanceTaskStoreImpl) ListTasks(ctx context.Context, userUUID string, filter types.AgentTaskFilter, per int, page int) ([]types.AgentTaskListItem, int, error) {
	type taskRow struct {
		ID          int64     `bun:"id"`
		TaskID      string    `bun:"task_id"`
		TaskName    string    `bun:"task_name"`
		TaskType    string    `bun:"task_type"`
		Status      string    `bun:"status"`
		InstanceID  int64     `bun:"instance_id"`
		SessionUUID string    `bun:"session_uuid"`
		UpdatedAt   time.Time `bun:"updated_at"`
		CreatedAt   time.Time `bun:"created_at"`
	}

	taskTypesToQuery := []types.AgentTaskType{}
	if filter.TaskType != "" {
		if _, exists := taskSourceMap[filter.TaskType]; exists {
			taskTypesToQuery = append(taskTypesToQuery, filter.TaskType)
		}
	} else {
		for taskType := range taskSourceMap {
			taskTypesToQuery = append(taskTypesToQuery, taskType)
		}
	}

	var allRows []taskRow
	for _, taskType := range taskTypesToQuery {
		query, err := s.buildListQuery(taskType, userUUID, filter)
		if err != nil {
			return nil, 0, errorx.HandleDBError(err, map[string]any{
				"user_uuid": userUUID,
			})
		}

		var rows []taskRow
		err = query.Scan(ctx, &rows)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, 0, errorx.HandleDBError(err, map[string]any{
				"user_uuid": userUUID,
			})
		}
		allRows = append(allRows, rows...)
	}

	// Sort by updated_at descending
	sort.Slice(allRows, func(i, j int) bool {
		return allRows[i].UpdatedAt.After(allRows[j].UpdatedAt)
	})

	total := len(allRows)

	// Apply pagination
	start := (page - 1) * per
	end := start + per
	var rows []taskRow
	if start > total {
		rows = []taskRow{}
	} else if end > total {
		rows = allRows[start:]
	} else {
		rows = allRows[start:end]
	}

	// Convert rows to results and map status
	results := make([]types.AgentTaskListItem, len(rows))
	for i, row := range rows {
		results[i] = types.AgentTaskListItem{
			ID:          row.ID,
			TaskID:      row.TaskID,
			TaskName:    row.TaskName,
			TaskType:    types.AgentTaskType(row.TaskType),
			TaskStatus:  s.mapTaskStatus(types.AgentTaskType(row.TaskType), row.Status),
			InstanceID:  row.InstanceID,
			SessionUUID: row.SessionUUID,
			UpdatedAt:   row.UpdatedAt,
			CreatedAt:   row.CreatedAt,
		}
	}

	return results, total, nil
}

// buildDetailQuery builds a query for getting task details by primary key id
func (s *agentInstanceTaskStoreImpl) buildDetailQuery(ctx context.Context, userUUID string, id int64) (*bun.SelectQuery, error) {
	// First, get the task to determine its type
	var task AgentInstanceTask
	err := s.db.Core.NewSelect().Model(&task).Where("id = ?", id).Where("user_uuid = ?", userUUID).Scan(ctx, &task)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorx.ErrDatabaseNoRows
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
			"id":        id,
		})
	}

	config, exists := taskSourceMap[task.TaskType]
	if !exists {
		return nil, fmt.Errorf("unsupported task type: %s", task.TaskType)
	}

	query := s.db.Core.NewSelect().
		TableExpr("agent_instance_tasks AS ait").
		Join("LEFT JOIN agent_instances AS ai ON ait.instance_id = ai.id").
		Join("LEFT JOIN agent_instance_sessions AS ais ON ait.session_uuid = ais.uuid").
		Join("LEFT JOIN users AS u ON ait.user_uuid = u.uuid").
		Join(fmt.Sprintf("JOIN %s ON %s", config.JoinTable, config.JoinCondition))

	// Build query based on task source
	switch config.Source {
	case TaskSourceArgoWorkflow:
		query, err = s.buildArgoWorkflowDetailQuery(query, task.TaskType)
	case TaskSourceDeploy:
		query, err = s.buildDeployDetailQuery(query, task.TaskType, task.TaskID)
	default:
		return nil, fmt.Errorf("unsupported task source: %s", config.Source)
	}
	if err != nil {
		return nil, err
	}

	query = query.Where("ait.user_uuid = ?", userUUID).
		Where("ait.id = ?", id)

	if config.AdditionalWhere != "" {
		query = query.Where(config.AdditionalWhere, config.AdditionalArgs...)
	}

	return query, nil
}

// buildArgoWorkflowDetailQuery builds the column expression for argo_workflow task details
func (s *agentInstanceTaskStoreImpl) buildArgoWorkflowDetailQuery(query *bun.SelectQuery, taskType types.AgentTaskType) (*bun.SelectQuery, error) {
	return query.ColumnExpr(`ait.id,
		ait.task_id,
		aw.task_name,
		aw.task_desc,
		? AS task_type,
		aw.status AS status,
		COALESCE(u.username, '') AS username,
		ait.instance_id,
		ai.type AS instance_type,
		ai.name AS instance_name,
		ait.session_uuid,
		COALESCE(ais.name, '') AS session_name,
		ait.created_at,
		ait.updated_at`,
		taskType), nil
}

// buildDeployDetailQuery builds the column expression for deploy task details
func (s *agentInstanceTaskStoreImpl) buildDeployDetailQuery(query *bun.SelectQuery, taskType types.AgentTaskType, taskID string) (*bun.SelectQuery, error) {
	var deployID int64
	_, err := fmt.Sscanf(taskID, "%d", &deployID)
	if err != nil {
		return nil, fmt.Errorf("invalid task_id for deploy: %w", err)
	}

	return query.ColumnExpr(`ait.id,
		ait.task_id,
		d.deploy_name AS task_name,
		COALESCE(d.message, d.variables, '') AS task_desc,
		? AS task_type,
		CAST(d.status AS TEXT) AS status,
		COALESCE(u.username, '') AS username,
		ait.instance_id,
		ai.type AS instance_type,
		ai.name AS instance_name,
		ait.session_uuid,
		COALESCE(ais.name, '') AS session_name,
		ait.created_at,
		ait.updated_at`,
		taskType).
		Where("d.id = ?", deployID), nil
}

// GetTaskByID retrieves a task detail by task ID (primary key)
func (s *agentInstanceTaskStoreImpl) GetTaskByID(ctx context.Context, userUUID string, id int64) (*types.AgentTaskDetail, error) {
	type taskDetailRow struct {
		ID           int64     `bun:"id"`
		TaskID       string    `bun:"task_id"`
		TaskName     string    `bun:"task_name"`
		TaskDesc     string    `bun:"task_desc"`
		TaskType     string    `bun:"task_type"`
		Status       string    `bun:"status"`
		Username     string    `bun:"username"`
		InstanceID   int64     `bun:"instance_id"`
		InstanceType string    `bun:"instance_type"`
		InstanceName string    `bun:"instance_name"`
		SessionUUID  string    `bun:"session_uuid"`
		SessionName  string    `bun:"session_name"`
		CreatedAt    time.Time `bun:"created_at"`
		UpdatedAt    time.Time `bun:"updated_at"`
	}

	query, err := s.buildDetailQuery(ctx, userUUID, id)
	if err != nil {
		return nil, err
	}

	var row taskDetailRow
	err = query.Scan(ctx, &row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorx.ErrDatabaseNoRows
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
			"id":        id,
		})
	}

	taskType := types.AgentTaskType(row.TaskType)
	status := s.mapTaskStatus(taskType, row.Status)
	config := taskSourceMap[taskType]

	sourceData, err := s.fetchSourceData(ctx, taskType, row.TaskID)
	if err != nil {
		slog.Warn("Failed to fetch source data", "task_type", taskType, "task_id", row.TaskID, "error", err)
	}

	detail := types.AgentTaskDetail{
		ID:           row.ID,
		TaskID:       row.TaskID,
		TaskName:     row.TaskName,
		TaskDesc:     row.TaskDesc,
		TaskType:     taskType,
		Status:       status,
		Username:     row.Username,
		InstanceID:   row.InstanceID,
		InstanceType: row.InstanceType,
		InstanceName: row.InstanceName,
		SessionUUID:  row.SessionUUID,
		SessionName:  row.SessionName,
		Backend:      string(config.Source),
		Metadata:     sourceData,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}

	return &detail, nil
}

// fetchSourceData fetches source-specific data based on task type
func (s *agentInstanceTaskStoreImpl) fetchSourceData(ctx context.Context, taskType types.AgentTaskType, taskID string) (map[string]any, error) {
	config, exists := taskSourceMap[taskType]
	if !exists {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}

	switch config.Source {
	case TaskSourceArgoWorkflow:
		return s.fetchArgoWorkflowSourceData(ctx, taskID)
	case TaskSourceDeploy:
		return s.fetchDeploySourceData(ctx, taskID)
	default:
		return nil, fmt.Errorf("unsupported task source: %s", config.Source)
	}
}

// fetchArgoWorkflowSourceData fetches additional fields from argo_workflows table
func (s *agentInstanceTaskStoreImpl) fetchArgoWorkflowSourceData(ctx context.Context, taskID string) (map[string]any, error) {
	type argoWorkflowRow struct {
		JobID        string    `bun:"id"`
		TaskDesc     string    `bun:"task_desc"`
		ResultURL    string    `bun:"result_url"`
		RepoIds      []string  `bun:"repo_ids"`
		Datasets     []string  `bun:"datasets"`
		SubmitTime   time.Time `bun:"submit_time"`
		StartTime    time.Time `bun:"start_time"`
		EndTime      time.Time `bun:"end_time"`
		ClusterID    string    `bun:"cluster_id"`
		Namespace    string    `bun:"namespace"`
		RepoType     string    `bun:"repo_type"`
		Reason       string    `bun:"reason"`
		Image        string    `bun:"image"`
		ResourceId   int64     `bun:"resource_id"`
		ResourceName string    `bun:"resource_name"`
		DownloadURL  string    `bun:"download_url"`
		FailuresURL  string    `bun:"failures_url"`
	}

	var row argoWorkflowRow
	err := s.db.Core.NewSelect().
		TableExpr("argo_workflows").
		ColumnExpr("id, task_desc, result_url, repo_ids, datasets, submit_time, start_time, end_time, cluster_id, namespace, repo_type, reason, image, resource_id, resource_name, download_url, failures_url").
		Where("task_id = ?", taskID).
		Scan(ctx, &row)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var duration *int64
	if !row.EndTime.IsZero() && !row.StartTime.IsZero() {
		dur := int64(row.EndTime.Sub(row.StartTime).Seconds())
		duration = &dur
	}

	sourceData := map[string]any{
		"job_id":        row.JobID,
		"cluster_id":    row.ClusterID,
		"namespace":     row.Namespace,
		"repo_ids":      row.RepoIds,
		"repo_type":     row.RepoType,
		"reason":        row.Reason,
		"image":         row.Image,
		"datasets":      row.Datasets,
		"resource_id":   row.ResourceId,
		"resource_name": row.ResourceName,
		"submit_time":   row.SubmitTime,
		"start_time":    row.StartTime,
		"end_time":      row.EndTime,
		"duration":      duration,
		"result_url":    row.ResultURL,
		"download_url":  row.DownloadURL,
		"failures_url":  row.FailuresURL,
	}
	return sourceData, nil
}

// fetchDeploySourceData fetches additional fields from deploys table
func (s *agentInstanceTaskStoreImpl) fetchDeploySourceData(ctx context.Context, taskID string) (map[string]any, error) {
	var deployID int64
	_, err := fmt.Sscanf(taskID, "%d", &deployID)
	if err != nil {
		return nil, fmt.Errorf("invalid task_id for deploy: %w", err)
	}

	type deployRow struct {
		SpaceID          int64     `bun:"space_id"`
		GitPath          string    `bun:"git_path"`
		GitBranch        string    `bun:"git_branch"`
		Env              string    `bun:"env"`
		Template         string    `bun:"template"`
		Hardware         string    `bun:"hardware"`
		ImageID          string    `bun:"image_id"`
		RuntimeFramework string    `bun:"runtime_framework"`
		ContainerPort    int       `bun:"container_port"`
		MinReplica       int       `bun:"min_replica"`
		MaxReplica       int       `bun:"max_replica"`
		SvcName          string    `bun:"svc_name"`
		ClusterID        string    `bun:"cluster_id"`
		SecureLevel      int       `bun:"secure_level"`
		Task             string    `bun:"task"`
		SKU              string    `bun:"sku"`
		OrderDetailID    int64     `bun:"order_detail_id"`
		EngineArgs       string    `bun:"engine_args"`
		Variables        string    `bun:"variables"`
		Reason           string    `bun:"reason"`
		UpdatedAt        time.Time `bun:"updated_at"`
	}

	var row deployRow
	err = s.db.Core.NewSelect().
		TableExpr("deploys").
		ColumnExpr("space_id, git_path, git_branch, env, template, hardware, image_id, runtime_framework, container_port, min_replica, max_replica, svc_name, cluster_id, secure_level, task, sku, order_detail_id, engine_args, variables, reason, updated_at").
		Where("id = ?", deployID).
		Scan(ctx, &row)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	sourceData := map[string]any{
		"space_id":          row.SpaceID,
		"git_path":          row.GitPath,
		"git_branch":        row.GitBranch,
		"env":               row.Env,
		"template":          row.Template,
		"hardware":          row.Hardware,
		"image_id":          row.ImageID,
		"runtime_framework": row.RuntimeFramework,
		"container_port":    row.ContainerPort,
		"min_replica":       row.MinReplica,
		"max_replica":       row.MaxReplica,
		"svc_name":          row.SvcName,
		"cluster_id":        row.ClusterID,
		"secure_level":      row.SecureLevel,
		"task":              row.Task,
		"sku":               row.SKU,
		"order_detail_id":   row.OrderDetailID,
		"engine_args":       row.EngineArgs,
		"variables":         row.Variables,
		"reason":            row.Reason,
		"updated_at":        row.UpdatedAt,
	}

	return sourceData, nil
}

// buildStatusFilter converts unified status to task-specific status values based on source config
func (s *agentInstanceTaskStoreImpl) buildStatusFilter(unifiedStatus types.AgentTaskStatus, config TaskSourceConfig) []string {
	switch unifiedStatus {
	case types.AgentTaskStatusInProgress:
		return s.getInProgressStatuses(config.Source)
	case types.AgentTaskStatusCompleted:
		return s.getCompletedStatuses(config.Source)
	case types.AgentTaskStatusFailed:
		return s.getFailedStatuses(config.Source)
	default:
		return []string{}
	}
}

// getInProgressStatuses returns status values for in_progress state based on source
func (s *agentInstanceTaskStoreImpl) getInProgressStatuses(source TaskSource) []string {
	switch source {
	case TaskSourceArgoWorkflow:
		return []string{string(v1alpha1.WorkflowRunning)}
	case TaskSourceDeploy:
		return []string{
			strconv.Itoa(common.Running),
			strconv.Itoa(common.Deploying),
			strconv.Itoa(common.Building),
			strconv.Itoa(common.BuildInQueue),
		}
	default:
		return []string{}
	}
}

// getCompletedStatuses returns status values for completed state based on source
func (s *agentInstanceTaskStoreImpl) getCompletedStatuses(source TaskSource) []string {
	switch source {
	case TaskSourceArgoWorkflow:
		return []string{string(v1alpha1.WorkflowSucceeded)}
	case TaskSourceDeploy:
		return []string{strconv.Itoa(common.Running)} // Running is considered completed for inference
	default:
		return []string{}
	}
}

// getFailedStatuses returns status values for failed state based on source
func (s *agentInstanceTaskStoreImpl) getFailedStatuses(source TaskSource) []string {
	switch source {
	case TaskSourceArgoWorkflow:
		return []string{
			string(v1alpha1.WorkflowFailed),
			string(v1alpha1.WorkflowError),
		}
	case TaskSourceDeploy:
		return []string{
			strconv.Itoa(common.BuildFailed),
			strconv.Itoa(common.DeployFailed),
			strconv.Itoa(common.RunTimeError),
			strconv.Itoa(common.Stopped),
			strconv.Itoa(common.Deleted),
		}
	default:
		return []string{}
	}
}

// mapTaskStatus maps task-specific status to unified status
func (s *agentInstanceTaskStoreImpl) mapTaskStatus(taskType types.AgentTaskType, status string) types.AgentTaskStatus {
	switch taskType {
	case types.AgentTaskTypeFinetuneJob:
		return s.mapArgoWorkflowStatus(status)
	case types.AgentTaskTypeInference:
		return s.mapDeployStatus(status)
	default:
		// Default to in progress for unknown task types
		return types.AgentTaskStatusInProgress
	}
}

// mapArgoWorkflowStatus maps ArgoWorkflow status to unified status
func (s *agentInstanceTaskStoreImpl) mapArgoWorkflowStatus(status string) types.AgentTaskStatus {
	switch v1alpha1.WorkflowPhase(status) {
	case v1alpha1.WorkflowRunning:
		return types.AgentTaskStatusInProgress
	case v1alpha1.WorkflowSucceeded:
		return types.AgentTaskStatusCompleted
	case v1alpha1.WorkflowFailed, v1alpha1.WorkflowError:
		return types.AgentTaskStatusFailed
	default:
		return types.AgentTaskStatusInProgress
	}
}

// mapDeployStatus maps Deploy status to unified status
func (s *agentInstanceTaskStoreImpl) mapDeployStatus(status string) types.AgentTaskStatus {
	var statusInt int
	_, err := fmt.Sscanf(status, "%d", &statusInt)
	if err != nil {
		return types.AgentTaskStatusInProgress
	}

	switch statusInt {
	case common.Running:
		return types.AgentTaskStatusCompleted
	case common.Deploying, common.Building, common.BuildInQueue, common.Startup:
		return types.AgentTaskStatusInProgress
	case common.BuildFailed, common.DeployFailed, common.RunTimeError, common.Stopped, common.Deleted:
		return types.AgentTaskStatusFailed
	default:
		return types.AgentTaskStatusInProgress
	}
}
