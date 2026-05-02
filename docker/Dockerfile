syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o scheduled-db \
    cmd/scheduled-db/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser && \
    mkdir -p /data && \
    chown -R appuser:appgroup /data

COPY --from=builder /app/scheduled-db /scheduled-db
RUN chmod +x /scheduled-db

USER appuser

EXPOSE 7000 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/scheduled-db", "--help"] || exit 1

ENTRYPOINT ["/scheduled-db"]
