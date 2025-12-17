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
	MessageScenarioRecharge             MessageScenario = "recharge"
	MessageScenarioLowBalance           MessageScenario = "low-balance"
	MessageScenarioRechargeSuccess      MessageScenario = "recharge-success"
	MessageScenarioWeeklyRecharges      MessageScenario = "weekly-recharges"
	MessageScenarioDeployment           MessageScenario = "deployment"
	MessageScenarioNegativeBalance      MessageScenario = "negative-balance"

	// inviter pending award notification
	// @Scenario inviter-pending-award
	// @Channels internal-message, email
	// @PayloadFields amount
	// @Template {
	//  "email": {
	//    "en-US": {
	//      "title": "Receive an inviter pending award",
	//      "content": "You receive an pending inviter award ¥{{.amount}}, it will be awarded on the 5th of next month"
	//    },
	//    "zh-CN": {
	//      "title": "收到待发放的邀请奖励",
	//      "content": "你收到待发放的邀请奖励 ¥{{.amount}}，将在下个月5号发放"
	//    },
	//    "zh-HK": {
	//      "title": "收到待發放的邀請獎勵",
	//      "content": "你收到待發放的邀請獎勵 ¥{{.amount}}，將在下個月5號發放"
	//    },
	//  },
	// }
	// @BuildTags saas
	MessageScenarioInviterPendingAward MessageScenario = "inviter-pending-award"

	// inviter award notification
	// @Scenario inviter-award
	// @Channels internal-message, email
	// @PayloadFields amount
	// @Template {
	//  "email": {
	//    "en-US": {
	//      "title": "Receive an inviter award",
	//      "content": "You receive an inviter award ¥{{.amount}}"
	//    },
	//    "zh-CN": {
	//      "title": "收到邀请奖励",
	//      "content": "你收到邀请奖励¥{{.amount}}"
	//    },
	//    "zh-HK": {
	//      "title": "收到邀請獎勵",
	//      "content": "你收到邀請獎勵¥{{.amount}}"
	//    },
	//  },
	// }
	// @BuildTags saas
	MessageScenarioInviterAward MessageScenario = "inviter-award"

	// invitee award notification
	// @Scenario invitee-award
	// @Channels internal-message, email
	// @PayloadFields amount
	// @Template {
	//  "email": {
	//    "en-US": {
	//      "title": "Receive an invitee award",
	//      "content": "You receive an invitee award ¥{{.amount}}"
	//    },
	//    "zh-CN": {
	//      "title": "收到受邀奖励",
	//      "content": "你收到受邀奖励 ¥{{.amount}}"
	//    },
	//    "zh-HK": {
	//      "title": "收到受邀請獎勵",
	//      "content": "你收到受邀請獎勵 ¥{{.amount}}"
	//    },
	//  },
	// }
	// @BuildTags saas
	MessageScenarioInviteeAward MessageScenario = "invitee-award"

	// invitation award be cancelled notification
	// @Scenario invitation-award-cancelled
	// @Channels internal-message, email
	// @PayloadFields amount
	// @Template {
	//  "email": {
	//    "en-US": {
	//      "title": "Invitation award cancelled",
	//      "content": "Invitation award cancelled ¥{{.amount}}, since not used within 90 days"
	//    },
	//    "zh-CN": {
	//      "title": "邀请奖励已取消",
	//      "content": "邀请奖励已取消 ¥{{.amount}}，因90天內未使用"
	//    },
	//    "zh-HK": {
	//      "title": "邀請獎勵已取消",
	//      "content": "邀請獎勵已取消 ¥{{.amount}}，因90天內未使用"
	//    },
	//  },
	// }
	// @BuildTags saas
	MessageScenarioInvitationAwardCancelled MessageScenario = "invitation-award-cancelled"

	// agent instance updated notification
	// @Scenario agent-instance-updated
	// @Channels internal-message, email
	// @PayloadFields instance_type, instance_name
	// @Template {
	//  "email": {
	//    "en-US": {
	//      "title": "Agent Instance Updated",
	//      "content": "Your agent instance '{{.instance_name}}' [{{.instance_type}}] has been successfully updated."
	//    },
	//    "zh-CN": {
	//      "title": "Agent实例已更新",
	//      "content": "您的Agent实例 '{{.instance_name}}' [{{.instance_type}}] 已成功更新。"
	//    },
	//    "zh-HK": {
	//      "title": "Agent實例已更新",
	//      "content": "您的Agent實例 '{{.instance_name}}' [{{.instance_type}}] 已成功更新。"
	//    },
	//  },
	// }
	// @BuildTags ce
	MessageScenarioAgentInstanceUpdated MessageScenario = "agent-instance-updated"

	// agent instance deleted notification
	// @Scenario agent-instance-deleted
	// @Channels internal-message, email
	// @PayloadFields instance_type, instance_name
	// @Template {
	//  "email": {
	//    "en-US": {
	//      "title": "Agent Instance Deleted",
	//      "content": "Your agent instance '{{.instance_name}}' [{{.instance_type}}] has been successfully deleted."
	//    },
	//    "zh-CN": {
	//      "title": "Agent实例已删除",
	//      "content": "您的Agent实例 '{{.instance_name}}' [{{.instance_type}}] 已成功删除。"
	//    },
	//    "zh-HK": {
	//      "title": "Agent實例已刪除",
	//      "content": "您的Agent實例 '{{.instance_name}}' [{{.instance_type}}] 已成功刪除。"
	//    },
	//  },
	// }
	// @BuildTags ce
	MessageScenarioAgentInstanceDeleted MessageScenario = "agent-instance-deleted"

	// invoice created notification
	// @Scenario invoice-created
	// @Channels email
	// @PayloadFields user_name, phone, amount
	// @Template {
	//  "email": {
	//    "zh-CN": {
	//      "title": "用户申请发票",
	//      "content": "用户ID：{{.user_name}}，手机号：{{.phone}}，开票金额：¥{{.amount}}"
	//    },
	//  },
	// }
	// @BuildTags saas
	MessageScenarioInvoiceCreated MessageScenario = "invoice-created"
)
