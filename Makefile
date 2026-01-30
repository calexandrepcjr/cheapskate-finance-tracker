# Makefile
.PHONY: all build run generate tools clean

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

# Run the application
run: generate
	go run server/main.go

# Run with hot reload (requires air, optional)
dev:
	air

clean:
	rm -rf bin
	rm -rf server/db/*.go
