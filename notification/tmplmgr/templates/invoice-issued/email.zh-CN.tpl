{{/* title section */}}
发票已开具
---
{{/* content section */}}
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#F2F4F7;font-family:'Helvetica Neue',Helvetica,'PingFang SC','Hiragino Sans GB','Microsoft YaHei','微软雅黑',Arial,sans-serif;">
    <div style="background-color:#F2F4F7;padding:32px 12px;">
        <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:600px;margin:0 auto;">
            <tr>
                <td style="background-color:#FFFFFF;border-radius:12px;overflow:hidden;border:1px solid #EAECF0;box-shadow:0px 1px 2px 0px rgba(16,24,40,0.05);">

                    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
                        <tr>
                            <td style="background:linear-gradient(135deg,#3250BD 0%,#223B99 100%);padding:28px 32px;">
                                <div style="color:#FFFFFF;font-size:18px;font-weight:600;line-height:28px;">发票已开具</div>
                                <div style="color:#ACBDFF;font-size:13px;line-height:20px;margin-top:4px;">您的开票申请已处理完成，请查收发票</div>
                            </td>
                        </tr>
                    </table>

                    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
                        <tr>
                            <td style="padding:32px;">
                                <p style="margin:0 0 20px;color:#101828;font-size:14px;line-height:22px;">您好，{{.user_name}}：</p>
                                <p style="margin:0 0 20px;color:#475467;font-size:14px;line-height:22px;">您申请的发票已开具完成，以下是本次开票的信息：</p>

                                <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border:1px solid #EAECF0;border-radius:8px;overflow:hidden;">
                                    <tr>
                                        <td style="padding:12px 16px;background-color:#F9FAFB;color:#667085;font-size:13px;line-height:18px;width:80px;">发票抬头</td>
                                        <td style="padding:12px 16px;color:#101828;font-size:14px;line-height:22px;">{{.invoice_title}}</td>
                                    </tr>
                                    {{if .invoice_type}}
                                    <tr>
                                        <td style="padding:12px 16px;background-color:#F9FAFB;color:#667085;font-size:13px;line-height:18px;">发票类型</td>
                                        <td style="padding:12px 16px;color:#101828;font-size:14px;line-height:22px;">{{if eq .invoice_type "vat"}}增值税专用发票{{else if eq .invoice_type "ordinary"}}普通发票{{else}}{{.invoice_type}}{{end}}</td>
                                    </tr>
                                    {{end}}
                                    <tr>
                                        <td style="padding:12px 16px;background-color:#F9FAFB;color:#667085;font-size:13px;line-height:18px;">开票金额</td>
                                        <td style="padding:12px 16px;color:#3250BD;font-size:14px;line-height:22px;font-weight:600;">¥{{.amount}}</td>
                                    </tr>
                                    {{if .invoice_date}}
                                    <tr>
                                        <td style="padding:12px 16px;background-color:#F9FAFB;color:#667085;font-size:13px;line-height:18px;">开票日期</td>
                                        <td style="padding:12px 16px;color:#101828;font-size:14px;line-height:22px;">{{.invoice_date}}</td>
                                    </tr>
                                    {{end}}
                                </table>

                                {{if .invoice_url}}
                                <div style="text-align:center;margin:28px 0 12px;">
                                    <a href="{{.invoice_url}}" style="display:inline-block;background-color:#3250BD;color:#FFFFFF;text-decoration:none;font-size:15px;font-weight:600;line-height:20px;padding:12px 48px;border-radius:8px;">下载发票</a>
                                </div>
                                <p style="text-align:center;margin:0;color:#98A2B3;font-size:12px;line-height:18px;">如无法点击，请复制以下链接到浏览器打开：<br><span style="word-break:break-all;color:#3250BD;">{{.invoice_url}}</span></p>
                                {{end}}
                            </td>
                        </tr>
                    </table>

                    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
                        <tr>
                            <td style="padding:20px 32px;border-top:1px solid #EAECF0;color:#667085;font-size:12px;line-height:20px;">
                                此邮件由系统自动发送，请勿直接回复。如有疑问，请联系客服。
                            </td>
                        </tr>
                    </table>

                </td>
            </tr>
        </table>
        <p style="text-align:center;color:#98A2B3;font-size:12px;margin:16px 0 0;">CSGHub</p>
    </div>
</body>
</html>