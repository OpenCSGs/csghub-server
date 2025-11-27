package types

const (

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
)
