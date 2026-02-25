# Build stage - no CGO required (pure Go SQLite via modernc.org/sqlite)
FROM golang:1.24-bookworm AS builder

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

# Build the application - CGO_ENABLED=0 for a fully static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/server ./server

# Runtime stage - scratch-compatible since binary is static and assets are embedded
FROM gcr.io/distroless/static-debian12

WORKDIR /app

# Copy binary from builder (schema, assets, and categories are embedded in the binary)
COPY --from=builder /app/bin/server /app/server

# Create directories for database and backups
# (distroless doesn't have mkdir, so we use VOLUME instead)
VOLUME ["/app/data", "/app/backups"]

# Expose the default port
EXPOSE 8080

# Run the server with database in the data directory and backups enabled
ENTRYPOINT ["/app/server"]
CMD ["--port", "8080", "--db", "/app/data/cheapskate.db", "--backup-path", "/app/backups"]
