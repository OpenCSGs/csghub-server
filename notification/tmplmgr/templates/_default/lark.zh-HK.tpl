{
  "zh_hk": {
    "title": "通知",
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