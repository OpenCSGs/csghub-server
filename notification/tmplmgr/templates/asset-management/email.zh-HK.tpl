{{/* title section */}}
{{if eq .operation "create"}}
    {{printf "[%s] 已創建" .repo_type}}
{{else if eq .operation "delete"}}
    {{printf "[%s] 已刪除" .repo_type}}
{{else}}
    {{printf "[%s] %s" .repo_type .operation}}
{{end}}
---
{{/* content section */}}
<html>
    <h3>
        {{if eq .operation "create"}}
            {{printf "[%s] 已創建" .repo_type}}
        {{else if eq .operation "delete"}}
            {{printf "[%s] 已刪除" .repo_type}}
        {{else}}
            {{printf "[%s] %s" .repo_type .operation}}
        {{end}}
    </h3>
    <p>
        <span>
            {{if eq .operation "create"}}
                {{printf "[%s] 創建成功。" .repo_path}}
            {{else if eq .operation "delete"}}
                {{printf "[%s] 已被刪除。" .repo_path}}
            {{else}}
                {{printf "[%s] %s。" .repo_path .operation}}
            {{end}}
        </span>
    </p>
</html> 