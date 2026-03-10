.PHONY: build test clean run lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/mcp-code-graph ./cmd/mcp-code-graph

test:
	go test -v -race ./...

test-short:
	go test -short -race ./...

clean:
	rm -rf bin/
	rm -rf .mcp-code-graph/

run:
	go run ./cmd/mcp-code-graph

lint:
	golangci-lint run

install: build
	cp bin/mcp-code-graph $(GOBIN)/mcp-code-graph
