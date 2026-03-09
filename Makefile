.PHONY: build test clean run

build:
	go build -o bin/mcp-code-graph ./cmd/mcp-code-graph

test:
	go test -v ./...

clean:
	rm -rf bin/
	rm -rf .mcp-code-graph/

run:
	go run ./cmd/mcp-code-graph

lint:
	golangci-lint run
