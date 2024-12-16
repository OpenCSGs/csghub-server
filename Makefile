.PHONY: test lint cover mock_wire mock_gen
 
test:
	go test ./...

lint:
	golangci-lint run

cover:
	go test -coverprofile=cover.out ./...
	go tool cover -html=cover.out -o cover.html
	open cover.html

mock_wire:
	@echo "Running wire for component mocks..."
	@go run -mod=mod github.com/google/wire/cmd/wire opencsg.com/csghub-server/component
	@if [ $$? -eq 0 ]; then \
		echo "Renaming wire_gen.go to wire_gen_test.go..."; \
		mv component/wire_gen.go component/wire_gen_test.go; \
	else \
		echo "Wire failed, skipping renaming."; \
	fi

mock_gen:
	mockery
