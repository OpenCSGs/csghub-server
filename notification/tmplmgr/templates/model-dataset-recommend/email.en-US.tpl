Model Dataset Recommendations from CSGHub
---
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
<p>Hi,</p>

<p>This week, CSGHub community has launched a batch of new hot models and high-quality datasets. Based on community usage trends and activity, we've selected content worth paying attention to for you:</p>

<h2 style="color: #ff4500;">🔥 Hot Models</h2>
{{range .models}}
<div style="margin-bottom: 20px; padding: 15px; background-color: #f9f9f9; border-radius: 5px;">
<h3 style="margin-top: 0;">{{.name}}</h3>
<p><strong>Brief:</strong> {{.desc}}</p>
{{if .highlight}}<p><strong>Highlight:</strong> {{.highlight}}</p>{{end}}
<p><a href="{{.link}}" style="color: #0066cc; text-decoration: none;">👉 View Model</a></p>
</div>
{{end}}

<h2 style="color: #1e90ff;">📊 Hot Datasets</h2>
{{range .datasets}}
<div style="margin-bottom: 20px; padding: 15px; background-color: #f9f9f9; border-radius: 5px;">
<h3 style="margin-top: 0;">{{.name}}</h3>
<p><strong>Brief:</strong> {{.desc}}</p>
{{if .domain}}<p><strong>Domain:</strong> {{.domain}} {{end}}{{if .size}}<strong>Size:</strong> {{.size}}{{end}}</p>
{{if .scene}}<p><strong>Applicable Scenarios:</strong> {{.scene}}</p>{{end}}
<p><a href="{{.link}}" style="color: #0066cc; text-decoration: none;">👉 View Dataset</a></p>
</div>
{{end}}

<p>You can also in the CSGHub community:</p>

<ul style="list-style-type: disc; margin-left: 20px;">
<li>One-click pull models/datasets for local or private use</li>
<li>Directly enter inference, fine-tuning or evaluation processes</li>
<li>Bookmark content and continuously track subsequent updates</li>
</ul>

<p><a href="https://opencsg.com/" style="color: #0066cc; text-decoration: none; font-weight: bold;">👉 Open CSGHub</a></p>

<p>If you don't want to receive such updates, you can adjust your email preferences in your personal settings.</p>

<p>OpenCSG Team<br>
Empowering Everyone with Large Language Models</p>
</body>
</html>