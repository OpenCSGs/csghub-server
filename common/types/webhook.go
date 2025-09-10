package types

import "encoding/json"

type WebHookEventType string

const (
	RunnerHeartbeat     WebHookEventType = "runner.heartbeat"
	RunnerClusterCreate WebHookEventType = "runner.cluster.create"
	RunnerClusterUpdate WebHookEventType = "runner.cluster.update"

	RunnerServiceCreate WebHookEventType = "runner.service.create"
	RunnerServiceChange WebHookEventType = "runner.service.change"
	RunnerServiceStop   WebHookEventType = "runner.service.stop"

	RunnerBuilderCreate  WebHookEventType = "runner.builder.create"
	RunnerBuilderSuccess WebHookEventType = "runner.builder.success"
	RunnerBuilderFailure WebHookEventType = "runner.builder.failure"
	RunnerBuilderChange  WebHookEventType = "runner.builder.change"
	RunnerBuilderDelete  WebHookEventType = "runner.builder.delete"

	RunnerWorkflowCreate WebHookEventType = "runner.evaluation.create"
	RunnerWorkflowChange WebHookEventType = "runner.evaluation.change"
)

type WebHookDataType string

const (
	WebHookDataTypeObject WebHookDataType = "object"
	WebHookDataTypeArray  WebHookDataType = "array"
)

type WebHookHeader struct {
	EventType  WebHookEventType `json:"event_type" binding:"required"`
	EventTime  int64            `json:"event_time"`
	ClusterID  string           `json:"cluster_id"`
	RunnerName string           `json:"runner_name"`
	DataType   WebHookDataType  `json:"data_type" binding:"required,oneof=object array"`
}

type WebHookRecvEvent struct {
	WebHookHeader
	Data json.RawMessage `json:"data" binding:"required"`
}

type WebHookSendEvent struct {
	WebHookHeader
	Data any `json:"data"`
}
