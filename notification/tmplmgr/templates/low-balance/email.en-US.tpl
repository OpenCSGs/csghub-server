{{/* title section */}}
Balance is below the set threshold
---
{{/* content section */}}
<html>
    <body>
        <h3>Balance is below the set threshold</h3>
        <p>Your account balance has fallen below {{if eq .currency "CNY"}}¥{{else if eq .currency "USD"}}${{else}}{{.currency}} {{end}}{{.amount}}. Please monitor it promptly.</p>
    </body>
</html> 