# Aliyun checker plugin

This directory contains the go-plugin implementation for Aliyun Green checks.
The host uses `builder/plugins_manager` to start and cache one plugin process.

## Build

```bash
go build -o bin/plugins/csghub_aliyun_checker/v1.0.0/csghub_aliyun_checker ./plugins/aliyun_checker/cmd

# or

go build -ldflags="-s -w" -o bin/plugins/csghub_aliyun_checker/v1.0.0/csghub_aliyun_checker ./plugins/aliyun_checker/cmd
```
