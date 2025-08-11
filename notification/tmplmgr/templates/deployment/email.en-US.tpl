
{{/* title section */}}
{{if eq .deploy_type "space"}}
Space {{.git_path}} Deployed Successfully
{{else if eq .deploy_type "inference"}}
Inference {{.deploy_name}}/{{.deploy_id}} Deployed Successfully
{{else if eq .deploy_type "finetune"}}
Finetune {{.deploy_name}}/{{.deploy_id}} Deployed Successfully
{{else if eq .deploy_type "evaluation"}}
Evaluation {{.deploy_name}} Starts Running
{{else if eq .deploy_type "serverless"}}
Serverless Deployed Successfully
{{else}}
Instance Deployed Successfully
{{end}}
---
{{/* content section */}}
<html>
    <body>
        <h3>
            {{if eq .deploy_type "space"}}
            Space {{.git_path}} Deployed Successfully
            {{else if eq .deploy_type "inference"}}
            Inference {{.deploy_name}}/{{.deploy_id}} Deployed Successfully
            {{else if eq .deploy_type "finetune"}}
            Finetune {{.deploy_name}}/{{.deploy_id}} Deployed Successfully
            {{else if eq .deploy_type "evaluation"}}
            Evaluation {{.deploy_name}} Starts Running.
            {{else if eq .deploy_type "serverless"}}
            Serverless Deployed Successfully
            {{else}}
            Instance Deployed Successfully
            {{end}} 
        </h3>
        <p>
            {{if eq .deploy_type "space"}}
            Your dedicated space instance <strong>{{.git_path}}</strong> has been deployed and is running successfully.
            {{else if eq .deploy_type "inference"}}
            Your Inference endpoint <strong>{{.deploy_name}}/{{.deploy_id}}</strong> has been deployed and is running successfully.
            {{else if eq .deploy_type "finetune"}}
            Your Finetune instance <strong>{{.deploy_name}}/{{.deploy_id}}</strong> has been deployed and is running successfully.
            {{else if eq .deploy_type "evaluation"}}
            Your Evaluation task <strong>{{.deploy_name}}</strong> starts running successfully.
            {{else if eq .deploy_type "serverless"}}
            Your Serverless instance has been deployed and is running successfully.
            {{else}}
            Your instance has been deployed and is running successfully.
            {{end}}
        </p>
    </body>
</html> 