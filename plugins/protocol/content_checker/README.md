# Content checker plugin protocol

This directory defines the shared `v1` gRPC protocol for content checker plugins.
Plugin implementations can register `v1.ContentCheckerServer`, and host-side
callers can use `v1.NewContentCheckerClient`.

## Define and build plugin proto

To regenerate Go types and gRPC stubs after changing
`v1/content_checker.proto`, use the generation commands below while passing
`plugins/protocol/content_checker/v1/content_checker.proto`.

1. maintain schema file

plugins/protocol/content_checker/v1/content_checker.proto

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
  plugins/protocol/content_checker/v1/content_checker.proto
```
