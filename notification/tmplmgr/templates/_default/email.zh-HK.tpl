{{/* title section */}}
通知
---
{{/* content section */}}
<html>
    <p> 
        通知
    </p>
    {{range $key, $value := .}}
    <p>{{$key}}: {{$value}}</p>
    {{end}}
</html>
