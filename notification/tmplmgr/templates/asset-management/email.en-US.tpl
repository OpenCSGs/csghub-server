{{/* title section */}}
{{if eq .operation "create"}}
    {{printf "[%s] Created" .repo_type}}
{{else if eq .operation "delete"}}
    {{printf "[%s] Deleted" .repo_type}}
{{else}}
    {{printf "[%s] %s" .repo_type .operation}}
{{end}}
---
{{/* content section */}}
<html>
    <h3>
        {{if eq .operation "create"}}
            {{printf "[%s] Created" .repo_type}}
        {{else if eq .operation "delete"}}
            {{printf "[%s] Deleted" .repo_type}}
        {{else}}
            {{printf "[%s] %s" .repo_type .operation}}
        {{end}}
    </h3>
    <p>
        <span>
            {{if eq .operation "create"}}
                {{printf "[%s] created successfully." .repo_path}}
            {{else if eq .operation "delete"}}
                {{printf "[%s] has been deleted." .repo_path}}
            {{else}}
                {{printf "[%s] %s." .repo_path .operation}}
            {{end}}
        </span>
    </p>
</html>
