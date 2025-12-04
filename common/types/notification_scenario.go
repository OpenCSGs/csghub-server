package types

type MessageScenario string

const (
	MessageScenarioRepoSync             MessageScenario = "repo-sync"
	MessageScenarioInternalNotification MessageScenario = "internal-notification"
	MessageScenarioEmailVerifyCode      MessageScenario = "email-verify-code"
	MessageScenarioSMSVerifyCode        MessageScenario = "sms-verify-code"
	MessageScenarioAssetManagement      MessageScenario = "asset-management"
	MessageScenarioUserVerify           MessageScenario = "user-verify"
	MessageScenarioOrgVerify            MessageScenario = "org-verify"
	MessageScenarioOrgMember            MessageScenario = "org-member"
	MessageScenarioDiscussion           MessageScenario = "discussion"
	MessageScenarioDeployment           MessageScenario = "deployment"
	MessageScenarioNegativeBalance      MessageScenario = "negative-balance"
)
