BINARY  := bin/dsv-tracking-mcp
CMD     := ./cmd/dsv-tracking-mcp

.PHONY: build test test-integration lint run clean

build:
	go build -o $(BINARY) $(CMD)

test:
	go test -race ./...

lint:
	golangci-lint run ./...

run: build
	$(BINARY)

run-demo:
	go run ./cmd/demo --

test-integration:
	BROWSER_INTEGRATION=true go test -race -tags=integration -count=1 ./...

clean:
	rm -rf bin/
