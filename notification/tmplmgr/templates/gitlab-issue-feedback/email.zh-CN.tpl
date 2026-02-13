{{/* title section */}}
{{if eq .event_type "create"}}您在 CSGHub 提交的问题已创建{{else if eq .event_type "comment"}}您提交的 CSGHub 问题 #{{.issue_id}} 收到新回复{{else}}您在 CSGHub 提交的问题已解决{{end}}
---
{{/* content section */}}
<html>
<body style="font-family: Arial, 'PingFang SC', 'Microsoft YaHei', sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
	<p style="margin-bottom: 20px;">您好{{if .user_name}}，{{.user_name}}{{end}}：</p>

	{{if eq .event_type "create"}}
	<h2 style="color: #1e90ff; margin-top: 0; font-size: 18px;">您在 CSGHub 提交的问题已创建</h2>
	<p style="margin-bottom: 16px;">感谢您的反馈，我们已收到您提交的问题，将尽快处理。</p>
	<div style="margin: 24px 0; padding: 16px; background-color: #f5f9ff; border-left: 4px solid #1e90ff; border-radius: 4px;">
		<p style="margin: 0 0 8px 0; font-size: 12px; color: #666;">问题 #{{.issue_id}}</p>
		<p style="margin: 0; font-weight: bold;">{{.issue_title}}</p>
	</div>
	{{else if eq .event_type "comment"}}
	<h2 style="color: #1e90ff; margin-top: 0; font-size: 18px;">您提交的问题收到新回复</h2>
	<p style="margin-bottom: 16px;">您提交的问题收到了新回复。</p>
	<div style="margin: 24px 0; padding: 16px; background-color: #f9f9f9; border-radius: 4px;">
		<p style="margin: 0 0 8px 0; font-size: 12px; color: #666;">问题 #{{.issue_id}} · {{.issue_title}}</p>
		{{if .comment}}
		<div style="margin-top: 12px; padding: 12px; background-color: #fff; border: 1px solid #e0e0e0; border-radius: 4px;">
			<p style="margin: 0; white-space: pre-wrap;">{{.comment}}</p>
		</div>
		{{end}}
	</div>
	{{else}}
	<h2 style="color: #2e7d32; margin-top: 0; font-size: 18px;">您在 CSGHub 提交的问题已解决</h2>
	<p style="margin-bottom: 16px;">感谢您的反馈，以下问题已解决：</p>
	<div style="margin: 24px 0; padding: 16px; background-color: #f1f8e9; border-left: 4px solid #2e7d32; border-radius: 4px;">
		<p style="margin: 0 0 8px 0; font-size: 12px; color: #666;">问题 #{{.issue_id}}</p>
		<p style="margin: 0; font-weight: bold;">{{.issue_title}}</p>
	</div>
	<p style="margin-top: 20px;">感谢您对 CSGHub 社区的贡献。</p>
	{{end}}

	<p style="margin-top: 32px; padding-top: 16px; border-top: 1px solid #eee; font-size: 12px; color: #888;">CSGHub 团队</p>
</body>
</html>
