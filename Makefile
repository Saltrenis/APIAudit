.PHONY: build install test vet fmt clean

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
	rm -rf bin/

# Install the Claude slash command globally
install-claude-command:
	mkdir -p ~/.claude/commands
	cp .claude/commands/api-audit-annotate.md ~/.claude/commands/

.DEFAULT_GOAL := build
