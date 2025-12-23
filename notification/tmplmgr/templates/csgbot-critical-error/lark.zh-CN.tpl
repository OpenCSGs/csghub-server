{
	"zh_cn": {
		"title": "{{.service_name}} - {{.error_type}}",
		"content": [
			[
				{"tag": "text", "text": "服务名称: {{.service_name}}\n"},
				{"tag": "text", "text": "错误类型: {{.error_type}}\n"},
				{"tag": "text", "text": "错误级别: {{.error_level}}\n\n"},
				{"tag": "text", "text": "发生位置: {{.location}}\n"},
				{"tag": "text", "text": "发生时间: {{.timestamp}}\n"},
				{"tag": "text", "text": "环境: {{.environment}}\n\n"},
				{"tag": "text", "text": "错误信息:\n{{.error_message}}\n\n"},
				{"tag": "text", "text": "请求ID: {{.request_id}}\n"},
				{"tag": "text", "text": "堆栈跟踪:\n{{.stack_trace}}\n"}
			]
		]
	}
}

