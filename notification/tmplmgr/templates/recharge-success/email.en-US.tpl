{{/* title section */}}
User recharge successful{{if .is_staging}} [staging]{{end}}
---
{{/* content section */}}
<html>
    <body>
        <h3>User recharge successful</h3>
        <p>{{.user_name}} (UUID: {{.user_uuid}}) successfully recharged {{if eq .currency "CNY"}}¥{{else if eq .currency "USD"}}${{else}}{{.currency}} {{end}}{{.amount}} via {{.pay_channel}}.</p>
        {{if .email}}
        <p>Email: {{.email}}</p>
        {{end}}
        {{if .phone}}
        <p>Phone: {{.phone}}</p>
        {{end}}
        {{if .user_info_url}}
        <p><a href="{{.user_info_url}}">View User Info</a></p>
        {{end}}
        {{if .frontend_url}}
        <p>Portal: <a href="{{.frontend_url}}">{{.frontend_url}}</a></p>
        {{end}}
    </body>
</html> 