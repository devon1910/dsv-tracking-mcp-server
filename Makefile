BINARY := dsv-tracking-mcp-server
CMD     := ./cmd/server

.PHONY: build test lint run clean

build:
	go build -o $(BINARY) $(CMD)

test:
	go test ./...

lint:
	golangci-lint run ./...

run:
	go run $(CMD)

clean:
	rm -f $(BINARY)
