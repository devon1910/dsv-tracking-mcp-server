BINARY  := bin/dsv-tracking-mcp-server
CMD     := ./cmd/server

.PHONY: build test test-integration lint run clean

build:
	go build -o $(BINARY) $(CMD)

test:
	go test -race ./...

lint:
	golangci-lint run ./...

run: build
	$(BINARY)

test-integration:
	go test -race -tags=integration ./...

clean:
	rm -rf bin/
