{{/* title section */}}
{{if eq .event_type "create"}}Your CSGHub issue has been created{{else if eq .event_type "comment"}}New comment on your CSGHub issue #{{.issue_id}}{{else}}Your CSGHub issue has been resolved{{end}}
---
{{/* content section */}}
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
	<p style="margin-bottom: 20px;">Hi{{if .user_name}} {{.user_name}}{{end}},</p>

	{{if eq .event_type "create"}}
	<h2 style="color: #1e90ff; margin-top: 0; font-size: 18px;">Your CSGHub issue has been created</h2>
	<p style="margin-bottom: 16px;">Thank you for submitting your feedback. We have received your issue and will process it as soon as possible.</p>
	<div style="margin: 24px 0; padding: 16px; background-color: #f5f9ff; border-left: 4px solid #1e90ff; border-radius: 4px;">
		<p style="margin: 0 0 8px 0; font-size: 12px; color: #666;">Issue #{{.issue_id}}</p>
		<p style="margin: 0; font-weight: bold;">{{.issue_title}}</p>
	</div>
	{{else if eq .event_type "comment"}}
	<h2 style="color: #1e90ff; margin-top: 0; font-size: 18px;">New comment on your CSGHub issue</h2>
	<p style="margin-bottom: 16px;">Your issue has received a new comment.</p>
	<div style="margin: 24px 0; padding: 16px; background-color: #f9f9f9; border-radius: 4px;">
		<p style="margin: 0 0 8px 0; font-size: 12px; color: #666;">Issue #{{.issue_id}} · {{.issue_title}}</p>
		{{if .comment}}
		<div style="margin-top: 12px; padding: 12px; background-color: #fff; border: 1px solid #e0e0e0; border-radius: 4px;">
			<p style="margin: 0; white-space: pre-wrap;">{{.comment}}</p>
		</div>
		{{end}}
	</div>
	{{else}}
	<h2 style="color: #2e7d32; margin-top: 0; font-size: 18px;">Your CSGHub issue has been resolved</h2>
	<p style="margin-bottom: 16px;">Thank you for your feedback. The following issue has been resolved:</p>
	<div style="margin: 24px 0; padding: 16px; background-color: #f1f8e9; border-left: 4px solid #2e7d32; border-radius: 4px;">
		<p style="margin: 0 0 8px 0; font-size: 12px; color: #666;">Issue #{{.issue_id}}</p>
		<p style="margin: 0; font-weight: bold;">{{.issue_title}}</p>
	</div>
	<p style="margin-top: 20px;">We appreciate your contribution to the CSGHub community.</p>
	{{end}}

	<p style="margin-top: 32px; padding-top: 16px; border-top: 1px solid #eee; font-size: 12px; color: #888;">CSGHub Team</p>
</body>
</html>
