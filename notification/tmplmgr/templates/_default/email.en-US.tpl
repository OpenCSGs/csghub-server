{{/* title section */}}
Notification
---
{{/* content section */}}
<html>
    <p> 
        Notification
    </p>
    {{range $key, $value := .}}
    <p>{{$key}}: {{$value}}</p>
    {{end}}
</html>
