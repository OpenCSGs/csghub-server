{{/* title section */}}
用户申请发票
---
{{/* content section */}}
<html>
	<body>
		<h3>用户申请发票</h3>
		<p>用户ID：{{.user_name}}，手机号：{{.phone}}，开票金额：¥{{.amount}}</p>
		{{if .user_info_url}}
		<p><a href="{{.user_info_url}}">查看用户信息</a></p>
		{{end}}
	</body>
</html>