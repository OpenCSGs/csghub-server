
{{/* title section */}}
{{if eq .deploy_type "space"}}
Space {{.git_path}} Is Running
{{else if eq .deploy_type "inference"}}
Inference {{.deploy_name}}/{{.deploy_id}} Is Running
{{else if eq .deploy_type "finetune"}}
Finetune {{.deploy_name}}/{{.deploy_id}} Is Running
{{else if eq .deploy_type "evaluation"}}
Evaluation {{.deploy_name}} Starts Running
{{else if eq .deploy_type "serverless"}}
Serverless Is Running
{{else}}
Instance Is Running
{{end}}
---
{{/* content section */}}
<html>
    <body>
        <h3>
            {{if eq .deploy_type "space"}}
            Space {{.git_path}} Is Running
            {{else if eq .deploy_type "inference"}}
            Inference {{.deploy_name}}/{{.deploy_id}} Is Running
            {{else if eq .deploy_type "finetune"}}
            Finetune {{.deploy_name}}/{{.deploy_id}} Is Running
            {{else if eq .deploy_type "evaluation"}}
            Evaluation {{.deploy_name}} Starts Running.
            {{else if eq .deploy_type "serverless"}}
            Serverless Is Running
            {{else}}
            Instance Is Running
            {{end}} 
        </h3>
        <p>
            {{if eq .deploy_type "space"}}
            Your dedicated space instance <strong>{{.git_path}}</strong> is running successfully.
            {{else if eq .deploy_type "inference"}}
            Your inference endpoint <strong>{{.deploy_name}}/{{.deploy_id}}</strong> is running successfully.
            {{else if eq .deploy_type "finetune"}}
            Your finetune instance <strong>{{.deploy_name}}/{{.deploy_id}}</strong> is running successfully.
            {{else if eq .deploy_type "evaluation"}}
            Your evaluation task <strong>{{.deploy_name}}</strong> starts running successfully.
            {{else if eq .deploy_type "serverless"}}
            Your serverless instance is running successfully.
            {{else}}
            Your instance is running successfully.
            {{end}}
        </p>
    </body>
</html> 