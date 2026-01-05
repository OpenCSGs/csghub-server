.PHONY: test lint cover mock_wire mock_gen swag db_migrate db_rollback test_all lint_all

GO_TAGS := $(GO_TAGS)

build:
	go build -tags "$(GO_TAGS)" -o ./bin/csghub-server ./cmd/csghub-server

test:
	go test -tags "$(GO_TAGS)" ./...

test_all:
	$(MAKE) test GO_TAGS=ce
	$(MAKE) test GO_TAGS=ee
	$(MAKE) test GO_TAGS=saas

lint:
	golangci-lint run --build-tags "$(GO_TAGS)"

lint_all:
	$(MAKE) lint GO_TAGS=ce
	$(MAKE) lint GO_TAGS=ee
	$(MAKE) lint GO_TAGS=saas

cover:
	go test -tags "$(GO_TAGS)" -coverprofile=cover.out ./...
	go tool cover -html=cover.out -o cover.html
	open cover.html

mock_wire:
	@echo "Running wire for component mocks..."
	@go run -mod=mod github.com/google/wire/cmd/wire opencsg.com/csghub-server/component/...
	@if [ $$? -eq 0 ]; then \
		echo "Renaming component wire_gen.go to wire_gen_test.go..."; \
		mv component/wire_gen.go component/wire_gen_test.go; \
		echo "Renaming component/callback wire_gen.go to wire_gen_test.go..."; \
		mv component/callback/wire_gen.go component/callback/wire_gen_test.go; \
	else \
		echo "Wire failed, skipping renaming."; \
	fi

mock_gen:
	mockery --tags "$(GO_TAGS)"

swag:
	swag init --pd -d cmd/csghub-server/cmd/start,api/router,api/handler,builder/store/database,common/types,accounting/handler,user/handler,moderation/handler,component,dataviewer/handler,aigateway/handler,aigateway/types,notification/handler -g server.go

db_migrate:
	@go run -tags "$(GO_TAGS)" cmd/csghub-server/main.go migration migrate --config local.toml

db_rollback:
	@go run -tags "$(GO_TAGS)" cmd/csghub-server/main.go migration rollback --config local.toml

start_server:
	@go run -tags "$(GO_TAGS)" cmd/csghub-server/main.go start server -l Info -f json --config local.toml

start_user:
	@go run -tags "$(GO_TAGS)" cmd/csghub-server/main.go user launch -l Info -f json --config local.toml

error_doc:
	@go run cmd/csghub-server/main.go errorx doc-gen

error_scan:
	@go run cmd/csghub-server/main.go errorx scan --dir $(dir) -v

notify_gen:
	@go run cmd/csghub-server/main.go notification notify-gen -l Info
