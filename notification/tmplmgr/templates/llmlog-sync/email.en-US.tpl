{{/* title section */}}
LLM log sync {{.status}} for {{.date}}
---
{{/* content section */}}
<html>
    <body style="margin:0;padding:24px;background:#f6f8fa;color:#24292f;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;font-size:14px;line-height:1.5;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:880px;margin:0 auto;background:#ffffff;border:1px solid #d0d7de;border-radius:8px;border-collapse:separate;">
            <tr>
                <td style="padding:20px 24px;border-bottom:1px solid #d0d7de;">
                    <h2 style="margin:0 0 8px;font-size:20px;line-height:1.3;color:#24292f;">LLM log sync {{.status}}</h2>
                    <div style="font-size:13px;color:#57606a;">Date: {{.date}}</div>
                </td>
            </tr>
            <tr>
                <td style="padding:20px 24px;">
                    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="border-collapse:collapse;">
                        <tr>
                            <td style="width:220px;padding:8px 12px;border-bottom:1px solid #d8dee4;color:#57606a;">Status</td>
                            <td style="padding:8px 12px;border-bottom:1px solid #d8dee4;font-weight:600;">{{.status}}</td>
                        </tr>
                        <tr>
                            <td style="width:220px;padding:8px 12px;border-bottom:1px solid #d8dee4;color:#57606a;">Discovered source objects</td>
                            <td style="padding:8px 12px;border-bottom:1px solid #d8dee4;">{{.file_count}}</td>
                        </tr>
                        <tr>
                            <td style="width:220px;padding:8px 12px;border-bottom:1px solid #d8dee4;color:#57606a;">Started at</td>
                            <td style="padding:8px 12px;border-bottom:1px solid #d8dee4;">{{.started_at}}</td>
                        </tr>
                        <tr>
                            <td style="width:220px;padding:8px 12px;border-bottom:1px solid #d8dee4;color:#57606a;">Finished at</td>
                            <td style="padding:8px 12px;border-bottom:1px solid #d8dee4;">{{.finished_at}}</td>
                        </tr>
        {{if .dataflow_started}}
                        <tr>
                            <td style="width:220px;padding:8px 12px;border-bottom:1px solid #d8dee4;color:#57606a;">Dataflow job ID</td>
                            <td style="padding:8px 12px;border-bottom:1px solid #d8dee4;">{{.dataflow_job_id}}</td>
                        </tr>
        {{if .argo_task_id}}
                        <tr>
                            <td style="width:220px;padding:8px 12px;border-bottom:1px solid #d8dee4;color:#57606a;">Argo task ID</td>
                            <td style="padding:8px 12px;border-bottom:1px solid #d8dee4;">{{.argo_task_id}}</td>
                        </tr>
        {{end}}
        {{if .job_name}}
                        <tr>
                            <td style="width:220px;padding:8px 12px;border-bottom:1px solid #d8dee4;color:#57606a;">Job name</td>
                            <td style="padding:8px 12px;border-bottom:1px solid #d8dee4;">{{.job_name}}</td>
                        </tr>
        {{end}}
                        <tr>
                            <td style="width:220px;padding:8px 12px;border-bottom:1px solid #d8dee4;color:#57606a;">Dataflow status</td>
                            <td style="padding:8px 12px;border-bottom:1px solid #d8dee4;">{{.dataflow_status}}</td>
                        </tr>
                    </table>
        {{if .message}}
                    <h3 style="margin:20px 0 8px;font-size:15px;color:#24292f;">Message</h3>
                    <div style="padding:12px;background:#f6f8fa;border:1px solid #d0d7de;border-radius:6px;white-space:pre-wrap;word-break:break-word;font-family:ui-monospace,SFMono-Regular,Consolas,'Liberation Mono',Menlo,monospace;font-size:13px;color:#24292f;">{{.message}}</div>
        {{end}}
        {{else}}
                    </table>
        {{end}}
        {{if .error_message}}
                    <h3 style="margin:20px 0 8px;font-size:15px;color:#cf222e;">Error</h3>
                    <div style="padding:12px;background:#fff8f8;border:1px solid #ffb3b8;border-radius:6px;white-space:pre-wrap;word-break:break-word;font-family:ui-monospace,SFMono-Regular,Consolas,'Liberation Mono',Menlo,monospace;font-size:13px;color:#24292f;">{{.error_message}}</div>
        {{end}}
        {{if .frontend_url}}
                    <p style="margin:20px 0 0;color:#57606a;">Portal: <a href="{{.frontend_url}}" style="color:#0969da;">{{.frontend_url}}</a></p>
        {{end}}
                </td>
            </tr>
        </table>
    </body>
</html>
