{{/* title section */}}
资源申请{{if .is_staging}} [staging]{{end}}
---
{{/* content section */}}
<html>
    <body>
        <h3>资源申请</h3>
        <p>用户 {{.user_name}}（UUID: {{.user_uuid}}）申请开通资源：{{.resource_sku}}，请尽快处理。</p>
        {{if .portal_url}}
        <p>Portal: <a href="{{.portal_url}}">{{.portal_url}}</a></p>
        {{end}}
    </body>
</html>
