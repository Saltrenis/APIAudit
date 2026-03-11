.PHONY: build install test vet fmt clean release web frontend-build frontend-dev

BINARY=apiaudit
MODULE=github.com/Saltrenis/APIAudit

build:
	go build -o bin/$(BINARY) ./cmd/apiaudit

install:
	go install ./cmd/apiaudit

# Build the web UI (frontend + Go server with embedded assets)
web: build frontend-build
	rm -rf cmd/apiaudit-web/frontend/dist
	cp -r frontend/dist cmd/apiaudit-web/frontend/dist
	go build -o bin/$(BINARY)-web ./cmd/apiaudit-web

# Build frontend assets
frontend-build:
	cd frontend && npm install && npm run build

# Start frontend dev server (use with: go run ./cmd/apiaudit-web --dev)
frontend-dev:
	cd frontend && npm run dev

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
