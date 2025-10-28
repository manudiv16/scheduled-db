# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o scheduled-db \
    cmd/scheduled-db/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates and create user
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser && \
    mkdir -p /data && \
    chown -R appuser:appgroup /data

# Copy the binary
COPY --from=builder /app/scheduled-db /scheduled-db

# Set ownership and permissions
RUN chmod +x /scheduled-db

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 7000 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/scheduled-db", "--help"] || exit 1

# Set entrypoint
ENTRYPOINT ["/scheduled-db"]
