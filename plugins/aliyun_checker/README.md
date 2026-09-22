# Aliyun checker plugin

This directory contains the go-plugin implementation for Aliyun Green checks.
The host uses `builder/plugins_manager` to start and cache one plugin process.

## Build

```bash
go build -o bin/plugins/csghub_aliyun_checker/v1.0.0/csghub_aliyun_checker ./plugins/aliyun_checker/cmd

# or

go build -ldflags="-s -w" -o bin/plugins/csghub_aliyun_checker/v1.0.0/csghub_aliyun_checker ./plugins/aliyun_checker/cmd
```

## Define and build plugin proto

1. maintain schema file

plugins/aliyun_checker/v1/aliyun_checker.proto

2. install protoc Go plugins

```bash
GOBIN=/tmp/csghub-protoc-bin go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
GOBIN=/tmp/csghub-protoc-bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
```

2. generate go code by run command under root directory

```bash
PATH=/tmp/csghub-protoc-bin:$PATH protoc \
  --go_out=. \
  --go_opt=module=opencsg.com/csghub-server \
  --go-grpc_out=. \
  --go-grpc_opt=module=opencsg.com/csghub-server \
  plugins/aliyun_checker/v1/aliyun_checker.proto
```
