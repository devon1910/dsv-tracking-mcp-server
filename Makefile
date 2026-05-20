BINARY  := bin/dsv-tracking-mcp
CMD     := ./cmd/dsv-tracking-mcp

.PHONY: build test test-integration verify lint run clean

build:
	go build -o $(BINARY) $(CMD)

test:
	go test -race ./...

lint:
	golangci-lint run ./...

run: build
	$(BINARY)

test-integration:
	BROWSER_INTEGRATION=true go test -race -tags=integration -count=1 ./...

verify: ## Run live-API verification against real DSV (requires Chrome, hits live infra)
	go run ./cmd/dsv-verify/

clean:
	rm -rf bin/
