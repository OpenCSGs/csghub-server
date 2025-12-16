package component

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

// AgentComponent defines the interface for agent-related operations
type AgentComponent interface {
	// Template operations
	CreateTemplate(ctx context.Context, template *types.AgentTemplate) error
	GetTemplateByID(ctx context.Context, id int64, userUUID string) (*types.AgentTemplate, error)
	ListTemplatesByUserUUID(ctx context.Context, userUUID string, filter types.AgentTemplateFilter, per int, page int) ([]types.AgentTemplate, int, error)
	UpdateTemplate(ctx context.Context, template *types.AgentTemplate) error
	DeleteTemplate(ctx context.Context, id int64, userUUID string) error

	// Instance operations
	CreateInstance(ctx context.Context, instance *types.AgentInstance) error
	GetInstanceByID(ctx context.Context, id int64, userUUID string) (*types.AgentInstance, error)
	IsInstanceExistsByContentID(ctx context.Context, instanceType string, instanceContentID string) (bool, error)
	ListInstancesByUserUUID(ctx context.Context, userUUID string, filter types.AgentInstanceFilter, per int, page int) ([]*types.AgentInstance, int, error)
	UpdateInstance(ctx context.Context, instance *types.AgentInstance) error
	UpdateInstanceByContentID(ctx context.Context, userUUID string, instanceType string, instanceContentID string, updateRequest types.UpdateAgentInstanceRequest) (*types.AgentInstance, error)
	DeleteInstance(ctx context.Context, id int64, userUUID string) error
	DeleteInstanceByContentID(ctx context.Context, userUUID string, instanceType string, instanceContentID string) error

	// Session operations
	CreateSession(ctx context.Context, userUUID string, req *types.CreateAgentInstanceSessionRequest) (sessionUUID string, err error)
	ListSessions(ctx context.Context, userUUID string, filter types.AgentInstanceSessionFilter, per int, page int) ([]*types.AgentInstanceSession, int, error)
	GetSessionByUUID(ctx context.Context, userUUID string, sessionUUID string, instanceID int64) (*types.AgentInstanceSession, error)
	DeleteSessionByUUID(ctx context.Context, userUUID string, sessionUUID string, instanceID int64) error
	UpdateSessionByUUID(ctx context.Context, userUUID string, sessionUUID string, instanceID int64, req *types.UpdateAgentInstanceSessionRequest) error
	ListSessionHistories(ctx context.Context, userUUID string, sessionUUID string, instanceID int64) ([]*types.AgentInstanceSessionHistory, error)
	CreateSessionHistories(ctx context.Context, userUUID string, instanceID int64, req *types.CreateSessionHistoryRequest) (*types.CreateSessionHistoryResponse, error)
	UpdateSessionHistoryFeedback(ctx context.Context, userUUID string, instanceID int64, sessionUUID string, req *types.FeedbackSessionHistoryRequest) error
	RewriteSessionHistory(ctx context.Context, userUUID string, instanceID int64, sessionUUID string, req *types.RewriteSessionHistoryRequest) (*types.RewriteSessionHistoryResponse, error)

	// Task operations
	CreateTaskIfInstanceExists(ctx context.Context, req *types.AgentInstanceTaskReq) error
	ListTasks(ctx context.Context, userUUID string, filter types.AgentTaskFilter, per int, page int) ([]types.AgentTaskListItem, int, error)
	GetTaskDetail(ctx context.Context, userUUID string, id int64) (*types.AgentTaskDetail, error)

	// Status operations
	GetInstancesStatus(ctx context.Context, userUUID string, instanceIDs []int64) ([]types.AgentInstanceStatusResponse, error)
	SetMonitor(ctx context.Context, userUUID string, request types.AgentMonitorRequest) error
	GetMonitor(ctx context.Context, monitorID string) ([]int64, error)
	RefreshMonitor(ctx context.Context, monitorID string) error

	// MCP Server operations
	CreateMCPServer(ctx context.Context, server *types.AgentMCPServer) error
	GetMCPServerByID(ctx context.Context, id string, userUUID string) (*types.AgentMCPServerDetail, error)
	ListMCPServers(ctx context.Context, userUUID string, filter types.AgentMCPServerFilter, per int, page int) ([]types.AgentMCPServerListItem, int, error)
	UpdateMCPServer(ctx context.Context, id string, userUUID string, req *types.UpdateAgentMCPServerRequest) error
	DeleteMCPServer(ctx context.Context, id string, userUUID string) error

	// Knowledge Base operations
	CreateKnowledgeBase(ctx context.Context, req *types.CreateAgentKnowledgeBaseReq) (*types.AgentKnowledgeBase, error)
	GetKnowledgeBaseByID(ctx context.Context, id int64, userUUID string) (*types.AgentKnowledgeBaseDetail, error)
	ListKnowledgeBases(ctx context.Context, userUUID string, filter types.AgentKnowledgeBaseFilter, per int, page int) ([]types.AgentKnowledgeBaseListItem, int, error)
	UpdateKnowledgeBase(ctx context.Context, id int64, userUUID string, req *types.UpdateAgentKnowledgeBaseRequest) error
	UpdateKnowledgeBaseByContentID(ctx context.Context, contentID string, userUUID string, req *types.UpdateAgentKnowledgeBaseRequest) error
	DeleteKnowledgeBase(ctx context.Context, id int64, userUUID string) error
	DeleteKnowledgeBaseByContentID(ctx context.Context, contentID string, userUUID string) error
}
