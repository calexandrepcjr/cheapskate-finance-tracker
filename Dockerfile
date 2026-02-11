# Build stage
FROM golang:1.24-bookworm AS builder

# Install build dependencies for CGO (required by go-sqlite3)
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

# Install code generation tools
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest && \
    go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source files
COPY . .

# Generate code (SQLC + Templ)
RUN sqlc generate && templ generate

# Build the application with CGO enabled (required for sqlite3)
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/server ./server

# Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies for SQLite
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/server /app/server

# Copy necessary runtime files
COPY --from=builder /app/server/db/schema.sql /app/server/db/schema.sql
COPY --from=builder /app/client/assets /app/client/assets
COPY --from=builder /app/categories.json /app/categories.json

# Create directories for database and backups
RUN mkdir -p /app/data /app/backups

# Expose the default port
EXPOSE 8080

# Run the server with database in the data directory and backups enabled
CMD ["/app/server", "--port", "8080", "--db", "/app/data/cheapskate.db", "--backup-path", "/app/backups"]
