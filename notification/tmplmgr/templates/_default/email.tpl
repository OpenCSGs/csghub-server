<html>
    <p> 
        Notification
    </p>
    {{range $key, $value := .}}
    <p>{{$key}}: {{$value}}</p>
    {{end}}
</html>
