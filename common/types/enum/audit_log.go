package enum

type AuditAction string

const (
	AuditActionCreation     AuditAction = "creation"
	AuditActionUpdate       AuditAction = "update"
	AuditActionDeletion     AuditAction = "deletion"
	AuditActionSoftDeletion AuditAction = "soft_deletion"
	AuditActionStopDeploy   AuditAction = "stop_deploy"
	AuditActionStopWorkflow AuditAction = "stop_workflow"
)
