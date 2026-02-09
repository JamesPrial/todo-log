BINARY := bin/save-todos
MCP_BINARY := bin/mcp-server
MODULE := github.com/JamesPrial/todo-log
GOFLAGS := -trimpath -ldflags="-s -w"

.PHONY: build build-mcp build-all test cover clean

build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/save-todos

build-mcp:
	go build $(GOFLAGS) -o $(MCP_BINARY) ./cmd/mcp-server

build-all: build build-mcp

test:
	go test -v -race ./...

cover:
	go test -cover ./...

clean:
	rm -f $(BINARY) $(MCP_BINARY)
