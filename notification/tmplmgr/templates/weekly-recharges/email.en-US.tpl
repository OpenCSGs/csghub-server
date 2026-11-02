{{/* title section */}}
Weekly recharge report{{if .is_staging}} [staging]{{end}}
---
{{/* content section */}}
<html>
    <body>
        <h3>Weekly recharge report</h3>
        <p>Attached is the weekly recharge report for the period from {{ .start_date }} to {{ .end_date }}.<br/>Please check the attached CSV file for detailed transaction records.</p>
        {{if .frontend_url}}
        <p>Portal: <a href="{{ .frontend_url }}">{{ .frontend_url }}</a></p>
        {{end}}
    </body>
</html> 