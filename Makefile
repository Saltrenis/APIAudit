.PHONY: build install test vet fmt clean release

BINARY=apiaudit
MODULE=github.com/Saltrenis/APIAudit

build:
	go build -o bin/$(BINARY) ./cmd/apiaudit

install:
	go install ./cmd/apiaudit

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -rf bin/ dist/

release:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/$(BINARY)-darwin-arm64 ./cmd/apiaudit
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/$(BINARY)-darwin-amd64 ./cmd/apiaudit
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/$(BINARY)-linux-amd64 ./cmd/apiaudit
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/$(BINARY)-linux-arm64 ./cmd/apiaudit
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/$(BINARY)-windows-amd64.exe ./cmd/apiaudit

# Install the Claude slash command globally
install-claude-command:
	mkdir -p ~/.claude/commands
	cp .claude/commands/api-audit-annotate.md ~/.claude/commands/

.DEFAULT_GOAL := build
