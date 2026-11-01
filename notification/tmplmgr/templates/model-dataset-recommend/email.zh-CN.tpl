CSGHub 模型数据集推荐
---
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
<p>您好，</p>

<p>本周 CSGHub 社区有一批新的热门模型与高质量数据集上线，基于社区使用趋势与活跃度，我们为您精选了值得关注的内容：</p>

<h2 style="color: #ff4500;">🔥 热门模型</h2>
{{range .models}}
<div style="margin-bottom: 20px; padding: 15px; background-color: #f9f9f9; border-radius: 5px;">
<h3 style="margin-top: 0;">{{.name}}</h3>
<p><strong>简述：</strong> {{.desc}}</p>
{{if .highlight}}<p><strong>亮点：</strong> {{.highlight}}</p>{{end}}
<p><a href="{{.link}}" style="color: #0066cc; text-decoration: none;">👉 查看模型</a></p>
</div>
{{end}}

<h2 style="color: #1e90ff;">📊 热门数据集</h2>
{{range .datasets}}
<div style="margin-bottom: 20px; padding: 15px; background-color: #f9f9f9; border-radius: 5px;">
<h3 style="margin-top: 0;">{{.name}}</h3>
<p><strong>简述：</strong> {{.desc}}</p>
{{if .domain}}<p><strong>领域：</strong> {{.domain}} {{end}}{{if .size}}<strong>规模：</strong> {{.size}}{{end}}</p>
{{if .scene}}<p><strong>适用场景：</strong> {{.scene}}</p>{{end}}
<p><a href="{{.link}}" style="color: #0066cc; text-decoration: none;">👉 查看数据集</a></p>
</div>
{{end}}

<p>您也可以在传神社区中：</p>

<ul style="list-style-type: disc; margin-left: 20px;">
<li>一键拉取模型 / 数据集进行本地或私有化使用</li>
<li>直接进入推理、微调或评测流程</li>
<li>收藏内容，持续跟踪后续更新</li>
</ul>

<p><a href="https://opencsg.com/" style="color: #0066cc; text-decoration: none; font-weight: bold;">👉 打开传神社区</a></p>

<p>如果您不想接收此类更新，可在个人设置中调整邮件偏好。</p>

<p>OpenCSG Team<br>
Empowering Everyone with Large Language Models</p>
</body>
</html>