.PHONY: test lint cover wire mock_gen swag migrate_local

test:
	go test ./...

lint:
	golangci-lint run

cover:
	go test -coverprofile=cover.out ./...
	go tool cover -html=cover.out -o cover.html
	open cover.html

wire:
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
	@echo "Running wire for api/router..."
	@go run -mod=mod github.com/google/wire/cmd/wire gen --header_file=wire/ce_header opencsg.com/csghub-server/api/router/...
	@mv api/router/wire_gen.go api/router/wire_gen_ce.go

	@if [ -f api/router/wire_ee.go ]; then \
		echo "Running wire for ee..."; \
		go run -mod=mod github.com/google/wire/cmd/wire gen -tags=ee --header_file=wire/ee_header opencsg.com/csghub-server/api/router/...; \
		mv api/router/wire_gen.go api/router/wire_gen_ee.go; \
	else \
		echo "wire_ee.go not exists, skipping ee generation..."; \
	fi

	@if [ -f api/router/wire_saas.go ]; then \
		echo "Running wire for saas..."; \
		go run -mod=mod github.com/google/wire/cmd/wire gen -tags=saas --header_file=wire/saas_header opencsg.com/csghub-server/api/router/...; \
		mv api/router/wire_gen.go api/router/wire_gen_saas.go; \
	else \
		echo "wire_saas.go not exists, skipping saas generation..."; \
	fi

mock_gen:
	mockery

swag:
	swag init --pd -d cmd/csghub-server/cmd/start,api/router,api/handler,builder/store/database,common/types,accounting/handler,user/handler,component -g server.go

migrate_local:
	go run cmd/csghub-server/main.go migration migrate --config local.toml
