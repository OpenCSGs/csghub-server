{
	"zh_cn": {
		"title": "Agentichub 用户反馈",
		"content": [
			[
				{"tag": "text", "text": "使用场景: {{.use_scene}}\n"},
				{"tag": "text", "text": "问题模块: {{.problem_module}}\n"},
				{"tag": "text", "text": "问题描述: {{.problem_description}}\n"},
				{"tag": "text", "text": "截图链接:\n{{range $i, $url := .screenshot_urls}}{{if $i}}\n{{end}}{{$url}}{{end}}\n"},
				{"tag": "text", "text": "改进方向: {{range $i, $area := .improvement_areas}}{{if $i}}, {{end}}{{$area}}{{end}}\n"},
				{"tag": "text", "text": "建议: {{.suggestions}}\n"},
				{"tag": "text", "text": "用户ID: {{.user_id}}\n"},
				{"tag": "text", "text": "用户名: {{.user_name}}\n"},
				{"tag": "text", "text": "邮箱: {{.user_email}}\n"},
				{"tag": "text", "text": "手机: {{.user_phone}}\n"},
				{"tag": "text", "text": "提交时间：{{.submit_time}}\n"}
			]
		]
	}
}
