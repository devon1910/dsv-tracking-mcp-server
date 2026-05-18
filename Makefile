BINARY  := bin/dsv-tracking-mcp-server
CMD     := ./cmd/server

.PHONY: build test lint run clean

build:
	go build -o $(BINARY) $(CMD)

test:
	go test -race ./...

lint:
	golangci-lint run ./...

run: build
	$(BINARY)

clean:
	rm -rf bin/
