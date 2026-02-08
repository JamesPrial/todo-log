BINARY := bin/save-todos
MODULE := github.com/JamesPrial/todo-log
GOFLAGS := -trimpath -ldflags="-s -w"

.PHONY: build test clean

build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/save-todos

test:
	go test -v -race ./...

cover:
	go test -cover ./...

clean:
	rm -f $(BINARY)
