{
  "zh_cn": {
    "title": "Notification",
    "content": [
      [
        {{- $first := true -}}
        {{- range $key, $value := . }}
          {{- if not $first }},{{end}}{"tag": "text", "text": "{{$key}}: {{$value}}\n"}{{- $first = false }}
        {{- end }}
      ]
    ]
  }
}