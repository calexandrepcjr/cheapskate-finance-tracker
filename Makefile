# Makefile
.PHONY: all build run generate tools clean hooks-cli setup-hooks test

all: build

# Install necessary tools
tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/air-verse/air@latest

# Generate code (SQLC + Templ)
generate:
	sqlc generate
	templ generate

# Build the application
build: generate
	go build -o bin/server ./server

# Build the hooks CLI tool
hooks-cli:
	go build -o bin/hooks-cli ./scripts/hooks-cli

# Setup git hooks (builds hooks-cli first)
setup-hooks: hooks-cli
	./bin/hooks-cli setup-hooks

# Run tests
test:
	go test ./... -v

# Run the application
run: generate
	go run server/main.go

# Run with hot reload (requires air, optional)
dev:
	export PATH=$(PATH):$(HOME)/go/bin && air

clean:
	rm -rf bin
	rm -rf server/db/*.go
