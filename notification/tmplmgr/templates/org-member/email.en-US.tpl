{{/* title section */}}
{{if eq .operation "add"}}
    Organization member change
{{else if eq .operation "remove"}}
    Organization member change
{{else if eq .operation "update"}}
    Organization member role change
{{else}}
    Organization member change
{{end}}
---
{{/* content section */}}
<html>
    <body>
        <h3>
            {{if eq .operation "add"}}
                Organization member change
            {{else if eq .operation "remove"}}
                Organization member change
            {{else if eq .operation "update"}}
                Organization member role change
            {{else}}
                Organization member change
            {{end}}
        </h3>
        {{if eq .operation "add"}}
            <p>New member {{.user_name}} joined organization {{.org_name}}.</p>
        {{else if eq .operation "remove"}}
            <p>{{.user_name}} left the organization {{.org_name}}.</p>
        {{else if eq .operation "update"}}
            <p>Changed permission of member {{.user_name}} to {{.new_role}} in organization {{.org_name}}.</p>
        {{else}}
            <p>Organization member change in {{.org_name}}.</p>
        {{end}}
    </body>
</html> 