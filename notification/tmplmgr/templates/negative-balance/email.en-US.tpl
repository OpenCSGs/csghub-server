{{/* title section */}}
Your account is past due
---
{{/* content section */}}
<html>
    <body>
        <h3>Your account is past due</h3>
        <p>Your account has a past due balance of {{if eq .currency "CNY"}}¥{{else if eq .currency "USD"}}${{else}}{{.currency}} {{end}}{{.amount}}. Services will be automatically suspended if the debt exceeds {{.threshold}}. Please recharge immediately to avoid interruption.</p>
    </body>
</html> 