.PHONY: build test fmt check

build:
	go build -o ghealth .
	go build -o ghealth-mcp ./cmd/ghealth-mcp

test:
	go test ./...

fmt:
	gofmt -w .

check: fmt test
	go build ./...
