{{/* title section */}}
{{if eq .status "succeeded"}}
    Recharge Successful
{{else if eq .status "created"}}
    Recharge Order Created
{{else if eq .status "closed"}}
    Recharge Order Closed
{{else if eq .status "deposited"}}
    Recharge Deposited
{{else}}
    Recharge Order Updated
{{end}}
---
{{/* content section */}}
<html>
    <body>
        {{if eq .status "succeeded"}}
            <h3>Recharge Successful</h3>
            <p>Your recharge order {{.order_no}} has been successfully processed.</p>
            <p>Amount: {{if eq .currency "CNY"}}¥{{else if eq .currency "USD"}}${{else}}{{.currency}} {{end}}{{.amount}}</p>
        {{else if eq .status "created"}}
            <h3>Recharge Order Created</h3>
            <p>Your recharge order {{.order_no}} has been created.</p>
            <p>Amount: {{if eq .currency "CNY"}}¥{{else if eq .currency "USD"}}${{else}}{{.currency}} {{end}}{{.amount}}</p>
        {{else if eq .status "closed"}}
            <h3>Recharge Order Closed</h3>
            <p>Your recharge order {{.order_no}} has been closed.</p>
            <p>Amount: {{if eq .currency "CNY"}}¥{{else if eq .currency "USD"}}${{else}}{{.currency}} {{end}}{{.amount}}</p>
        {{else if eq .status "deposited"}}
            <h3>Recharge Deposited</h3>
            <p>Your recharge order {{.order_no}} has been deposited.</p>
            <p>Amount: {{if eq .currency "CNY"}}¥{{else if eq .currency "USD"}}${{else}}{{.currency}} {{end}}{{.amount}}</p>
        {{else}}
            <h3>Recharge Order Updated</h3>
            <p>The status of your recharge order {{.order_no}} has been updated to {{.status}}.</p>
            <p>Amount: {{if eq .currency "CNY"}}¥{{else if eq .currency "USD"}}${{else}}{{.currency}} {{end}}{{.amount}}</p>
        {{end}}
    </body>
</html> 